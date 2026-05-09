package main

import (
	"context"
	"fmt"
)

// ExitInfo describes why the loop returned.
type ExitInfo struct {
	Reason string // "MAX_TURNS_EXCEEDED" | "TASK_DONE" | "EXITED" | "ERROR"
	Turns  int
}

// Run is the s01 minimal agent loop.
//
// Upstream parallel:
//   agent_loop.py:agent_runner_loop. The upstream version handles tool dispatch,
//   StepOutcome, callbacks, plan-mode, history compression, and provider rotation.
//   In s01 we strip every layer except: prompt → provider → text reply → done.
//
//   while turn < max_turns:                  ← here
//       response = client.chat(messages)     ← here (provider.Chat)
//       if not response.tool_calls: break    ← s01 has no tools, every reply ends
//       ...                                  ← s02 adds the dispatch path
//
// What's missing on purpose (will be added in later sessions):
//   - tool call detection                  (s02)
//   - StepOutcome control flow             (s03)
//   - real LLM provider                    (s04)
//   - tool implementations                 (s05–s06)
//   - layered memory / working checkpoint  (s07)
//   - skill-tree loading                   (s08)
//   - multi-provider failover              (s09)
//   - reflect / autonomous trigger         (s10)
func Run(ctx context.Context,
	provider Provider,
	sysPrompt, userInput string,
	maxTurns int,
	chunks chan<- string,
) (ExitInfo, error) {
	msgs := []Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userInput},
	}

	for turn := 1; turn <= maxTurns; turn++ {
		// Surface a heartbeat to the consumer so a slow provider feels alive.
		select {
		case chunks <- fmt.Sprintf("\n--- Turn %d ---\n", turn):
		case <-ctx.Done():
			return ExitInfo{Reason: "ERROR", Turns: turn}, ctx.Err()
		}

		resp, err := provider.Chat(ctx, msgs, chunks)
		if err != nil {
			return ExitInfo{Reason: "ERROR", Turns: turn}, err
		}

		// In s01 the model has no tools, so every assistant reply terminates the
		// task. From s02 onward, we'll only exit when the model declines to call
		// a tool *and* there's nothing else queued.
		msgs = append(msgs, Message{Role: "assistant", Content: resp.Content})
		return ExitInfo{Reason: "TASK_DONE", Turns: turn}, nil
	}

	return ExitInfo{Reason: "MAX_TURNS_EXCEEDED", Turns: maxTurns}, nil
}
