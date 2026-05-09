package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	reg := NewRegistry()

	// finishTask: tool that asks the loop to terminate.
	reg.Register(ToolSpec{
		Name:        "finish_task",
		Description: "Call when the work is complete. Loop will exit.",
	}, func(ctx context.Context, args map[string]any, chunks chan<- string) StepOutcome {
		summary, _ := args["summary"].(string)
		select {
		case chunks <- "\n[finish] " + summary + "\n":
		case <-ctx.Done():
		}
		return StepOutcome{Data: summary, ShouldExit: true}
	})

	// thinkAgain: tool that requests another turn with a hint.
	reg.Register(ToolSpec{
		Name:        "think_again",
		Description: "Request another turn with a hint message.",
	}, func(_ context.Context, args map[string]any, _ chan<- string) StepOutcome {
		hint, _ := args["hint"].(string)
		return StepOutcome{Data: "ok", NextPrompt: "[hint] " + hint}
	})

	// done: tool that signals "this iteration is done; loop should TASK_DONE
	// since NextPrompt is empty".
	reg.Register(ToolSpec{
		Name:        "done",
		Description: "Done with this micro-task. No further hint.",
	}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{Data: "complete"}
	})

	provider := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "think_again", Args: map[string]any{"hint": "consider edge case"}}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "finish_task", Args: map[string]any{"summary": "all checks passed"}}}},
	}}

	chunks := make(chan string, 32)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()
	exit, err := Run(context.Background(), provider, reg, "demo s03", "go", 10, chunks)
	close(chunks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[err]", err)
		os.Exit(1)
	}
	fmt.Printf("\n[exit] reason=%s turns=%d data=%v\n", exit.Reason, exit.Turns, exit.Data)
}
