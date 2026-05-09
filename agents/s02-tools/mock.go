package main

import (
	"context"
	"fmt"
)

// MockProvider replies according to a fixed Script. Each entry is either a
// final text reply or a list of tool calls to emit. Lets us drive the loop in
// tests without an LLM.
type MockReply struct {
	Text      string
	ToolCalls []ToolCall // if set, MockProvider returns these as tool_calls
}

type MockProvider struct {
	Script []MockReply
	calls  int
}

func (m *MockProvider) Chat(ctx context.Context, msgs []Message, _ []ToolSpec, chunks chan<- string) (Response, error) {
	if m.calls >= len(m.Script) {
		return Response{}, fmt.Errorf("MockProvider out of script (call %d, %d configured)", m.calls+1, len(m.Script))
	}
	r := m.Script[m.calls]
	m.calls++
	if r.Text != "" {
		select {
		case chunks <- r.Text:
		case <-ctx.Done():
			return Response{}, ctx.Err()
		}
	}
	return Response{Content: r.Text, ToolCalls: r.ToolCalls}, nil
}
