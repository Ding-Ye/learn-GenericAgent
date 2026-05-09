package main

import (
	"context"
	"fmt"
)

type ExitInfo struct {
	Reason string
	Turns  int
}

// Run is the s02 loop. Diff vs s01:
//
//   * Provider.Chat now takes []ToolSpec.
//   * After each Chat we look at resp.ToolCalls.
//     - empty → assistant produced final text → exit TASK_DONE.
//     - non-empty → dispatch each one, collect results, build the next user
//       message containing tool_results, loop again.
//   * History accumulates: assistant message, then tool messages, then loop.
//
// What's still missing (added in s03):
//   * StepOutcome.next_prompt — the "agent wants to continue but needs to
//     hint the next user message" pathway.
//   * StepOutcome.should_exit — explicit early termination.
//   * `no_tool` placeholder logic (s03 will handle the case where the model
//     replies in plain text yet wants the loop to keep going).
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

		// Always append the assistant's reply (text + tool_calls) so subsequent
		// turns see it in their history.
		msgs = append(msgs, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			return ExitInfo{Reason: "TASK_DONE", Turns: turn}, nil
		}

		// Dispatch each tool call; build a fresh user message with all
		// tool_results so the next turn sees them.
		nextMsgs := make([]Message, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			select {
			case chunks <- fmt.Sprintf("\n🛠 %s(%v)\n", tc.Name, tc.Args):
			case <-ctx.Done():
				return ExitInfo{Reason: "ERROR", Turns: turn}, ctx.Err()
			}

			data, derr := registry.Dispatch(ctx, tc.Name, tc.Args, chunks)
			content := MarshalToolResult(data)
			if derr != nil {
				content = "tool error: " + derr.Error()
			}
			nextMsgs = append(nextMsgs, Message{
				Role:      "tool",
				Name:      tc.Name,
				ToolUseID: tc.ID,
				Content:   content,
			})
		}
		msgs = append(msgs, nextMsgs...)
	}

	return ExitInfo{Reason: "MAX_TURNS_EXCEEDED", Turns: maxTurns}, nil
}
