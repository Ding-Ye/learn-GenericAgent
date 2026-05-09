# s02 — Tool Registry & Dispatch

## Problem

The s01 loop ends after one turn. But a real agent needs to call tools: read
files, run code, fetch web pages. How does the model "call" a tool?

The answer is in the OpenAI / Anthropic APIs — the response includes a
`tool_calls` array, each item with a `name` and `arguments`. We need to:

1. Tell the provider what tools are available
2. On each turn, dispatch each tool call to the corresponding Go function
3. Feed the results back as the next turn's input

Upstream `agent_loop.py` does this via Python reflection — the `do_<tool_name>`
method-name convention. Go isn't that dynamic, but offers something nicer:
an **explicit registry**.

## Solution

```
   ┌─────────────────────┐
   │  Registry           │
   │  ───────────────    │
   │  map[name]ToolFunc  │ ◀──── reg.Register("echo", echoFn)
   │  map[name]ToolSpec  │
   └──────────┬──────────┘
              │
              ▼ Dispatch(name, args)
        ┌──────────┐
        │ ToolFunc │  func(ctx, args, chunks) (any, error)
        └──────────┘

   loop:
      resp := provider.Chat(msgs, reg.Specs(), chunks)
      if len(resp.ToolCalls) == 0: break
      for tc := range resp.ToolCalls:
          data, err := reg.Dispatch(tc.Name, tc.Args, chunks)
          msgs = append(msgs, Message{Role:"tool", ..., Content: marshal(data)})
```

`Registry` is the source of truth: at register time we land both `ToolSpec`
(schema) and `ToolFunc` (implementation). The loop pulls `Specs()` for the
provider and `Dispatch()` for execution.

## How It Works

The `ToolFunc` signature is the most important line in this chapter:

```go
type ToolFunc func(ctx context.Context,
                  args map[string]any,
                  chunks chan<- string) (data any, err error)
```

Why three return paths (data + error + streamed via chunks) instead of a
single `StepOutcome`? Because s02 hasn't introduced `StepOutcome` yet — s03
will fold flow control into outcome. This chapter only handles **invocation**;
we keep the shape close to "ordinary Go function."

`Dispatch` is four lines:

```go
fn, ok := r.funcs[name]
if !ok { return nil, fmt.Errorf("unknown tool: %q", name) }
return fn(ctx, args, chunks)
```

If the model hallucinates a nonexistent tool, we **don't return an error to
the loop** — we feed the error string back as a tool_result so the model can
self-correct. That's a common agent-harness trick: errors are inputs too.

## What Changed

Precise diff vs s01:

| File | Change |
|------|--------|
| `types.go` | `Message` gains `ToolCalls/ToolUseID/Name`; `Response` gains `ToolCalls`; `Provider.Chat` gains `tools []ToolSpec` |
| `loop.go` | No longer single-turn-and-exit; dispatches tool calls, accumulates tool messages |
| `mock.go` | `MockReply{Text, ToolCalls}` replaces plain string |
| **`registry.go`** | New — the source of truth for dispatch |

## Try It

```bash
cd agents/s02-tools
go run .
```

You should see:

```
--- Turn 1 ---

🛠 echo(map[text:hello])
[echo] hello

--- Turn 2 ---

🛠 upper(map[text:loop])

--- Turn 3 ---
all done

[exit] reason=TASK_DONE turns=3
```

Experiment: change MockProvider to script a `nonexistent` tool call and
watch the loop survive — the "unknown tool" error is fed back to the model.

## Upstream Source Reading

Upstream uses reflection (`hasattr` + `getattr`) for dispatch:

```python
# agent_loop.py:18-25 ── BaseHandler.dispatch
def dispatch(self, tool_name, args, response, index=0):
    method_name = f"do_{tool_name}"
    if hasattr(self, method_name):
        args['_index'] = index
        prer = yield from try_call_generator(self.tool_before_callback, tool_name, args, response)
        ret = yield from try_call_generator(getattr(self, method_name), args, response)
        _ = yield from try_call_generator(self.tool_after_callback, tool_name, args, response, ret)
        return ret
```

Notice three hooks: `tool_before_callback`, `tool_after_callback`, and an
outer `turn_end_callback`. Our s02 `Registry.Dispatch` has none of those.
Why: upstream's hooks mainly serve working-checkpoint injection and plan-mode
checks — high-level features that arrive in s07 and s_full. s02's job is just
to make "name → invocation" work.

Tool registration upstream is "add a method":

```python
# ga.py (inside the GenericAgentHandler class)
class GenericAgentHandler(BaseHandler):
    def do_code_run(self, args, response): ...
    def do_file_read(self, args, response): ...
    def do_file_write(self, args, response): ...
    def do_web_scan(self, args, response): ...
    # nine do_* methods → nine tools
```

Our Go translation is explicit `reg.Register(spec, fn)`. Trade-offs:
- Python reflection: shorter to write, but the tool list scatters and IDE
  jumps are harder
- Go registry: one extra `Register` line, but the tool list is centrally
  visible and spec/fn are tightly bound

Next: [s03-outcome](s03-outcome.md) — bring in `StepOutcome` so tools can
shape the loop's flow.
