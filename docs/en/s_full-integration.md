# s_full — End-to-End Integration

## Wiring s01-s10 into one runnable path

Each chapter s01-s10 is its own Go module with no cross-imports. The
upside: every chapter is self-contained for teaching. The downside: we lose
**one look at "how they work together."** This appendix patches that with
an ASCII diagram + a 16-step trace.

## Full-stack architecture

```
                ┌─────────────────────────────────────────┐
                │    main()                               │
                │      ▼                                  │
                │   ReflectLoop ◀─── (s10)                │
                │      ▼                                  │
                │   Run(ctx, task)                        │
                │      ▼                                  │
                │   AgentRunner                           │
                │   ┌─────────────────────────────────┐   │
                │   │  agent.Loop ◀─── (s01..s03)     │   │
                │   │   ▼                              │   │
                │   │  Provider ◀─── (s04 + s09)       │   │
                │   │  Mixin([Anthropic, OpenAI])     │   │
                │   │   ▲                              │   │
                │   │   │                              │   │
                │   │  Registry ◀─── (s02)             │   │
                │   │   ├── code_run    (s05)          │   │
                │   │   ├── file_read   (s06)          │   │
                │   │   ├── file_write  (s06)          │   │
                │   │   ├── file_patch  (s06)          │   │
                │   │   ├── skill_search (s08)         │   │
                │   │   ├── load_skill  (s08)          │   │
                │   │   └── update_checkpoint (s07)    │   │
                │   │                                  │   │
                │   │  Memory  ◀──── (s07 + s08)       │   │
                │   │   ├── L0 sysPromptBase            │   │
                │   │   ├── L1 insight (file)           │   │
                │   │   ├── L2 global (file)            │   │
                │   │   ├── L3 SOPs (skill tree)        │   │
                │   │   └── L4 sessions (archive)       │   │
                │   │                                  │   │
                │   └─────────────────────────────────┘   │
                └─────────────────────────────────────────┘
```

Each chapter's "part" is annotated. Notice:

- **Provider** is an interface; s04 writes one concrete impl, s09 a Mixin
  composer, s_full wires them together.
- **Registry** is s02's contribution; s05/s06/s07/s08 each register a few
  ToolFuncs into it.
- **Memory** is s07's contribution; s08's SkillTree slots into Memory's L3
  interface.

## A 16-step execution trace (real use case)

**User scenario**: in reflect mode, the agent watches a GitHub repo's CI
status. When CI goes red, the agent auto-fetches the error log, locates the
issue, applies a fix, runs tests, commits, and pushes.

```
Step  Layer        Detail                                                Upstream
────  ──────────   ──────────────────────────────────────────────────────────────
 1   reflect      JSONCheck("/etc/agent/ci.json") sees status=red         s10
 2   reflect      ReflectLoop dispatches task = "CI is red on abc; fix"  s10
 3   loop         Run(ctx, task) enters main loop turn 1                  s01
 4   memory       AssembleSystemPrompt() → L0+L1+L2 + current checkpoint s07
 5   provider     MixinProvider.Chat() tries AnthropicProvider first      s09→s04
 6   provider     SSE: model emits tool_call skill_search("CI debug")     s04
 7   loop         non-empty ToolCalls → dispatch first one                 s02
 8   tool         skill_search finds ci_sop.md in SkillTree, returns idx   s08
 9   loop         outcome.NextPrompt = "use load_skill to read..."         s03
10   loop         turn 2 starts; msgs accumulated tool_result + nextPrompt s01
11   provider     Claude returns tool_call: load_skill("ci_sop")           s04
12   tool         reads memory/ci_sop.md body, returns                     s08
13   loop         turn 3: model now calls code_run("git log ...")          s05
14   tool         code_run streams git log output via chunks               s05
15   loop         turn N: file_patch fixes a line, code_run runs tests     s06+s05
16   loop         model stops calling tools; final text → ExitInfo TASK_DONE s03
```

Notice step 8: **the SOP isn't preloaded into the system prompt**. The
model self-determines it needs the "CI debugging" skill at step 6 and pulls
it via tool call. That's the heart of s08's skill-tree design — load on
demand.

## What "small things, big power" looks like

Across 10 modules, ~2700 lines of Go. Each chapter is small:
- s01 is just 50 lines of loop
- s02 is just 60 lines of registry
- s03 is 4 lines of dataclass + 30 lines of outcome aggregation

Yet combined, s_full is a **complete runnable agent harness** — calls real
models, calls real tools, has memory, can be auto-triggered. That's the
essence of upstream's "3K-line agent that does things."

## What we deliberately omit

| Upstream feature | Why omitted | How to add it |
|------------------|-------------|---------------|
| simphtml.py | 1000-line HTML processor, out of scope | Use golang.org/x/net/html for a mini |
| TMWebDriver | OS-coupled, needs real Chrome | Use chromedp library to write a do_browser |
| Multi-modal vision_api | Domain-specific | Add a VisionProvider interface |
| frontends (qt/streamlit/...) | We only ship CLI + Next.js docs | Pick a framework, write the adapter |
| code_run_header.py | Too specific to GenericAgent's UX | Skip |
| compress_history_tags | Upstream optimization | Add in the loop, careful with Provider state |
| Plan mode | Advanced feature | Wrap the registry with a PlanHandler interceptor |

You can fill these in part-by-part — that's how the upstream got there.

## How to actually run s_full

We didn't ship s_full as a module (avoiding go.work module-count bloat).
For "real" integration, do:

```bash
# Start your own Go module
mkdir my-agent && cd my-agent
go mod init github.com/me/my-agent

# Vendor in s01..s10 or add them to go.work
# Or: copy core files in (most pedagogically clear)
```

Or: study upstream's [`agentmain.py:GenericAgent`](https://github.com/lsdefine/GenericAgent/blob/main/agentmain.py) class — it's the Python version of s_full.

Next: [appendix A — the essence of self-evolution](appendix-a-self-evolution.md).
