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

func runWith(t *testing.T, reg *Registry, prov Provider, maxTurns int) (ExitInfo, string) {
	t.Helper()
	chunks := make(chan string, 64)
	doneStream := make(chan string, 1)
	go func() { doneStream <- drain(chunks) }()
	exit, err := Run(context.Background(), prov, reg, "sys", "go", maxTurns, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	return exit, <-doneStream
}

func TestStepOutcome_ShouldExit(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "exit"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{Data: "bye", ShouldExit: true}
	})
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "exit"}}},
	}}
	exit, _ := runWith(t, reg, prov, 5)
	if exit.Reason != "EXITED" {
		t.Fatalf("want EXITED got %q", exit.Reason)
	}
	if exit.Data != "bye" {
		t.Fatalf("data lost: %v", exit.Data)
	}
}

func TestStepOutcome_NextPromptThenDone(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "more"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{NextPrompt: "more please"}
	})
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "more"}}},
		{Text: "OK done"}, // no tool_calls → loop ends
	}}
	exit, _ := runWith(t, reg, prov, 10)
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE got %q", exit.Reason)
	}
	if exit.Turns != 2 {
		t.Fatalf("want 2 turns got %d", exit.Turns)
	}
}

func TestStepOutcome_EmptyNextPrompt_TaskDone(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "noop"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{Data: "fine"} // no NextPrompt, no ShouldExit → done
	})
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "noop"}}},
	}}
	exit, _ := runWith(t, reg, prov, 5)
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE got %q", exit.Reason)
	}
	if exit.Data != "fine" {
		t.Fatalf("expected outcome.Data to flow through, got %v", exit.Data)
	}
}

func TestUnknownTool_FedBackAsNextPrompt(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "real"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{}
	})
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{{ID: "1", Name: "ghost"}}}, // unknown
		{Text: "I'll try real next"},                       // no tool_calls
	}}
	exit, _ := runWith(t, reg, prov, 5)
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE after self-correct, got %q", exit.Reason)
	}
}

func TestMultipleNextPrompts_Dedup(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolSpec{Name: "a"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{NextPrompt: "same"}
	})
	reg.Register(ToolSpec{Name: "b"}, func(_ context.Context, _ map[string]any, _ chan<- string) StepOutcome {
		return StepOutcome{NextPrompt: "same"}
	})
	prov := &MockProvider{Script: []MockReply{
		{ToolCalls: []ToolCall{
			{ID: "1", Name: "a"},
			{ID: "2", Name: "b"},
		}},
		{Text: "ok"}, // close out
	}}
	exit, _ := runWith(t, reg, prov, 5)
	if exit.Reason != "TASK_DONE" {
		t.Fatalf("want TASK_DONE got %q", exit.Reason)
	}
}
