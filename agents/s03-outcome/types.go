package main

import "context"

type Message struct {
	Role      string
	Content   string
	ToolCalls []ToolCall
	ToolUseID string
	Name      string
}

type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Provider interface {
	Chat(ctx context.Context, msgs []Message, tools []ToolSpec, chunks chan<- string) (Response, error)
}

// StepOutcome is the s03 protagonist. Every tool returns one of these.
//
// Three fields, three semantics:
//
//   Data        — what we put into the next tool_result for the model.
//   NextPrompt  — if non-empty, becomes the next user message; if empty,
//                 the loop exits with reason TASK_DONE.
//   ShouldExit  — true means hard-exit with reason EXITED, regardless of
//                 NextPrompt.
//
// Upstream parallel: agent_loop.py:5-8
//   @dataclass
//   class StepOutcome:
//       data: Any
//       next_prompt: Optional[str] = None
//       should_exit: bool = False
type StepOutcome struct {
	Data       any
	NextPrompt string
	ShouldExit bool
}

// ToolFunc returns a StepOutcome instead of (any, error). Errors that the
// model should learn from become outcomes with NextPrompt="error message".
// Errors that should stop the loop become outcomes with ShouldExit=true.
type ToolFunc func(ctx context.Context, args map[string]any, chunks chan<- string) StepOutcome
