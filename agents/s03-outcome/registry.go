package main

import (
	"context"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	funcs map[string]ToolFunc
	specs map[string]ToolSpec
}

func NewRegistry() *Registry {
	return &Registry{
		funcs: map[string]ToolFunc{},
		specs: map[string]ToolSpec{},
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

// Dispatch returns a StepOutcome. If the tool name is unknown, the outcome
// has NextPrompt set to "unknown tool: ..." so the model gets a chance to
// self-correct on the next turn.
func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]any, chunks chan<- string) StepOutcome {
	r.mu.RLock()
	fn, ok := r.funcs[name]
	r.mu.RUnlock()
	if !ok {
		return StepOutcome{
			Data:       nil,
			NextPrompt: "unknown tool: " + name,
		}
	}
	return fn(ctx, args, chunks)
}
