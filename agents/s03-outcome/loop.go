package main

import (
	"context"
	"encoding/json"
	"fmt"
)

type ExitInfo struct {
	Reason string
	Turns  int
	Data   any // last outcome's Data, when relevant
}

func marshalToolResult(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	if data == nil {
		return ""
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("[unmarshalable: %v]", err)
	}
	return string(b)
}

// Run is the s03 loop. The big new idea: tool returns dictate flow.
//
// For each turn:
//   1. provider.Chat(msgs, specs)
//   2. dispatch every tool_call → []StepOutcome
//   3. union semantics:
//      - if any outcome.ShouldExit: EXITED.
//      - if no outcome has NextPrompt: TASK_DONE.
//      - else: collect all NextPrompts (deduplicated) and feed as the next
//        user message body, plus tool_results from each outcome.Data.
//
// This matches `agent_loop.py:agent_runner_loop:60-87`:
//   for ii, tc in enumerate(tool_calls):
//       gen = handler.dispatch(...)
//       outcome = yield from gen
//       if outcome.should_exit: exit_reason = ...; break
//       if not outcome.next_prompt: exit_reason = {'result': 'CURRENT_TASK_DONE'}; break
//       next_prompts.add(outcome.next_prompt)
func Run(ctx context.Context,
	provider Provider,
	registry *Registry,
	sysPrompt, userInput string,
	maxTurns int,
	chunks chan<- string,
) (ExitInfo, error) {
	msgs := []Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userInput},
	}
	specs := registry.Specs()

	for turn := 1; turn <= maxTurns; turn++ {
		select {
		case chunks <- fmt.Sprintf("\n--- Turn %d ---\n", turn):
		case <-ctx.Done():
			return ExitInfo{Reason: "ERROR", Turns: turn}, ctx.Err()
		}

		resp, err := provider.Chat(ctx, msgs, specs, chunks)
		if err != nil {
			return ExitInfo{Reason: "ERROR", Turns: turn}, err
		}

		// Empty tool_calls means: model produced final text, no more turns.
		if len(resp.ToolCalls) == 0 {
			return ExitInfo{Reason: "TASK_DONE", Turns: turn}, nil
		}

		msgs = append(msgs, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Dispatch all tools, collect outcomes.
		var outcomes []StepOutcome
		var toolResults []Message
		for _, tc := range resp.ToolCalls {
			select {
			case chunks <- fmt.Sprintf("\n🛠 %s(%v)\n", tc.Name, tc.Args):
			case <-ctx.Done():
				return ExitInfo{Reason: "ERROR", Turns: turn}, ctx.Err()
			}
			oc := registry.Dispatch(ctx, tc.Name, tc.Args, chunks)
			outcomes = append(outcomes, oc)
			toolResults = append(toolResults, Message{
				Role:      "tool",
				Name:      tc.Name,
				ToolUseID: tc.ID,
				Content:   marshalToolResult(oc.Data),
			})
		}

		// Inspect outcomes for hard exit.
		for _, oc := range outcomes {
			if oc.ShouldExit {
				return ExitInfo{Reason: "EXITED", Turns: turn, Data: oc.Data}, nil
			}
		}

		// Aggregate next_prompts (dedup, drop empties).
		seen := map[string]struct{}{}
		var nextPrompt string
		for _, oc := range outcomes {
			if oc.NextPrompt == "" {
				continue
			}
			if _, ok := seen[oc.NextPrompt]; ok {
				continue
			}
			seen[oc.NextPrompt] = struct{}{}
			if nextPrompt == "" {
				nextPrompt = oc.NextPrompt
			} else {
				nextPrompt += "\n" + oc.NextPrompt
			}
		}

		if nextPrompt == "" {
			// All tools said "I'm done."
			return ExitInfo{
				Reason: "TASK_DONE",
				Turns:  turn,
				Data:   outcomes[len(outcomes)-1].Data,
			}, nil
		}

		// Append tool_results, then the synthesized next user message.
		msgs = append(msgs, toolResults...)
		msgs = append(msgs, Message{Role: "user", Content: nextPrompt})
	}
	return ExitInfo{Reason: "MAX_TURNS_EXCEEDED", Turns: maxTurns}, nil
}
