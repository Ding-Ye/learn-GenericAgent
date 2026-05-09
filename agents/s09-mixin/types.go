package main

import "context"

type Message struct {
	Role      string
	Content   string
	ToolCalls []ToolCall
	ToolUseID string
	Name      string
}

type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Provider interface {
	Chat(ctx context.Context, msgs []Message, tools []ToolSpec, chunks chan<- string) (Response, error)
	// Name is used to label which backend served a request, for diagnostics.
	Name() string
}
