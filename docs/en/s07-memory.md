# s07 — Layered Memory + Working Checkpoint

## Problem

Long-running agents have memory issues:
- The user once said "use yarn, not npm" — how do we persist that?
- After 100 turns of decisions, the conversation history overflows — how
  do we compress?
- A "key fact" remembered at turn 5 needs to still be visible at turn 50.

Stuffing everything into the system prompt doesn't work (200K context fills
fast). GenericAgent's solution is **layering** — slice information into 5
tiers by "change frequency + load scope."

## Solution

```
   L0 hardcoded                ◀── never changes. This chapter defines sysPromptBase.
   L1 insight (routing index)  ◀── occasionally hand-edited. memory/global_mem_insight.txt.
   L2 global_mem (stable kb)   ◀── agent appends incremental facts.
   L3 SOP markdown             ◀── a set of "skill" recipes, loaded on demand. s08 dives in.
   L4 archived sessions        ◀── full traces of completed runs. /resume reads this.

   working checkpoint          ◀── cross-turn-but-same-task scratchpad
                                  (in-memory variable, not persisted)
```

Before each request, `AssembleSystemPrompt()` glues:

```
<L0>

[L1 Insight]
<L1 content>

[L2 Global Memory]
<L2 content>

[Working checkpoint, set 30s ago]
<checkpoint>
```

L3 isn't dumped in full — s08 covers per-query SOP selection. L4 isn't in
the prompt at all; it's the archive read by the `/resume` command.

## How It Works

5 APIs + 1 assembler. Core storage is the filesystem:

```go
type Memory struct {
    dir          string
    mu           sync.RWMutex
    checkpoint   string
    checkpointTS time.Time
}

func (m *Memory) UpdateGlobalMem(line string) error
func (m *Memory) L1Insight() (string, error)
func (m *Memory) L2Global() (string, error)
func (m *Memory) L3LoadSOP(topic string) (string, error)
func (m *Memory) L3ListSOPs() ([]string, error)
func (m *Memory) L4AppendSession(content string) error
func (m *Memory) SetCheckpoint(s string)
func (m *Memory) Checkpoint() (string, time.Time)
func (m *Memory) AssembleSystemPrompt() string
```

The lock around `SetCheckpoint` exists because in s_full integration, `do_*`
tools may update the working checkpoint concurrently while the loop reads it
to assemble the prompt.

## What Changed

A standalone module. No new loop or tool concepts — just the `Memory` type.

| File | Content |
|------|---------|
| `memory.go` | `Memory` type + L0..L4 + assembler |
| `main.go` | CLI: `go run . -op show / append / checkpoint` |
| `memory_test.go` | 7 tests |

## Try It

```bash
cd agents/s07-memory
go run . -dir /tmp/mem -op show
# Shows the empty initial prompt (L1 and L2 are empty)

go run . -dir /tmp/mem -op append -value "Rule: prefer pnpm over npm in this repo"
go run . -dir /tmp/mem -op append -value "Convention: tests live in *_test.go"
go run . -dir /tmp/mem -op show
# Now the L2 Global Memory section has content
```

Experiment: hand-write `/tmp/mem/browser_sop.md` and watch `L3ListSOPs()`
return it (s08 will use this).

## Upstream Source Reading

The five tiers live in different upstream files:

| Tier | Upstream location |
|------|--------------------|
| L0 | `assets/sys_prompt.txt`, read by `agentmain.py:get_system_prompt` |
| L1 | `memory/global_mem_insight.txt` |
| L2 | `memory/global_mem.txt` |
| L3 | `memory/<topic>_sop.md` (~20 SOP files; s08 dives in) |
| L4 | `temp/model_responses/<microsec>.txt` |

`agentmain.py:get_system_prompt` does the assembly:

```python
def get_system_prompt():
    with open(f'assets/sys_prompt{lang_suffix}.txt') as f: prompt = f.read()
    prompt += f"\nToday: {time.strftime('%Y-%m-%d %a')}\n"
    prompt += get_global_memory()  # reads global_mem.txt + global_mem_insight.txt
    return prompt
```

Working-memory injection happens in `agent_loop.py:turn_end_callback`
(overridden by `ga.py:GenericAgentHandler`):

```python
# ga.py: GenericAgentHandler.turn_end_callback (paraphrased)
def turn_end_callback(self, response, tool_calls, tool_results, turn, next_prompt, exit_reason):
    if 'key_info' in self.working:
        next_prompt = next_prompt + f"\n[Working memory]\n{self.working['key_info']}\n"
    return next_prompt
```

Note: upstream injects working checkpoint into the **next user message**
(next_prompt), not the system prompt. Our s07 puts it in the system —
either works. Why upstream chose user message: the system prompt is cached
inside the ToolClient layer; user messages are rebuilt every turn, so
checkpoint changes take effect immediately.

`do_update_working_checkpoint` is a one-line tool (`ga.py`):

```python
def do_update_working_checkpoint(self, args, response):
    self.working['key_info'] = args['key_info']
    yield f'[Checkpoint] {args["key_info"][:60]}...\n'
    return StepOutcome(data='checkpoint updated')
```

Next: [s08-skills](s08-skills.md) — the skill tree (L3) and how
`skill_search` recalls relevant SOPs by query.
