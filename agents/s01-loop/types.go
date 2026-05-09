package main

import "context"

// Message is one item in the conversation history sent to the provider.
// Roles: "system", "user", "assistant", "tool".
//
// Upstream parallel:
//   agentmain.py builds messages = [{role, content}] and passes them to client.chat().
//   See agent_loop.py:agent_runner_loop:38-43.
type Message struct {
	Role    string
	Content string
}

// Response is what the provider returns at the end of a turn. In s01 it's just
// text; from s02 onward it carries tool calls too.
type Response struct {
	Content string
}

// Provider is the LLM endpoint abstraction. Chat streams text chunks on
// `chunks` while it works, and returns the final Response when the model
// finishes.
//
// Upstream parallel:
//   llmcore.py defines BaseSession with raw_ask(messages). ToolClient wraps it
//   and exposes chat(messages, tools) returning a generator + final response.
//   We collapse those two layers into one Go interface.
type Provider interface {
	Chat(ctx context.Context, msgs []Message, chunks chan<- string) (Response, error)
}
