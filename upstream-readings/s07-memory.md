# Upstream reading for s07 — memory

## File map

```
GenericAgent/
├── assets/
│   ├── sys_prompt.txt              ← L0 (English) / sys_prompt_en.txt
│   └── global_mem_insight_template.txt   ← seed for L1
├── memory/
│   ├── global_mem.txt              ← L2
│   ├── global_mem_insight.txt      ← L1
│   ├── tmwebdriver_sop.md          ← L3 (one of ~20 SOPs)
│   ├── plan_sop.md                 ← L3
│   ├── memory_management_sop.md    ← L3
│   ├── ...
│   ├── L4_raw_sessions/            ← L4 archive
│   └── skill_search/SKILL.md       ← s08 will explain
└── temp/
    ├── model_responses/<microsec>.txt   ← also L4
    └── <task-name>/                ← per-task scratch
```

## L0 — `assets/sys_prompt.txt`

A ~200-line markdown that defines the agent's persona and behavior rules:
- "Use code_run for arbitrary work"
- "Always prefer file_patch over file_write for edits"
- "When you finish, call no tool and return a summary"

This file is **the agent's constitution**. Worth reading entirely.

## L1 — `memory/global_mem_insight.txt`

A short routing index: "topics you might care about." Hand-edited by the
user. Loaded into every system prompt.

## L2 — `memory/global_mem.txt`

Stable accumulated knowledge. The agent itself appends to it via
`do_start_long_term_update` (a meta-tool that triggers a memory-consolidation
session).

## L3 — `memory/<topic>_sop.md`

These are the "skill" files. Each is a markdown procedural recipe for one
domain. Examples upstream has:

- `tmwebdriver_sop.md` — how to use the Chrome CDP browser tool
- `vue3_component_sop.md` — recipe for editing Vue 3 components
- `memory_management_sop.md` — meta-recipe for updating L1/L2
- `verify_sop.md` — verification checklist

These do **not** all load every turn. `skill_search/SKILL.md` is the index;
the agent calls a `skill_search` tool to fetch only the relevant one. s08
explains.

## L4 — `temp/model_responses/<microsec>.txt`

Every turn's full transcript is appended here. Survives across runs. The
`/resume` slash command reads this to find recent sessions.

## `do_update_working_checkpoint`

In `ga.py`, search for this name. It's the one-line tool that does:

```python
self.working['key_info'] = args['key_info']
```

`self.working` is a dict on the handler that survives across turns of the
**same task**, and gets re-injected into the next user message via
`turn_end_callback`. Per-task → reset on new task.

## Reading order

1. `assets/sys_prompt.txt` — the constitution
2. `agentmain.py:get_system_prompt` — assembly
3. `ga.py:turn_end_callback` (search for `def turn_end_callback`) — injection
4. `ga.py:do_update_working_checkpoint` — checkpoint write
5. `memory/memory_management_sop.md` — the upstream's own meta-doc on how
   memory should be used
