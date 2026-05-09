# Upstream reading for s02 — `BaseHandler` & friends

[`agent_loop.py:15-25`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L15-L25) defines the dispatch contract; [`ga.py`](https://github.com/lsdefine/GenericAgent/blob/main/ga.py) packs every concrete `do_<tool>` onto one subclass.

## The dispatch contract (10 lines)

```python
class BaseHandler:
    def tool_before_callback(self, tool_name, args, response): pass
    def tool_after_callback(self, tool_name, args, response, ret): pass
    def turn_end_callback(self, response, tool_calls, tool_results, turn,
                          next_prompt, exit_reason): return next_prompt
    def dispatch(self, tool_name, args, response, index=0):
        method_name = f"do_{tool_name}"
        if hasattr(self, method_name):
            args['_index'] = index
            prer = yield from try_call_generator(self.tool_before_callback, tool_name, args, response)
            ret = yield from try_call_generator(getattr(self, method_name), args, response)
            _ = yield from try_call_generator(self.tool_after_callback, tool_name, args, response, ret)
            return ret
```

Three things to notice:

1. **Tool name → method name is direct (`f"do_{tool_name}"`)**. No registration call. Adding a tool means adding a method.
2. **Three lifecycle hooks**. `tool_before_callback` and `tool_after_callback` are called per tool call; `turn_end_callback` is called per turn. The `turn_end_callback` *returns* the `next_prompt`, so it can rewrite the next user message — used to inject working-memory hints, memory insights, etc.
3. **Generators all the way down**. Each callback uses `yield from`, meaning callbacks can stream their own progress back to the user.

## What's piled on top in `ga.py`

`GenericAgentHandler(BaseHandler)` is one ~900-line class with these tool methods:

| `do_*` | What it does | Will be in our session |
|--------|---------------|------------------------|
| `do_code_run` | Run python/bash, stream stdout | s05 |
| `do_ask_user` | Pop a CLI prompt and read a line | (omitted — interactive) |
| `do_web_scan` | Read tabs + simplified HTML from a CDP-controlled Chrome | (stubbed in s_full) |
| `do_web_execute_js` | Inject JS via CDP | (stubbed in s_full) |
| `do_file_patch` | Replace `old` block with `new` block | s06 |
| `do_file_write` | Write/append/prepend | s06 |
| `do_file_read` | Read with line range or keyword | s06 |
| `do_update_working_checkpoint` | Save scratchpad | s07 |
| `do_no_tool` | Empty-reply placeholder | s03 |
| `do_start_long_term_update` | Trigger memory consolidation | s07 |

Plus override hooks `turn_end_callback` (injects memory insights, enforces turn limits) and `tool_before_callback` (logs to file, blocks unsafe ops in plan mode).

## Read these line ranges

- [`agent_loop.py:15-25`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L15-L25) — the contract (10 lines)
- [`ga.py:do_code_run`](https://github.com/lsdefine/GenericAgent/blob/main/ga.py) — search the file for `def do_code_run` (one example tool)
- [`ga.py:turn_end_callback`](https://github.com/lsdefine/GenericAgent/blob/main/ga.py) — search `def turn_end_callback` (one example hook)

## Mapping to the Go `Registry`

| Python | Go (s02) |
|--------|----------|
| `def do_<name>` on a subclass | `reg.Register(spec, fn)` call |
| `hasattr` + `getattr` lookup | `r.funcs[name]` map lookup |
| `tool_before_callback` | (deferred to s07) |
| `turn_end_callback` returning a string | (deferred to s03 as `StepOutcome.NextPrompt`) |
| `args['_index'] = index` | We pass an explicit `index int` if needed (s_full) |

The Go form is more verbose by ~3 lines per tool. In exchange you get static
type safety on the spec and easy-to-grep registration sites.
