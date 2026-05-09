package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	user := flag.String("user", "Say hi in one short sentence, then call no tools.", "user message")
	model := flag.String("model", "claude-haiku-4-5-20251001", "anthropic model id")
	flag.Parse()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY is not set; run with -h to see options.")
		os.Exit(2)
	}

	provider := NewAnthropicProvider(apiKey, *model)

	reg := NewRegistry()
	reg.Register(ToolSpec{
		Name:        "echo",
		Description: "Echo a string. Useful only for demos.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []string{"text"},
		},
	}, func(ctx context.Context, args map[string]any, chunks chan<- string) StepOutcome {
		t, _ := args["text"].(string)
		select {
		case chunks <- "[echo] " + t + "\n":
		case <-ctx.Done():
		}
		return StepOutcome{Data: map[string]any{"echoed": t}}
	})

	chunks := make(chan string, 64)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()
	exit, err := Run(context.Background(), provider, reg, "You are a brief helpful agent.", *user, 6, chunks)
	close(chunks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n[err]", err)
		os.Exit(1)
	}
	fmt.Printf("\n[exit] reason=%s turns=%d\n", exit.Reason, exit.Turns)
}
