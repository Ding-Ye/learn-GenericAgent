package main

import (
	"context"
	"encoding/json"
	"fmt"
)

type ExitInfo struct {
	Reason string
	Turns  int
	Data   any
}

func marshalToolResult(d any) string {
	if s, ok := d.(string); ok {
		return s
	}
	if d == nil {
		return ""
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func Run(ctx context.Context, provider Provider, registry *Registry, sysPrompt, userInput string, maxTurns int, chunks chan<- string) (ExitInfo, error) {
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
		if len(resp.ToolCalls) == 0 {
			return ExitInfo{Reason: "TASK_DONE", Turns: turn}, nil
		}
		msgs = append(msgs, Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})

		var outcomes []StepOutcome
		var trMsgs []Message
		for _, tc := range resp.ToolCalls {
			select {
			case chunks <- fmt.Sprintf("\n🛠 %s(%v)\n", tc.Name, tc.Args):
			case <-ctx.Done():
				return ExitInfo{Reason: "ERROR", Turns: turn}, ctx.Err()
			}
			oc := registry.Dispatch(ctx, tc.Name, tc.Args, chunks)
			outcomes = append(outcomes, oc)
			trMsgs = append(trMsgs, Message{Role: "tool", Name: tc.Name, ToolUseID: tc.ID, Content: marshalToolResult(oc.Data)})
		}
		for _, oc := range outcomes {
			if oc.ShouldExit {
				return ExitInfo{Reason: "EXITED", Turns: turn, Data: oc.Data}, nil
			}
		}
		seen := map[string]struct{}{}
		var np string
		for _, oc := range outcomes {
			if oc.NextPrompt == "" {
				continue
			}
			if _, ok := seen[oc.NextPrompt]; ok {
				continue
			}
			seen[oc.NextPrompt] = struct{}{}
			if np == "" {
				np = oc.NextPrompt
			} else {
				np += "\n" + oc.NextPrompt
			}
		}
		if np == "" {
			return ExitInfo{Reason: "TASK_DONE", Turns: turn, Data: outcomes[len(outcomes)-1].Data}, nil
		}
		msgs = append(msgs, trMsgs...)
		msgs = append(msgs, Message{Role: "user", Content: np})
	}
	return ExitInfo{Reason: "MAX_TURNS_EXCEEDED", Turns: maxTurns}, nil
}
