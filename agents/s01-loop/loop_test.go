package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func collectChunks(ch <-chan string) string {
	var b strings.Builder
	for c := range ch {
		b.WriteString(c)
	}
	return b.String()
}

func TestRun_SingleTurn(t *testing.T) {
	provider := &MockProvider{Replies: []string{"hello world"}}
	chunks := make(chan string, 16)

	doneStream := make(chan string, 1)
	go func() { doneStream <- collectChunks(chunks) }()

	exit, err := Run(context.Background(), provider, "sys", "go", 5, chunks)
	close(chunks)
	streamed := <-doneStream

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE, got %q", exit.Reason)
	}
	if exit.Turns != 1 {
		t.Fatalf("want 1 turn, got %d", exit.Turns)
	}
	if !strings.Contains(streamed, "hello world") {
		t.Fatalf("expected streamed text to contain reply, got %q", streamed)
	}
	if !strings.Contains(streamed, "Turn 1") {
		t.Fatalf("expected turn marker, got %q", streamed)
	}
}

func TestRun_ContextCancel(t *testing.T) {
	provider := &MockProvider{
		Replies:     []string{strings.Repeat("token ", 20)},
		StreamDelay: 100 * time.Millisecond,
	}
	chunks := make(chan string, 64)
	go func() {
		for range chunks {
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the loop even starts → first send to chunks should fail

	_, err := Run(ctx, provider, "sys", "go", 5, chunks)
	close(chunks)

	if err == nil {
		t.Fatalf("expected ctx.Err(), got nil")
	}
}

func TestRun_NoCannedReply(t *testing.T) {
	provider := &MockProvider{Replies: nil}
	chunks := make(chan string, 16)
	go func() {
		for range chunks {
		}
	}()
	_, err := Run(context.Background(), provider, "sys", "go", 5, chunks)
	close(chunks)
	if err == nil {
		t.Fatalf("expected error from MockProvider, got nil")
	}
	if !strings.Contains(err.Error(), "out of canned replies") {
		t.Fatalf("error message regression: %v", err)
	}
}

func TestRun_MaxTurnsZero(t *testing.T) {
	provider := &MockProvider{Replies: []string{"unreached"}}
	chunks := make(chan string, 16)
	go func() {
		for range chunks {
		}
	}()
	exit, err := Run(context.Background(), provider, "sys", "go", 0, chunks)
	close(chunks)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if exit.Reason != "MAX_TURNS_EXCEEDED" {
		t.Fatalf("want MAX_TURNS_EXCEEDED, got %q", exit.Reason)
	}
}
