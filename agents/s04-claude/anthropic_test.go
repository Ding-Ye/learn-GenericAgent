package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeSSE returns a handler that streams a canned SSE script. We use this to
// drive parseSSE end-to-end without hitting the real Anthropic API.
func fakeSSE(events []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, ev := range events {
			fmt.Fprint(w, ev)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
}

func TestAnthropicProvider_TextStreaming(t *testing.T) {
	events := []string{
		`data: {"type":"message_start"}` + "\n\n",
		`data: {"type":"content_block_start","content_block":{"type":"text","text":""}}` + "\n\n",
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hello "}}` + "\n\n",
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"world"}}` + "\n\n",
		`data: {"type":"content_block_stop"}` + "\n\n",
		`data: {"type":"message_stop"}` + "\n\n",
	}
	srv := httptest.NewServer(fakeSSE(events))
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-test")
	p.Endpoint = srv.URL

	chunks := make(chan string, 32)
	var streamed strings.Builder
	doneStream := make(chan struct{})
	go func() {
		for c := range chunks {
			streamed.WriteString(c)
		}
		close(doneStream)
	}()

	resp, err := p.Chat(context.Background(),
		[]Message{{Role: "system", Content: "be brief"}, {Role: "user", Content: "hi"}},
		nil, chunks)
	close(chunks)
	<-doneStream

	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Fatalf("want 'hello world', got %q", resp.Content)
	}
	if streamed.String() != "hello world" {
		t.Fatalf("expected streamed text 'hello world', got %q", streamed.String())
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("no tool calls expected, got %d", len(resp.ToolCalls))
	}
}

func TestAnthropicProvider_ToolUseStreaming(t *testing.T) {
	events := []string{
		`data: {"type":"message_start"}` + "\n\n",
		`data: {"type":"content_block_start","content_block":{"type":"tool_use","id":"tu_1","name":"echo","input":{}}}` + "\n\n",
		`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"text\":"}}` + "\n\n",
		`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"\"hi\"}"}}` + "\n\n",
		`data: {"type":"content_block_stop"}` + "\n\n",
		`data: {"type":"message_stop"}` + "\n\n",
	}
	srv := httptest.NewServer(fakeSSE(events))
	defer srv.Close()

	p := NewAnthropicProvider("k", "m")
	p.Endpoint = srv.URL

	chunks := make(chan string, 32)
	go func() {
		for range chunks {
		}
	}()
	resp, err := p.Chat(context.Background(),
		[]Message{{Role: "user", Content: "use echo"}}, nil, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "echo" || tc.ID != "tu_1" {
		t.Fatalf("name/id mismatch: %+v", tc)
	}
	if got, _ := tc.Args["text"].(string); got != "hi" {
		t.Fatalf("args.text = %q, want hi", got)
	}
}

func TestToAnthropic_RoleConversion(t *testing.T) {
	system, msgs := toAnthropic([]Message{
		{Role: "system", Content: "S1"},
		{Role: "system", Content: "S2"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "thinking", ToolCalls: []ToolCall{{ID: "t1", Name: "x", Args: map[string]any{"k": "v"}}}},
		{Role: "tool", ToolUseID: "t1", Content: "result", Name: "x"},
	})
	if !strings.Contains(system, "S1") || !strings.Contains(system, "S2") {
		t.Fatalf("system fold lost data: %q", system)
	}
	if len(msgs) != 3 {
		t.Fatalf("want 3 (user, assistant, user[toolresult]), got %d", len(msgs))
	}
	if msgs[2].Role != "user" {
		t.Fatalf("tool_result must live in user role, got %q", msgs[2].Role)
	}
}

func TestParseSSE_ErrorEvent(t *testing.T) {
	body := strings.NewReader(`data: {"type":"error","error":{"message":"boom"}}` + "\n\n")
	chunks := make(chan string, 4)
	go func() {
		for range chunks {
		}
	}()
	_, err := parseSSE(body, chunks)
	close(chunks)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should propagate api message: %v", err)
	}
}

func TestAnthropicProvider_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.Copy(w, bytes.NewBufferString(`{"error":"slow down"}`))
	}))
	defer srv.Close()
	p := NewAnthropicProvider("k", "m")
	p.Endpoint = srv.URL
	chunks := make(chan string, 1)
	go func() {
		for range chunks {
		}
	}()
	_, err := p.Chat(context.Background(),
		[]Message{{Role: "user", Content: "x"}}, nil, chunks)
	close(chunks)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 error, got %v", err)
	}
}
