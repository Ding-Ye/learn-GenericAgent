# Appendix A — The Essence of Self-Evolution

## What does "Self-Evolving Agent" actually mean?

GenericAgent's README opens with:

> Self-evolving agent: grows skill tree from 3.3K-line seed, achieving full
> system control with 6x less token consumption.

"Self-evolving" sounds mystical. But after reading the source you find:
**it's a specific, observable engineering behavior**. This appendix breaks
it into 3 steps.

## Step 1: What it isn't

**Not**:
- Online training (fine-tuning during deploy)
- Model weights modifying themselves
- Neural architecture evolution
- Anything ML-flavored "learning"

**Is**:
- Evolution at the **prompt + filesystem** level — **model weights stay
  frozen**

## Step 2: What carries the evolution

GenericAgent's evolution carriers are two file kinds in `memory/`:

1. **L2 (`global_mem.txt`)** — incremental fact accumulation
   - "User prefers pnpm over npm"
   - "This machine's Python is /opt/homebrew/bin/python3, not /usr/bin"
   - "Internal jira webhook is hooks.example.com/jira"

2. **L3 (`<topic>_sop.md`)** — procedural knowledge crystallization
   - "How to inject JS via Chrome CDP" — 10-step recipe
   - "How to write a Vue 3 component our linter accepts" — 20-line template
   - "How to verify a git commit didn't introduce regression" — 5-step
     checklist

L2 is facts; L3 is procedures. **Both are plain markdown files** — not
databases, not vectors. The agent edits them via `do_file_write` /
`do_file_patch` — the two tools you already learned in s06.

## Step 3: How does evolution happen "automatically"

The key design: there's a special SOP called `memory_management_sop.md` that
tells the agent:

> "After solving each non-trivial task, review the key decisions of this
> session; if any decision generalizes to future similar tasks, write it
> into L2; if any solution path can be packaged as a recipe, write it into
> `<topic>_sop.md`."

Plus a special tool `do_start_long_term_update` — when the user types
`/update_memory`, this tool fires and the agent spawns a **sub-session**
dedicated to memory consolidation.

Pseudocode:

```
on user task done:
    if task was non-trivial AND user not in a hurry:
        suggest "shall I crystallize this into memory?"

on /update_memory or do_start_long_term_update:
    sub-agent = spawn(memory_management_sop as task)
    sub-agent.review_recent_sessions()
    sub-agent.propose_L2_diffs()
    sub-agent.propose_L3_new_sops()
    user.review_and_apply()
```

**Result**: after a week of use, your GenericAgent's `memory/` looks
nothing like mine. It "grows into" your specialized version.

## Why this saves 6× tokens

vs. traditional agent harnesses (langchain / autogen / crewai):

| Dimension | Traditional | GenericAgent |
|-----------|-------------|--------------|
| Tool count | 100+ predefined | 9 atomic tools |
| Tool descriptions in prompt | Full schema for 100 | Full schema for 9 |
| Skill / capability | Hardcoded in framework | L3 markdown loaded on demand |
| Personalization | Stuffed in system prompt | Incrementally in L1/L2 files |
| Avg tokens per task | 30K-200K | 5K-30K |

**The savings aren't from the model running faster** — they're from
**unnecessary context being cut**. 9 atomic tools paired with `code_run` can
do almost anything (need to scrape a web page?
`code_run("python -c 'import requests; ...'")`). No need for 100
pre-wrapped tools. SOPs irrelevant to the current task aren't injected — s08's
on-demand recall.

## The price of self-evolution

It's not free:
- **Cold start is slow** — a fresh install knows nothing; the first few
  tasks are sluggish
- **Requires user-in-the-loop** — L2/L3 evolution needs occasional
  human "approve / reject"
- **Doesn't transfer across users** — your SOPs may not fit someone else's
  setup
- **Memory drift** — over time, accumulated facts become stale or
  contradictory; needs periodic GC

Upstream handles the last issue with `memory_cleanup_sop.md` — the agent
periodically cleans its own memory. An interesting recursion: an agent that
maintains the agent's own memory.

## What our learn version does and doesn't

| Self-evolution mechanism | Ours (s07/s08) | Upstream |
|--------------------------|----------------|----------|
| L0-L4 layered structure | ✅ Memory + SkillTree | ✅ |
| L2 incremental append | ✅ UpdateGlobalMem | ✅ |
| L3 SOP files | ✅ SkillTree.Load | ✅ |
| L3 on-demand recall | ✅ Search | ✅ |
| do_update_working_checkpoint | ✅ SetCheckpoint | ✅ |
| memory_management_sop meta-SOP | ❌ | ✅ |
| do_start_long_term_update meta-tool | ❌ | ✅ |
| /resume reads L4 | ❌ | ✅ |

We do the **mechanism** but not the **meta** — the meta (agent editing its
own memory) requires a real running agent and several user-facing rounds to
demonstrate. The static learn version can't prove it works, so we omit it.

But: spin up s_full + add a few markdown files + teach it to write to L2,
and **the evolution starts**. That's upstream's trick: **evolution isn't a
new mechanism, it's combining existing mechanisms (file_write / file_read /
tool calls) creatively**.

## Further reading

- Upstream's `memory/memory_management_sop.md` — the meta-SOP in full
- Upstream's `memory/autonomous_operation_sop.md` — when self-evolution
  should fire
- ["The Bitter Lesson"](http://www.incompleteideas.net/IncIdeas/BitterLesson.html)
  by Sutton — why prompt + files outperforms predefined tools
