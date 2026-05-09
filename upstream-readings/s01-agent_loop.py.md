# Upstream reading for s01 — `agent_loop.py`

[Source on GitHub](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py) · upstream commit at the time of writing this guide is on `main`.

This file is the entire agent kernel: 100 LOC, no dependencies, defines `BaseHandler`, `StepOutcome`, and `agent_runner_loop`. Read it once before s02.

---

## Walkthrough by line range

### Lines 1-9 — imports + dataclass

```python
import json, re, os
from dataclasses import dataclass
from typing import Any, Optional

@dataclass
class StepOutcome:
    data: Any
    next_prompt: Optional[str] = None
    should_exit: bool = False
```

`StepOutcome` is the most important type in the whole project. Three fields.
We'll re-implement it in Go in **s03**.

### Lines 11-13 — generator helper

```python
def try_call_generator(func, *args, **kwargs):
    ret = func(*args, **kwargs)
    if hasattr(ret, '__iter__') and not isinstance(ret, (str, bytes, dict, list)): ret = yield from ret
    return ret
```

Lets a callback be either a normal function or a generator. If it returns an
iterable, transparently `yield from` it. Go-equivalent: just always pass a
`chan<- string` and let producers decide if they push or not.

### Lines 15-25 — `BaseHandler.dispatch`

```python
class BaseHandler:
    def tool_before_callback(self, tool_name, args, response): pass
    def tool_after_callback(self, tool_name, args, response, ret): pass
    def turn_end_callback(self, response, tool_calls, tool_results, turn, next_prompt, exit_reason): return next_prompt
    def dispatch(self, tool_name, args, response, index=0):
        method_name = f"do_{tool_name}"
        if hasattr(self, method_name):
            args['_index'] = index
            prer = yield from try_call_generator(self.tool_before_callback, tool_name, args, response)
            ret = yield from try_call_generator(getattr(self, method_name), args, response)
            _ = yield from try_call_generator(self.tool_after_callback, tool_name, args, response, ret)
            return ret
```

This is the **dispatch table by reflection** pattern. To register a tool, just
add a `do_<name>` method on a subclass. We'll re-implement this in **s02**.

### Lines 33-88 — `agent_runner_loop`

The core loop. Four blocks:

**Block 1 (lines 38-49)** — message init + turn counter + tool-cache reset every 10 turns:
```python
if turn%10 == 0: client.last_tools = ''
```

**Block 2 (lines 50-58)** — provider call + tool_calls extraction:
```python
response_gen = client.chat(messages=messages, tools=tools_schema)
response = yield from response_gen
if not response.tool_calls: tool_calls = [{'tool_name': 'no_tool', 'args': {}}]
else: tool_calls = [...]
```

**Block 3 (lines 60-79)** — per-tool dispatch loop, accumulates `outcome`s:
```python
for ii, tc in enumerate(tool_calls):
    ...
    gen = handler.dispatch(tool_name, args, response, index=ii)
    outcome = (yield from gen) if verbose else exhaust(gen)
    if outcome.should_exit: exit_reason = ...; break
    if not outcome.next_prompt: exit_reason = {...}; break
    next_prompts.add(outcome.next_prompt)
```

**Block 4 (lines 80-87)** — exit decision + next-message construction:
```python
if len(next_prompts) == 0 or exit_reason: break
next_prompt = handler.turn_end_callback(...)
messages = [{"role": "user", "content": next_prompt, "tool_results": tool_results}]
```

Note the very subtle bit on the last line: history is **not** kept in
`messages` here. It's kept inside `client.backend.history` (look in
`llmcore.py:NativeClaudeSession`). This loop sees only the new turn's payload.
That decoupling is a clever way to let the provider session manage compression
and L4 archival without the loop knowing.

### Lines 90-109 — text post-processing helpers

`_clean_content` strips noise from streamed assistant text (collapses long
code blocks, removes nested `<file_content>`/`<tool_use>` markers).
`_compact_tool_args` makes one-line tool-call pretty prints.

You don't need to port these in s01.

---

## What to take away before s02

1. **`StepOutcome` triple is the entire control flow**: `data` flows back as a
   tool result, `next_prompt` becomes the next user message, `should_exit`
   ends the loop. Three fields express every possible loop transition.

2. **`do_<tool_name>` reflection** is a tiny but powerful pattern. Add a
   method, get a tool. The Go equivalent will be a `map[string]ToolFunc` plus
   a `Register(name, fn)` method.

3. **Provider state lives in the provider, not in `messages`**. The loop only
   passes the new turn's payload. This is hygienic: it lets you swap providers
   between turns without losing context.

---

## Open the file yourself

```bash
curl -sL https://raw.githubusercontent.com/lsdefine/GenericAgent/main/agent_loop.py | less
# or:
git clone https://github.com/lsdefine/GenericAgent.git /tmp/ga && less /tmp/ga/agent_loop.py
```

Read it slowly, end to end. Should take about 20 minutes.
