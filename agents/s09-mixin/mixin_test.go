package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recProvider struct {
	name  string
	calls atomic.Int32
	fail  func(call int32) error
	reply string
}

func (r *recProvider) Name() string { return r.name }
func (r *recProvider) Chat(_ context.Context, _ []Message, _ []ToolSpec, _ chan<- string) (Response, error) {
	c := r.calls.Add(1)
	if r.fail != nil {
		if err := r.fail(c); err != nil {
			return Response{}, err
		}
	}
	return Response{Content: r.reply}, nil
}

func drain(ch <-chan string) string {
	var b strings.Builder
	for c := range ch {
		b.WriteString(c)
	}
	return b.String()
}

func TestMixin_PrimarySucceeds(t *testing.T) {
	a := &recProvider{name: "A", reply: "from A"}
	b := &recProvider{name: "B", reply: "from B"}
	m := NewMixinProvider(1000, a, b)

	chunks := make(chan string, 16)
	streamD := make(chan string, 1)
	go func() { streamD <- drain(chunks) }()

	resp, err := m.Chat(context.Background(), nil, nil, chunks)
	close(chunks)
	<-streamD

	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "from A" {
		t.Fatalf("got %q", resp.Content)
	}
	if a.calls.Load() != 1 || b.calls.Load() != 0 {
		t.Fatalf("calls A=%d B=%d", a.calls.Load(), b.calls.Load())
	}
}

func TestMixin_PrimaryFails_FallbackToB(t *testing.T) {
	a := &recProvider{name: "A", fail: func(int32) error { return errors.New("rate limit hit") }}
	b := &recProvider{name: "B", reply: "from B"}
	m := NewMixinProvider(1000, a, b)

	chunks := make(chan string, 32)
	go func() {
		for range chunks {
		}
	}()
	resp, err := m.Chat(context.Background(), nil, nil, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "from B" {
		t.Fatalf("got %q", resp.Content)
	}
	if a.calls.Load() != 1 || b.calls.Load() != 1 {
		t.Fatalf("calls A=%d B=%d", a.calls.Load(), b.calls.Load())
	}
}

func TestMixin_StickyToFallback(t *testing.T) {
	a := &recProvider{name: "A", fail: func(int32) error { return errors.New("503") }}
	b := &recProvider{name: "B", reply: "from B"}
	m := NewMixinProvider(500, a, b)

	chunks := make(chan string, 32)
	go func() {
		for range chunks {
		}
	}()
	// First call fails over to B; current = 1.
	if _, err := m.Chat(context.Background(), nil, nil, chunks); err != nil {
		t.Fatal(err)
	}
	// Within sticky window: should hit B directly without retrying A again.
	calsBefore := a.calls.Load()
	if _, err := m.Chat(context.Background(), nil, nil, chunks); err != nil {
		t.Fatal(err)
	}
	close(chunks)
	if a.calls.Load() != calsBefore {
		t.Fatalf("expected A skipped on 2nd call within sticky; A calls=%d->%d",
			calsBefore, a.calls.Load())
	}
}

func TestMixin_SpringBackAfterCooldown(t *testing.T) {
	a := &recProvider{name: "A", reply: "from A"}
	a.fail = func(c int32) error {
		if c == 1 {
			return errors.New("503")
		}
		return nil
	}
	b := &recProvider{name: "B", reply: "from B"}
	m := NewMixinProvider(50, a, b) // 50ms sticky
	chunks := make(chan string, 32)
	go func() {
		for range chunks {
		}
	}()

	// First call: A fails, B serves.
	if _, err := m.Chat(context.Background(), nil, nil, chunks); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	// After cooldown: should spring back to A; A is now healthy on call 2.
	resp, err := m.Chat(context.Background(), nil, nil, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "from A" {
		t.Fatalf("expected to spring back to A; got %q", resp.Content)
	}
}

func TestMixin_NonRetryableErrorPropagates(t *testing.T) {
	a := &recProvider{name: "A", fail: func(int32) error { return errors.New("invalid api key") }}
	b := &recProvider{name: "B", reply: "from B"}
	m := NewMixinProvider(1000, a, b)
	chunks := make(chan string, 16)
	go func() {
		for range chunks {
		}
	}()
	_, err := m.Chat(context.Background(), nil, nil, chunks)
	close(chunks)
	if err == nil {
		t.Fatal("expected propagation, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("got %v", err)
	}
	// Did not try B because the primary error wasn't retryable.
	if b.calls.Load() != 0 {
		t.Fatalf("B should be untouched, got %d", b.calls.Load())
	}
}

func TestMixin_AllFail(t *testing.T) {
	a := &recProvider{name: "A", fail: func(int32) error { return errors.New("503") }}
	b := &recProvider{name: "B", fail: func(int32) error { return errors.New("rate limit") }}
	m := NewMixinProvider(1000, a, b)
	chunks := make(chan string, 16)
	go func() {
		for range chunks {
		}
	}()
	_, err := m.Chat(context.Background(), nil, nil, chunks)
	close(chunks)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "all") {
		t.Fatalf("got %v", err)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := map[string]bool{
		"rate limit exceeded":           true,
		"503 service unavailable":       true,
		"timeout connecting":            true,
		"connection refused":            true,
		"invalid api key":               false,
		"unauthorized":                  false,
		"":                              false, // err == nil case wraps separately
	}
	for msg, want := range cases {
		var err error
		if msg != "" {
			err = errors.New(msg)
		}
		if got := isRetryable(err); got != want {
			t.Errorf("isRetryable(%q) = %v, want %v", msg, got, want)
		}
	}
	if !isRetryable(context.DeadlineExceeded) {
		t.Error("ctx.DeadlineExceeded should be retryable")
	}
}
