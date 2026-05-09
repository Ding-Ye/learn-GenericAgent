package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// echoTool is a 1-line example tool: it prints its `text` arg and returns it.
// Mirrors how upstream tools are tiny pure functions you stack on a registry.
func echoTool(ctx context.Context, args map[string]any, chunks chan<- string) (any, error) {
	text, _ := args["text"].(string)
	select {
	case chunks <- "[echo] " + text + "\n":
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return map[string]any{"echoed": text}, nil
}

// upperTool is a second tool to demonstrate two-tool dispatch.
func upperTool(ctx context.Context, args map[string]any, chunks chan<- string) (any, error) {
	text, _ := args["text"].(string)
	return strings.ToUpper(text), nil
}

func main() {
	reg := NewRegistry()
	reg.Register(ToolSpec{
		Name:        "echo",
		Description: "Echoes the given text back.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []string{"text"},
		},
	}, echoTool)
	reg.Register(ToolSpec{
		Name:        "upper",
		Description: "Uppercases the text.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []string{"text"},
		},
	}, upperTool)

	// Scripted run: turn 1 calls echo, turn 2 calls upper, turn 3 says "done".
	provider := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "t1", Name: "echo", Args: map[string]any{"text": "hello"}}}},
		{ToolCalls: []ToolCall{{ID: "t2", Name: "upper", Args: map[string]any{"text": "loop"}}}},
		{Text: "all done"},
	}}

	chunks := make(chan string, 32)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()
	exit, err := Run(context.Background(), provider, reg, "you can use echo+upper", "demo", 10, chunks)
	close(chunks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[err]", err)
		os.Exit(1)
	}
	fmt.Printf("\n[exit] reason=%s turns=%d\n", exit.Reason, exit.Turns)
}
