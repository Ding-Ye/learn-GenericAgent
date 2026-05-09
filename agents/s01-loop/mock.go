package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockProvider returns canned replies; useful for tests and for trying the
// loop without an API key. Each call to Chat consumes one entry from Replies.
//
// Upstream parallel:
//   The upstream has no MockProvider; it always talks to a real LLM. We add one
//   in the learn version because the loop is the lesson — we don't want
//   API-key plumbing to be in the way of seeing the loop run.
type MockProvider struct {
	Replies     []string
	StreamDelay time.Duration // optional: simulate streaming
	calls       int
}

func (m *MockProvider) Chat(ctx context.Context, msgs []Message, chunks chan<- string) (Response, error) {
	if m.calls >= len(m.Replies) {
		return Response{}, fmt.Errorf("MockProvider: out of canned replies (call %d, only %d configured)", m.calls+1, len(m.Replies))
	}
	reply := m.Replies[m.calls]
	m.calls++

	// Simulate streaming token-by-token. Word-level is plenty for a demo.
	for _, tok := range strings.SplitAfter(reply, " ") {
		select {
		case chunks <- tok:
		case <-ctx.Done():
			return Response{}, ctx.Err()
		}
		if m.StreamDelay > 0 {
			select {
			case <-time.After(m.StreamDelay):
			case <-ctx.Done():
				return Response{}, ctx.Err()
			}
		}
	}
	return Response{Content: reply}, nil
}
