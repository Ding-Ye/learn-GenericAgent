package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// CheckResult is what a Check func returns each tick.
//
//   Task    — non-empty → submit this string as a new agent task
//   Exit    — true → reflect loop terminates
//   Sleep   — override the default interval for the next tick
//
// Upstream parallel:
//   reflect/scheduler.py and reflect/goal_mode.py expose a check() function
//   that returns either a task string or None. We add Exit + Sleep so the
//   Go version is self-contained without a separate INTERVAL/ONCE module
//   global.
type CheckResult struct {
	Task  string        `json:"task,omitempty"`
	Exit  bool          `json:"exit,omitempty"`
	Sleep time.Duration `json:"-"`
	// SleepMS lets a JSON config file express Sleep in milliseconds.
	SleepMS int `json:"sleep_ms,omitempty"`
}

// CheckFunc is what the user supplies. Pure function: same input every call,
// the user is responsible for any state via closure.
type CheckFunc func(ctx context.Context, tick int) (CheckResult, error)

// AgentRunner is the bridge between reflect mode and your agent harness.
// In s_full this would be the full agent's `Run` function. Here it's an
// interface so we can test reflect without dragging in the whole stack.
type AgentRunner interface {
	Run(ctx context.Context, task string) (string, error)
}

// ReflectLoop polls `check` every `interval`. When it returns a non-empty
// Task, hands it to the agent. Optional `onDone(result)` callback. Exits on
// CheckResult.Exit, ctx cancellation, or check func error (non-recoverable).
//
// Upstream parallel:
//   agentmain.py's --reflect mode polls a user-provided Python script's
//   check() every INTERVAL seconds. We translate to a typed CheckFunc.
type ReflectLoop struct {
	Check    CheckFunc
	Agent    AgentRunner
	Interval time.Duration
	OnDone   func(task, result string, err error)
	once     bool // when true, exits after first task completes
	tick     atomic.Int32
}

func NewReflectLoop(check CheckFunc, agent AgentRunner, interval time.Duration) *ReflectLoop {
	return &ReflectLoop{Check: check, Agent: agent, Interval: interval}
}

// Once configures the loop to terminate after one task completes.
func (r *ReflectLoop) Once() *ReflectLoop { r.once = true; return r }

// Run blocks until exit. Safe to invoke from main.
func (r *ReflectLoop) Run(ctx context.Context) error {
	timer := time.NewTimer(r.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			t := r.tick.Add(1)
			result, err := r.Check(ctx, int(t))
			if err != nil {
				return fmt.Errorf("check error: %w", err)
			}
			if result.Exit {
				return nil
			}
			if result.Task != "" {
				out, runErr := r.Agent.Run(ctx, result.Task)
				if r.OnDone != nil {
					r.OnDone(result.Task, out, runErr)
				}
				if r.once {
					return nil
				}
			}
			next := r.Interval
			if result.Sleep > 0 {
				next = result.Sleep
			}
			timer.Reset(next)
		}
	}
}

// JSONCheck adapts a JSON config file into a CheckFunc. The file shape is:
//
//   { "task": "do X", "exit": false, "sleep_ms": 5000 }
//
// On every tick we re-read the file (mtime-aware). Returning the same task
// twice in a row is the user's responsibility — typically the user clears
// "task" via an external process when the trigger condition has been
// consumed.
//
// Upstream parallel:
//   The upstream reflect script can hot-reload its source. We do hot-reload
//   on the JSON config instead — far simpler than dlopen, identical UX from
//   the agent author's perspective.
func JSONCheck(path string) CheckFunc {
	var lastMTime int64
	var cached CheckResult
	return func(_ context.Context, _ int) (CheckResult, error) {
		st, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return CheckResult{}, nil
			}
			return CheckResult{}, err
		}
		mt := st.ModTime().UnixNano()
		if mt == lastMTime {
			return cached, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return CheckResult{}, err
		}
		var r CheckResult
		if err := json.Unmarshal(b, &r); err != nil {
			return CheckResult{}, fmt.Errorf("bad json in %s: %w", path, err)
		}
		if r.SleepMS > 0 {
			r.Sleep = time.Duration(r.SleepMS) * time.Millisecond
		}
		lastMTime = mt
		cached = r
		return r, nil
	}
}
