package main

import (
	"context"
	"fmt"
)

type MockReply struct {
	Text      string
	ToolCalls []ToolCall
}

type MockProvider struct {
	Script []MockReply
	calls  int
}

func (m *MockProvider) Chat(ctx context.Context, _ []Message, _ []ToolSpec, chunks chan<- string) (Response, error) {
	if m.calls >= len(m.Script) {
		return Response{}, fmt.Errorf("MockProvider out of script (%d/%d)", m.calls+1, len(m.Script))
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
