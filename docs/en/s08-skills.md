# s08 — Skill Tree & Skill Search

## Problem

s07's L3 is a collection of `*_sop.md` files — browser automation SOP,
Vue 3 component SOP, memory-management SOP, etc. Upstream has ~20. If we
inject all 20 into the system prompt every turn:
- Wastes tokens (most are irrelevant to the current task)
- Adds noise (off-topic SOPs distract the model)
- Doesn't fit the context window

The right move is **load on demand**:
1. System prompt holds only the **index** (one-line summary per SOP)
2. Model decides "I need SOP X" → calls a `skill_search` tool
3. Tool returns relevant SOP body
4. Model proceeds based on it

That's exactly upstream's `memory/skill_search/SKILL.md` design.

## Solution

```
   /tmp/skills/                          ┌─────────────────┐
   ├── browser_sop.md      ──── scan ──► │ SkillTree       │
   ├── plan_sop.md                       │ skills []Skill  │
   ├── verify_sop.md                     └─────────────────┘
                                                   │
                                                   ▼
                              List() ── index ── system prompt
                              Search(query) ── tool: skill_search → matches
                              Load(name)   ── tool: load_skill   → one SOP body
```

`Skill` carries 4 fields:

```go
type Skill struct {
    Name    string  // "browser"
    Path    string  // "/tmp/skills/browser_sop.md"
    Title   string  // "Browser SOP" (first H1 in file; falls back to Name)
    Summary string  // first non-empty paragraph after H1
}
```

## How It Works

`Scan()` reads the directory, filters `_sop.md` files, parses the head to get
Title/Summary. The output is a sorted `[]Skill`.

`Search()` uses the simplest substring match — case-folded, joining
name+title+summary. **In production** you'd swap in BM25 or vector search;
this chapter aims for "good enough," since the abstraction allows replacement.

`Load()` only reads the file body after a name match — avoiding loading all
20 SOPs into memory at scan time.

## What Changed

New module. Three APIs: List / Search / Load.

| File | Content |
|------|---------|
| `skills.go` | `SkillTree` type + `Scan/List/Search/Load` |
| `main.go` | CLI |
| `skills_test.go` | 6 tests |

## Try It

```bash
cd agents/s08-skills
mkdir /tmp/skills
cat > /tmp/skills/browser_sop.md <<EOF
# Browser SOP

Use this when you need to drive a real Chrome via CDP.

Steps:
1. ...
EOF

cat > /tmp/skills/plan_sop.md <<EOF
# Planning SOP

Write a plan file before any multi-step task.
EOF

go run . -dir /tmp/skills -op list
# Shows the index: one line per SOP

go run . -dir /tmp/skills -op search -q chrome
# Hits: browser

go run . -dir /tmp/skills -op load -q browser
# Prints browser_sop.md body
```

Experiment: write a SOP whose summary contains two keywords ("chrome
browser planning") and watch different queries each return it.

## Upstream Source Reading

Upstream's skill-search design is `memory/skill_search/SKILL.md`:

```markdown
# Skill Search

This file is the skill index. Each entry below points to a markdown SOP in
the parent memory/ directory.

- `tmwebdriver_sop.md` — Use this for browser automation via Chrome CDP
- `plan_sop.md` — Use this when writing or following a multi-step plan
- `vue3_component_sop.md` — Use this when editing Vue 3 component code
- `memory_management_sop.md` — Use this when consolidating L1/L2 memory
- `verify_sop.md` — Use this after edits, to verify with tests
- ...
```

Note that upstream has **no dedicated `skill_search` tool** — it simply lets
the model use `file_read memory/skill_search/SKILL.md` to scan the index,
then `file_read memory/<topic>_sop.md` to load the chosen one.

Our s08 abstracts those two steps into `tree.Search()` / `tree.Load()`
methods, but **structurally it's the same idea**: "put the on-demand layer
behind tool calls + file ops, not the system prompt."

The `memory/skill_search/skill_search/` nested directory holds meta-meta
data (how to search the skill index itself). s08 doesn't reproduce that
layer of meta.

Next: [s09-mixin](s09-mixin.md) — multi-provider failover (Go port of
MixinSession).
