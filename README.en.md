# learn-GenericAgent

> A 10-chapter progressive Go re-implementation that lets you read your way through [lsdefine/GenericAgent](https://github.com/lsdefine/GenericAgent) (10K⭐ self-evolving agent framework).

[中文版](README.md) · [Upstream](https://github.com/lsdefine/GenericAgent)

---

## What this is

Upstream GenericAgent claims "3.3K-line core + 6× less token consumption." But
the source still has ~30 files and 5 abstraction layers — hard to see the
essence on first read.

This repo breaks upstream into 10 Go learning modules. Each chapter is a
**standalone runnable program**:

- Its own `go.mod` / `main.go` / unit tests
- No imports from prior chapters (we re-declare canonical types per chapter,
  with deliberate diffs — not a shared library)
- Bilingual six-section docs (Problem · Solution · How It Works · What Changed
  · Try It · Upstream Source Reading)
- The **Upstream Source Reading** in each chapter pairs to a real file at a
  real line range in the upstream repo

> Pedagogy adapted from [shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code):
> mental model first → ASCII diagram → 30–60 lines of core code → diff vs
> previous chapter → hands-on → upstream source reading.

## Curriculum

| #  | Module | What it teaches | Upstream | Status |
|----|--------|-----------------|----------|--------|
| s01 | [s01-loop](agents/s01-loop) | The minimal agent loop | `agent_loop.py:agent_runner_loop` | ✅ |
| s02 | s02-tools | Tool registry & dispatch | `agent_loop.py:BaseHandler` | ⏳ |
| s03 | s03-outcome | StepOutcome control flow | `agent_loop.py:StepOutcome` | ⏳ |
| s04 | s04-claude | Real Anthropic Claude provider | `llmcore.py:NativeClaudeSession` | ⏳ |
| s05 | s05-coderun | Streaming code execution tool | `ga.py:code_run` | ⏳ |
| s06 | s06-fileops | File read / write / patch tools | `ga.py:file_read/write/patch` | ⏳ |
| s07 | s07-memory | Layered memory + working checkpoint | `memory/`, `ga.py:do_update_working_checkpoint` | ⏳ |
| s08 | s08-skills | Skill tree & skill search | `memory/skill_search/SKILL.md` | ⏳ |
| s09 | s09-mixin | Multi-provider failover | `llmcore.py:MixinSession` | ⏳ |
| s10 | s10-reflect | Reflect mode & autonomous scheduling | `agentmain.py --reflect`, `reflect/scheduler.py` | ⏳ |
| s_full | Integration | Wire all 10 chapters into one end-to-end use case | `agentmain.py:GenericAgent` | ⏳ |
| Appendix A | The essence of self-evolution | The "skill crystallization" mental model | README + memory/ | ⏳ |
| Appendix B | Upstream source reading map | File-by-file → chapter cross-reference | whole repo | ⏳ |

✅ = available; ⏳ = coming.

## Quickstart

```bash
git clone https://github.com/Ding-Ye/learn-GenericAgent.git
cd learn-GenericAgent
cd agents/s01-loop
go run . -user "Hello?"
```

Tests:

```bash
go test -count=1 ./agents/...
```

Optional docs site:

```bash
cd web
npm install
npm run dev
# open http://localhost:3000
```

## Why read this

- **To understand "self-evolving agents"** — upstream's core idea is
  *skill crystallization*: distilling solved-task traces into reusable SOP
  markdown. This thread runs through s07 + s08 + Appendix A.
- **To write your own agent harness** — each chapter is ~250 lines of Go;
  copy-pasting them gets you a minimal runnable agent harness.
- **To make build-vs-buy decisions** — after reading, you can decide whether
  to fork GenericAgent, build on LangGraph / claude-code-sdk, or roll your own.
- **To practice Go** — translating Python generators to Go channels ten
  times sharpens your concurrency intuition.

## Who this is for

- You've read the README of LangGraph / autogen / claude-code but never the source
- You want to build an agent but feel constrained by an off-the-shelf framework
- You come from Python, are migrating to Go, and want a side-by-side practice project

## Acknowledgements

- Upstream: [lsdefine/GenericAgent](https://github.com/lsdefine/GenericAgent) (MIT)
- Pedagogy: [shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)

## License

[MIT](LICENSE)
