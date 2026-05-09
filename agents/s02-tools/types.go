package main

import "context"

// Message — same as s01 but now with optional tool_use / tool_result fields.
//
// Upstream parallel:
//   GenericAgent uses Claude/OAI native blocks: {role, content: [{type:"tool_use", ...}]}.
//   Our flat shape is the protocol-level shorthand — adequate for s02 mock plumbing.
type Message struct {
	Role      string
	Content   string
	ToolCalls []ToolCall // populated on assistant messages that called tools
	ToolUseID string     // when Role == "tool"
	Name      string     // tool name, when Role == "tool"
}

// ToolCall is one invocation requested by the assistant.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ToolSpec is what we advertise to the provider so the model knows what's
// available. The model's tool_use blocks must reference Name.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// Response now carries tool_calls in addition to text.
type Response struct {
	Content   string
	ToolCalls []ToolCall
}

// Provider stays interface-shaped; we just added the `tools` parameter.
type Provider interface {
	Chat(ctx context.Context, msgs []Message, tools []ToolSpec, chunks chan<- string) (Response, error)
}
