# Upstream reading for s03 — `StepOutcome` and the loop's flow control

[`agent_loop.py:5-8`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L5-L8) defines a 4-line dataclass that drives the entire loop's flow control. It is the smallest type in GenericAgent and arguably its most important.

## Definition (verbatim)

```python
@dataclass
class StepOutcome:
    data: Any
    next_prompt: Optional[str] = None
    should_exit: bool = False
```

## How it's consumed

In `agent_runner_loop`, after every tool dispatch:

```python
# agent_loop.py:60-79
gen = handler.dispatch(tool_name, args, response, index=ii)
outcome = ...  # extract from generator's StopIteration.value

if outcome.should_exit:
    exit_reason = {'result': 'EXITED', 'data': outcome.data}; break
if not outcome.next_prompt:
    exit_reason = {'result': 'CURRENT_TASK_DONE', 'data': outcome.data}; break
if outcome.data is not None and tool_name != 'no_tool':
    datastr = json.dumps(outcome.data, ensure_ascii=False, default=json_default) \
              if type(outcome.data) in [dict, list] else str(outcome.data)
    tool_results.append({'tool_use_id': tid, 'content': datastr})
next_prompts.add(outcome.next_prompt)
```

## State machine

```
                       ┌──────────────────┐
                       │ tool returns     │
                       │ StepOutcome(d,n,e)│
                       └────────┬─────────┘
                                ▼
                ┌───────────────────────────────┐
                │  e == True?                   │
                │     yes → EXITED, data=d      │ ◀─ hard stop
                │     no                        │
                └────────┬──────────────────────┘
                         ▼
                ┌───────────────────────────────┐
                │  n == "" or None?             │
                │     yes → TASK_DONE, data=d   │ ◀─ no further hint
                │     no                        │
                └────────┬──────────────────────┘
                         ▼
                ┌───────────────────────────────┐
                │  add d to tool_results        │
                │  add n to next_prompts (set)  │
                │  loop continues               │
                └───────────────────────────────┘
```

## Why this matters

Most agent harnesses expose flow control as a separate "exit signal" / 
"continuation" / "error" — three or four mechanisms. By collapsing to one
struct with three fields, GenericAgent achieves:

- **Single return shape**: every `do_<tool>` returns the same type. No
  function variants. No "tool that exits" vs "tool that returns data".
- **Composable callbacks**: the lifecycle hooks
  (`tool_before_callback` etc.) all share the same outcome type.
- **Explicit error path**: there is no Python `raise` flowing through the
  loop. Tools that *want* to surface an error to the model do so by setting
  `next_prompt = "error: ..."`. Tools that crash unexpectedly throw, and that
  is a different (and rare) bug class.

## Where this contract appears in real `do_*` methods

Every `ga.py:do_<tool>` returns one. Sample (paraphrased):

```python
# ga.py: do_file_patch (paraphrased)
def do_file_patch(self, args, response):
    path = args['path']; old = args['old_content']; new = args['new_content']
    yield f'[Patch] {path}\n'
    try:
        file_patch(path, old, new)
        return StepOutcome(data='success')
    except Exception as e:
        return StepOutcome(
            data=str(e),
            next_prompt=f'patch failed for {path}: {e}',
        )
```

The "found-block-not-matching" path doesn't `raise` — it returns an outcome
with `next_prompt`. The model sees the failure on the next turn and can fix
its diff.

## Read these line ranges

- [`agent_loop.py:5-8`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L5-L8) — the type
- [`agent_loop.py:60-87`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L60-L87) — the consumer
- Sample `do_*` methods in `ga.py` — search for `return StepOutcome(`
