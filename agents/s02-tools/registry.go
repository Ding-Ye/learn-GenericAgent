package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolFunc is the signature of a registered tool. It runs synchronously,
// streams text on `chunks`, and returns its data + an error.
//
// Upstream parallel:
//   GenericAgent's tools are Python generators that `yield` chunks and return
//   a StepOutcome via StopIteration.value. We split that into two channels:
//   `chunks` for streaming text and the (data, error) tuple for the final.
//   The StepOutcome elaboration arrives in s03.
type ToolFunc func(ctx context.Context, args map[string]any, chunks chan<- string) (data any, err error)

// Registry is the canonical Go translation of `BaseHandler.dispatch` — a map
// from tool name to a Go func, plus a Specs() helper that emits the schema
// list to send to the provider.
type Registry struct {
	mu    sync.RWMutex
	funcs map[string]ToolFunc
	specs map[string]ToolSpec
}

func NewRegistry() *Registry {
	return &Registry{
		funcs: make(map[string]ToolFunc),
		specs: make(map[string]ToolSpec),
	}
}

func (r *Registry) Register(spec ToolSpec, fn ToolFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs[spec.Name] = fn
	r.specs[spec.Name] = spec
}

func (r *Registry) Specs() []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ToolSpec, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s)
	}
	return out
}

// Dispatch is the s02 equivalent of upstream `BaseHandler.dispatch`:
// look up by name and call. Unknown tool name → an error message that the
// caller should feed back as a tool_result so the model can self-correct.
func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]any, chunks chan<- string) (any, error) {
	r.mu.RLock()
	fn, ok := r.funcs[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown tool: %q", name)
	}
	return fn(ctx, args, chunks)
}

// MarshalToolResult serialises whatever the tool returned into a string
// that's safe to put back into the conversation as a tool_result content.
func MarshalToolResult(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("[unmarshalable result: %v]", err)
	}
	return string(b)
}
