# s03 — StepOutcome Control Flow

## Problem

In s02 a `ToolFunc` returns `(data any, err error)`. But that can't express:
- **"I'm done; loop, please stop"** — e.g. a `finish_task` tool
- **"I finished one step; loop, run another turn with this hint"** — e.g.
  `code_run` failed and we want the model to retry
- **"I had a semantic error (not a panic); feed it back so the model can
  self-correct"** — e.g. `file_patch` couldn't find the `old` block

If we only have `error`, the loop can't distinguish "program crashed" from
"semantic signal." Upstream GenericAgent's solution is `StepOutcome`:
**three fields encoding three distinct semantics**.

## Solution

```go
type StepOutcome struct {
    Data       any    // shown to the model as the tool_result
    NextPrompt string // non-empty → next user message; empty → task done
    ShouldExit bool   // true → terminate loop immediately
}
```

Three fields, three loop exits:

```
   tool returns StepOutcome
            │
            ├── ShouldExit=true ─────────► EXITED
            │
            ├── NextPrompt == "" ────────► TASK_DONE (current micro-task done)
            │
            └── NextPrompt != "" ────────► loop continues, user msg = NextPrompt
```

## How It Works

`Registry.Dispatch` now returns `StepOutcome`. An unknown tool no longer
returns an `error` — it returns `StepOutcome{NextPrompt: "unknown tool: <name>"}`
so the model gets the error message on the next turn.

After dispatching all tool_calls in a turn, `Run` does three things:

```go
// 1. Collect outcomes
for _, tc := range resp.ToolCalls {
    oc := registry.Dispatch(...)
    outcomes = append(outcomes, oc)
}

// 2. Any ShouldExit=true → exit now
for _, oc := range outcomes {
    if oc.ShouldExit { return ExitInfo{Reason:"EXITED", Data: oc.Data}, nil }
}

// 3. Collect all non-empty NextPrompts (deduplicated) as next user msg
nextPrompt := joinUnique(outcomes, "\n")
if nextPrompt == "" { return ExitInfo{Reason: "TASK_DONE"}, nil }
msgs = append(msgs, Message{Role:"user", Content: nextPrompt})
```

Step 3 is the **multi-tool merge semantics** that matters: if the model called
3 tools in one turn and each returned a different NextPrompt, the loop joins
them with `\n` into one user message. If all three returned the *same*
NextPrompt, dedup keeps just one — no redundant hint spam.

## What Changed

Precise diff vs s02:

| File | Change |
|------|--------|
| `types.go` | New `StepOutcome` type; `ToolFunc` signature changes from `(any, error)` to `StepOutcome` |
| `registry.go` | `Dispatch` returns `StepOutcome`; unknown tool surfaces as `NextPrompt`, not `error` |
| `loop.go` | Main loop adds outcome aggregation (exit / done / next_prompt branches) |

## Try It

```bash
cd agents/s03-outcome
go run .
```

Output:

```
--- Turn 1 ---

🛠 think_again(map[hint:consider edge case])
--- Turn 2 ---

🛠 finish_task(map[summary:all checks passed])

[finish] all checks passed

[exit] reason=EXITED turns=2 data=all checks passed
```

Experiment: change `finish_task`'s `ShouldExit: true` to `false`. Watch the
loop fall through to the "empty NextPrompt → TASK_DONE" branch instead.

## Upstream Source Reading

Upstream's `StepOutcome` is at `agent_loop.py:5-8` — a 4-line dataclass:

```python
# agent_loop.py:5-8
@dataclass
class StepOutcome:
    data: Any
    next_prompt: Optional[str] = None
    should_exit: bool = False
```

Consumed in the loop at `agent_loop.py:60-79`:

```python
for ii, tc in enumerate(tool_calls):
    ...
    gen = handler.dispatch(tool_name, args, response, index=ii)
    try:
        v = next(gen)
        def proxy(): yield v; return (yield from gen)
        outcome = (yield from proxy()) if verbose else exhaust(proxy())
    except StopIteration as e: outcome = e.value

    if outcome.should_exit:
        exit_reason = {'result': 'EXITED', 'data': outcome.data}; break
    if not outcome.next_prompt:
        exit_reason = {'result': 'CURRENT_TASK_DONE', 'data': outcome.data}; break
    if outcome.next_prompt.startswith('未知工具'): client.last_tools = ''
    if outcome.data is not None and tool_name != 'no_tool':
        datastr = ...
        tool_results.append({'tool_use_id': tid, 'content': datastr})
    next_prompts.add(outcome.next_prompt)
```

Notice three details:

1. **`StopIteration.value` carries the outcome** — Python generators stuff their
   return value into a `StopIteration` exception. Go uses a channel for chunks
   and a plain return for the outcome — cleaner.

2. **`if outcome.next_prompt.startswith('未知工具'): client.last_tools = ''`**
   — on unknown-tool errors, the client's tool-description cache is wiped to
   force the next turn to resend the full tool spec. A small upstream
   optimization; we don't replicate it in s03 — it's deferred to s07
   (memory & cache).

3. **`next_prompts.add(...)`** — Python `set` auto-dedups. We use
   `map[string]struct{}` for the same effect.

Next: [s04-claude](s04-claude.md) — finally swap in a real model: native HTTP
calls to the Anthropic Claude API.
