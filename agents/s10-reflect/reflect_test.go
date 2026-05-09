package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recAgent struct {
	mu    sync.Mutex
	tasks []string
	delay time.Duration
}

func (r *recAgent) Run(_ context.Context, task string) (string, error) {
	r.mu.Lock()
	r.tasks = append(r.tasks, task)
	r.mu.Unlock()
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return "ok:" + task, nil
}

func TestReflectLoop_FiresWhenCheckReturnsTask(t *testing.T) {
	var calls atomic.Int32
	check := func(_ context.Context, tick int) (CheckResult, error) {
		c := calls.Add(1)
		if c == 1 {
			return CheckResult{}, nil // tick 1: no task
		}
		return CheckResult{Task: "do thing", Exit: false}, nil
	}
	a := &recAgent{}
	loop := NewReflectLoop(check, a, 10*time.Millisecond).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := loop.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tasks) != 1 || a.tasks[0] != "do thing" {
		t.Fatalf("got %v", a.tasks)
	}
}

func TestReflectLoop_ExitOnSignal(t *testing.T) {
	check := func(_ context.Context, _ int) (CheckResult, error) {
		return CheckResult{Exit: true}, nil
	}
	loop := NewReflectLoop(check, &recAgent{}, 5*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("expected nil exit, got %v", err)
	}
}

func TestReflectLoop_CtxCancelPropagates(t *testing.T) {
	check := func(_ context.Context, _ int) (CheckResult, error) {
		return CheckResult{}, nil
	}
	loop := NewReflectLoop(check, &recAgent{}, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	err := loop.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestReflectLoop_CheckErrorTerminates(t *testing.T) {
	check := func(_ context.Context, _ int) (CheckResult, error) {
		return CheckResult{}, errors.New("boom")
	}
	loop := NewReflectLoop(check, &recAgent{}, 5*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := loop.Run(ctx)
	if err == nil || err.Error() != "check error: boom" {
		t.Fatalf("got %v", err)
	}
}

func TestReflectLoop_OverrideSleep(t *testing.T) {
	var ticks atomic.Int32
	check := func(_ context.Context, _ int) (CheckResult, error) {
		ticks.Add(1)
		// First tick: ask the loop to slow down to 50ms; second tick: ask
		// it to speed back up to 5ms.
		if ticks.Load() == 1 {
			return CheckResult{Sleep: 50 * time.Millisecond}, nil
		}
		return CheckResult{Sleep: 5 * time.Millisecond}, nil
	}
	loop := NewReflectLoop(check, &recAgent{}, 5*time.Millisecond) // base 5ms
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)
	// Within 200ms at 5ms base: tick 1 fires after 5ms, then 50ms,
	// then ~30 more 5ms ticks = ≥5 total.
	if ticks.Load() < 5 {
		t.Fatalf("expected ≥5 ticks; got %d", ticks.Load())
	}
}

func TestJSONCheck_HotReload(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cfg.json")

	check := JSONCheck(p)

	// File doesn't exist yet → empty result.
	r, err := check(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Task != "" || r.Exit {
		t.Fatalf("unexpected: %+v", r)
	}

	// Write a task.
	if err := os.WriteFile(p, []byte(`{"task":"first","sleep_ms":50}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, _ = check(context.Background(), 2)
	if r.Task != "first" || r.Sleep != 50*time.Millisecond {
		t.Fatalf("got %+v", r)
	}

	// Same mtime → cached.
	r2, _ := check(context.Background(), 3)
	if r2 != r {
		t.Fatalf("expected cached: %+v vs %+v", r, r2)
	}

	// Update mtime + content.
	time.Sleep(20 * time.Millisecond) // ensure new mtime nano
	if err := os.WriteFile(p, []byte(`{"task":"second"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r3, _ := check(context.Background(), 4)
	if r3.Task != "second" {
		t.Fatalf("expected hot-reload, got %+v", r3)
	}
}

func TestReflectLoop_OnDoneInvoked(t *testing.T) {
	var seen string
	check := func(_ context.Context, tick int) (CheckResult, error) {
		if tick == 1 {
			return CheckResult{Task: "x"}, nil
		}
		return CheckResult{Exit: true}, nil
	}
	loop := NewReflectLoop(check, &recAgent{}, 5*time.Millisecond)
	loop.OnDone = func(task, result string, err error) { seen = result }
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := loop.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if seen != "ok:x" {
		t.Fatalf("OnDone got %q", seen)
	}
}
