package main

import (
	"context"
	"strings"
	"testing"
)

func drain(ch <-chan string) string {
	var b strings.Builder
	for c := range ch {
		b.WriteString(c)
	}
	return b.String()
}

func newTestRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "echo"}, func(_ context.Context, args map[string]any, _ chan<- string) (any, error) {
		return map[string]any{"echoed": args["text"]}, nil
	})
	reg.Register(ToolSpec{Name: "upper"}, func(_ context.Context, args map[string]any, _ chan<- string) (any, error) {
		t, _ := args["text"].(string)
		return strings.ToUpper(t), nil
	})
	return reg
}

func TestRun_TwoToolCalls_ThenDone(t *testing.T) {
	reg := newTestRegistry()
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "echo", Args: map[string]any{"text": "a"}}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "upper", Args: map[string]any{"text": "b"}}}},
		{Text: "all done"},
	}}
	chunks := make(chan string, 64)
	doneStream := make(chan string, 1)
	go func() { doneStream <- drain(chunks) }()

	exit, err := Run(context.Background(), prov, reg, "sys", "go", 10, chunks)
	close(chunks)
	streamed := <-doneStream

	if err != nil {
		t.Fatal(err)
	}
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE got %q", exit.Reason)
	}
	if exit.Turns != 3 {
		t.Fatalf("want 3 turns got %d", exit.Turns)
	}
	if !strings.Contains(streamed, "🛠 echo") {
		t.Fatalf("expected dispatch trace, got: %q", streamed)
	}
	if !strings.Contains(streamed, "🛠 upper") {
		t.Fatalf("expected upper dispatch trace, got: %q", streamed)
	}
}

func TestRun_UnknownTool_ReturnsErrorContent(t *testing.T) {
	reg := newTestRegistry()
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "nonexistent", Args: nil}}},
		{Text: "ok i'll stop"},
	}}
	chunks := make(chan string, 32)
	go func() {
		for range chunks {
		}
	}()
	exit, err := Run(context.Background(), prov, reg, "sys", "go", 5, chunks)
	close(chunks)

	if err != nil {
		t.Fatalf("loop should not error on unknown tool, got: %v", err)
	}
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE got %q", exit.Reason)
	}
}

func TestRegistry_DispatchUnknown(t *testing.T) {
	reg := NewRegistry()
	chunks := make(chan string, 4)
	go func() {
		for range chunks {
		}
	}()
	_, err := reg.Dispatch(context.Background(), "missing", nil, chunks)
	close(chunks)
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("error message regression: %v", err)
	}
}

func TestMarshalToolResult_StringPassthrough(t *testing.T) {
	if got := MarshalToolResult("hello"); got != "hello" {
		t.Fatalf("string passthrough broken: %q", got)
	}
	got := MarshalToolResult(map[string]string{"k": "v"})
	if !strings.Contains(got, "\"k\":\"v\"") {
		t.Fatalf("json encoding broken: %q", got)
	}
}
