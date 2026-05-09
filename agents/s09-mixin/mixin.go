package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MixinProvider wraps multiple Providers and tries them in order until one
// succeeds. Spring-back behavior: after a fallback succeeds, future calls
// retry from the *primary* (index 0), not the fallback.
//
// Upstream parallel:
//   llmcore.py:MixinSession. The Python version does the same dance with
//   _pick() rotating among `_sessions`, plus per-session error allowlist
//   and a "spring-back" `_priority` field that decays over time.
//   Our Go version is simpler but captures the load-bearing behavior.
type MixinProvider struct {
	primaries []Provider
	mu        sync.Mutex
	current   int       // index of last-used provider; 0 by default
	cooldown  time.Time // primary returns to use after this time
	stickyMS  int       // ms a fallback "sticks" before primary is retried
}

func NewMixinProvider(stickyMS int, providers ...Provider) *MixinProvider {
	if len(providers) == 0 {
		panic("MixinProvider needs at least one provider")
	}
	return &MixinProvider{primaries: providers, stickyMS: stickyMS}
}

func (m *MixinProvider) Name() string {
	parts := make([]string, len(m.primaries))
	for i, p := range m.primaries {
		parts[i] = p.Name()
	}
	return "Mixin(" + strings.Join(parts, ",") + ")"
}

// Chat tries providers in order starting from `current`. On success, if the
// successful one is *not* the primary, sets a cooldown — within stickyMS we
// keep using that fallback; after stickyMS, we spring back to primary.
//
// Errors that are "retryable" (rate limits, network, 5xx) trigger fallback.
// We use a simple heuristic: any error matching certain substrings counts as
// retryable.
func (m *MixinProvider) Chat(ctx context.Context, msgs []Message, tools []ToolSpec, chunks chan<- string) (Response, error) {
	startIdx := m.pickStart()
	var lastErr error
	for off := 0; off < len(m.primaries); off++ {
		i := (startIdx + off) % len(m.primaries)
		p := m.primaries[i]

		// Tell the consumer which backend we're trying.
		select {
		case chunks <- fmt.Sprintf("\n[mixin] try=%s\n", p.Name()):
		default:
		}

		resp, err := p.Chat(ctx, msgs, tools, chunks)
		if err == nil {
			m.recordSuccess(i)
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return resp, err
		}
		select {
		case chunks <- fmt.Sprintf("\n[mixin] %s failed (retryable): %v; falling over\n", p.Name(), err):
		default:
		}
	}
	return Response{}, fmt.Errorf("all %d providers failed; last err: %w", len(m.primaries), lastErr)
}

// pickStart decides which provider index to start trying.
//
// - if cooldown has passed, always start from 0 (primary)
// - else, start from `current` (last-used backend)
func (m *MixinProvider) pickStart() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Now().After(m.cooldown) {
		m.current = 0
	}
	return m.current
}

// recordSuccess updates `current` and `cooldown`. If the winning provider is
// not the primary, set a sticky window during which we'll keep using it.
func (m *MixinProvider) recordSuccess(idx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = idx
	if idx == 0 {
		m.cooldown = time.Time{} // already on primary; no cooldown
		return
	}
	m.cooldown = time.Now().Add(time.Duration(m.stickyMS) * time.Millisecond)
}

// isRetryable inspects the error message for common retryable signals.
// Crude but matches the upstream's substring-based filter in MixinSession._pick.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{
		"timeout", "deadline", "rate limit", "429", "500", "502", "503", "504",
		"connection refused", "connection reset", "eof",
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	// Wrap chain too.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}
