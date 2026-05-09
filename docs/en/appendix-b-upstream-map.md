# Appendix B — Upstream Source Reading Map

After all 10 chapters, you've internalized GenericAgent's skeleton. To
actually read the upstream source, this map tells you: **which file → which
chapter of this course → which line range**.

Sorted by file size, large first.

## Large files (core)

### `llmcore.py` (58 KB, ~1500 LOC)

**The largest and most complex file.** LLM provider abstraction layer.

| Content | Approx. lines | Course chapter |
|---------|---------------|----------------|
| `_load_mykeys` / `reload_mykeys` | 5-30 | not taught |
| `compress_history_tags` | 30-90 | not taught (advanced opt) |
| `BaseSession` base | 100-200 | s04 |
| `ClaudeSession` (HTTP-only) | 200-400 | s04 |
| **`NativeClaudeSession`** | 400-650 | s04 main |
| `_parse_claude_sse` | 350-450 | s04 main |
| `NativeOAISession` | 650-850 | s04 sidebar |
| `_parse_openai_sse` | 750-850 | s04 sidebar |
| `ToolClient` (protocol-mode tool calls) | 850-1050 | s04 advanced |
| `NativeToolClient` | 1050-1200 | s04 advanced |
| **`MixinSession`** | 1200-1350 | s09 main |
| `_msgs_claude2oai` | 1350-1450 | s09 sidebar |
| `openai_tools_to_claude` | 1450-1500 | s09 sidebar |

**Read order**: `NativeClaudeSession.raw_ask` + `_parse_claude_sse` (already
paired in s04); then `MixinSession` (s09). Everything else: "come back to
when you trip over it."

### `simphtml.py` (42 KB, ~1100 LOC)

**HTML → text simplifier** — used by the `web_scan` tool.
**Not taught** (too domain-specific, low pedagogical ROI).

If you're building browser automation: pick a lightweight replacement —
e.g. `golang.org/x/net/html` + a 50-line visitor for "extract main text /
lists / links." **Don't reimplement simphtml from scratch**.

### `ga.py` (34 KB, ~900 LOC)

**`GenericAgentHandler`** class — implementations of all 9 do_* tools.

| `def code_run` | top ~13-110 | s05 |
| `def file_read` | mid | s06 |
| `def file_patch` | mid | s06 |
| `def smart_format` | mid | s05/s06 shared helper |
| `class GenericAgentHandler.__init__` | mid | s07 (memory wiring) |
| `def do_code_run` | mid | s05 |
| `def do_file_read/write/patch` | mid | s06 |
| `def do_web_scan` | mid | stubbed for us |
| `def do_web_execute_js` | mid | stubbed for us |
| `def do_ask_user` | mid | not done (interactive) |
| `def do_update_working_checkpoint` | bottom | s07 |
| `def do_start_long_term_update` | bottom | not done (meta) |
| `def turn_end_callback` | bottom | s07 |
| `def tool_before_callback` | bottom | s07 (plan-mode) |

**Read order**: `do_code_run` → `do_file_read` → `do_file_patch` →
`turn_end_callback`.

## Medium files

### `agentmain.py` (15 KB, ~500 LOC)

**Orchestration** — `GenericAgent` class.

| `class GenericAgent.__init__` | top | s_full |
| `load_llm_sessions` (mykey parsing) | mid | s09 config layer |
| `next_llm` (provider switch slash cmd) | mid | s09 |
| `_handle_slash_cmd` | mid | not taught (UX) |
| `run` (main task loop) | mid | s_full |
| `__main__` arg parser | bottom | s10 (--reflect branch) |
| `--task` one-shot mode | bottom | not taught |
| **`--reflect` mode** | bottom | s10 main |
| Interactive mode (CLI input loop) | bottom | not taught |

**Read order**: `__main__`'s `args.reflect` branch (paired in s10); then
`class GenericAgent.run` (heart of s_full).

### `TMWebDriver.py` (15 KB, ~400 LOC)

CDP-based Chrome controller. **Not taught.** Requires Chrome with
`--remote-debugging-port`, then WebSocket injection.

Replacement: [`chromedp`](https://github.com/chromedp/chromedp) Go library.

## Small files

### `agent_loop.py` (6.6 KB, ~100 LOC)

**The core loop file.** Smaller than you'd expect, right?

The whole file covers s01 + s02 + s03. Already paired chapter by chapter.
**Read it end-to-end** — only 100 lines.

### `reflect/scheduler.py` (5 KB, ~120 LOC)

Cron-style scheduler example showing how to write a `check()` function.
Detailed in s10.

### `reflect/goal_mode.py` (3.3 KB, ~80 LOC)

Goal-driven autonomous mode example. Detailed in s10.

### `reflect/autonomous.py` (200 bytes)

Minimum reflect script — 5 lines. After reading you'll see the reflect
protocol is barely a protocol.

## memory/ directory

| File | Role | Course |
|------|------|--------|
| `global_mem.txt` | L2 | s07 |
| `global_mem_insight.txt` | L1 | s07 |
| `<topic>_sop.md` (~20) | L3 | s07 + s08 |
| `skill_search/SKILL.md` | L3 index | s08 |
| `L4_raw_sessions/` | L4 archive | s07 |

**Worth manually reading 2-3 SOPs**:
- `memory_management_sop.md` — meta-SOP, tells the agent how to evolve itself
- `tmwebdriver_sop.md` — concrete browser automation recipe
- `verify_sop.md` — verification checklist

## Things we don't teach (summary)

| Upstream path | Size | Why omitted |
|---------------|------|-------------|
| `simphtml.py` | 42 KB | Too domain-specific |
| `TMWebDriver.py` | 15 KB | OS-coupled |
| `frontends/*.py` | ~200 KB | Out of course scope (UI frameworks) |
| `assets/*.json` | tens of KB | Tool schema, generate yourself |
| `mykey_template.py` | 32 KB | Template, eyeball briefly |
| `plugins/langfuse_tracing.py` | 5 KB | Optional plugin |
| `memory/skill_search/skill_search/` | nested | meta-meta |
| `hub.pyw` / `launch.pyw` | launchers | UX entry points |

Adds up to ~300 KB of "useful but not pedagogical" content. After cutting,
~150 KB ≈ 3500 LOC core, matching the README's "3.3K-line seed."

## Recommended one-week reading path

| Day | Read | With this course |
|-----|------|------------------|
| 1 | `agent_loop.py` end-to-end | s01-s03 covers it |
| 2 | `agentmain.py` end-to-end | s_full + s10 |
| 3 | `ga.py` first half (~code_run + file_*) | s05 + s06 |
| 4 | `ga.py` second half (callback + memory) | s07 |
| 5 | `llmcore.py` (NativeClaudeSession + parse_sse) | s04 |
| 6 | `llmcore.py` (MixinSession + adapters) | s09 |
| 7 | 3 SOP files + `memory_management_sop.md` | s08 + Appendix A |

After: you can
1. Distinguish at a glance GenericAgent's design choices vs LangGraph vs Autogen
2. Fork upstream and add your own Provider / your own tools / your own memory shape
3. Submit PRs to upstream (you're now at lsdefine-insider level)

## A small exercise to start reading

Open `agent_loop.py` lines 60-87 (the `for ii, tc in enumerate(tool_calls):`
block). **Without consulting our chapter**, narrate it yourself:

1. If outcome has neither `next_prompt` nor `should_exit`, what happens?
2. If two tool_calls both return identical `next_prompt`, what happens?
3. What's the purpose of the `client.last_tools = ''` line?

Answers are in s03's doc. If you got all three right, you understand the
loop.

End.
