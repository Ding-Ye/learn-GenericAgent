# Upstream reading for s08 — skill tree

## Files

- `memory/skill_search/SKILL.md` — the master index
- `memory/skill_search/skill_search/` — the nested meta-skill directory
- `memory/<topic>_sop.md` — concrete skills (~20 of them)

## Worth grepping

```bash
grep -l "Use this when" memory/*.md | head -20
```

Many SOPs follow the convention "first paragraph after H1 starts with
'Use this when ...'." That convention is what makes a substring search like
ours useful: the index becomes effectively a "when-to" table.

## Sample SOPs to read end-to-end

- `memory/tmwebdriver_sop.md` — concrete browser-automation steps
- `memory/plan_sop.md` — meta-recipe for plan files
- `memory/memory_management_sop.md` — meta-recipe for editing L1/L2

Reading 2-3 of these gives you a feel for what "skill-shaped knowledge"
looks like in practice.

## How upstream's agent decides which skill to load

There is no learned classifier. The model just reads `SKILL.md`, sees the
one-line summaries, and decides. This works because:

- The index is short enough to fit
- Each SOP's summary is well-structured ("Use this when ...")
- The model is competent enough to do the matching itself

This is a **strong design lesson**: not every retrieval problem needs an
embedding model. Sometimes a markdown index plus a competent reader is
enough.

## Reading order

1. `memory/skill_search/SKILL.md`
2. Pick 2-3 SOPs and read them end-to-end
3. Look at how `agentmain.py:get_system_prompt` decides what to inject
   (hint: it injects the L1 insight + L2 global, not L3 SOPs)
4. Trace one task in `temp/model_responses/` to see the agent calling
   `file_read memory/<topic>_sop.md`
