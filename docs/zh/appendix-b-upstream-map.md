# 附录 B — 上游源码导读地图

读完本仓库 10 节后，你已经知道了 GenericAgent 的"骨架"。如果想读上游真源码，
这张地图告诉你：**每个上游文件 → 对应本课程哪一节 → 看哪几行**。

按文件大小由大到小排列。

## 大文件（核心）

### `llmcore.py` (58 KB, ~1500 LOC)

**最大也最复杂的文件**。LLM provider 抽象层。

| 内容 | 行号大致 | 对应本课程 |
|------|---------|----------|
| `_load_mykeys` / `reload_mykeys` | 5-30 | 不教 |
| `compress_history_tags` | 30-90 | 不教（高级优化） |
| `BaseSession` 基类 | 100-200 | s04 |
| `ClaudeSession` (HTTP-only) | 200-400 | s04 |
| **`NativeClaudeSession`** | 400-650 | s04 主对应 |
| `_parse_claude_sse` | 350-450 | s04 主对应 |
| `NativeOAISession` | 650-850 | s04 旁支 |
| `_parse_openai_sse` | 750-850 | s04 旁支 |
| `ToolClient` (protocol-mode tool calls) | 850-1050 | s04 高级 |
| `NativeToolClient` | 1050-1200 | s04 高级 |
| **`MixinSession`** | 1200-1350 | s09 主对应 |
| `_msgs_claude2oai` 协议适配 | 1350-1450 | s09 旁支 |
| `openai_tools_to_claude` | 1450-1500 | s09 旁支 |

**先读建议**：`NativeClaudeSession.raw_ask` + `_parse_claude_sse`（s04 已对照）；然后 `MixinSession`（s09 已对照）。其它都是"读到了再回来看"。

### `simphtml.py` (42 KB, ~1100 LOC)

**HTML → text 简化器**——给 `web_scan` 工具用。
**本课程不教**（域过专、教学回报低）。

如果你想做浏览器自动化工具：找一个轻量替代，比如 `golang.org/x/net/html` + 自己写一个 50 行的"取主体文字 / 列表 / 链接"的 visitor。**100% 不要从头实现 simphtml**。

### `ga.py` (34 KB, ~900 LOC)

**`GenericAgentHandler`** 类——所有 9 个 do_* 工具的实现。

| `def code_run` | 顶部 ~13-110 | s05 |
| `def file_read` | 中段 | s06 |
| `def file_patch` | 中段 | s06 |
| `def smart_format` | 中段 | s05/s06 共享 helper |
| `class GenericAgentHandler.__init__` | 中段 | s07 (memory 接入) |
| `def do_code_run` | 中段 | s05 |
| `def do_file_read/write/patch` | 中段 | s06 |
| `def do_web_scan` | 中段 | 我们 stub 掉了 |
| `def do_web_execute_js` | 中段 | 我们 stub 掉了 |
| `def do_ask_user` | 中段 | 我们没做（交互） |
| `def do_update_working_checkpoint` | 后段 | s07 |
| `def do_start_long_term_update` | 后段 | 我们没做（meta） |
| `def turn_end_callback` | 后段 | s07 |
| `def tool_before_callback` | 后段 | s07 (plan-mode 检查) |

**先读建议**：`do_code_run` → `do_file_read` → `do_file_patch` → `turn_end_callback`。

## 中等文件

### `agentmain.py` (15 KB, ~500 LOC)

**编排层**——`GenericAgent` 类。

| `class GenericAgent.__init__` | 顶部 | s_full |
| `load_llm_sessions` (mykey 解析) | 中段 | s09 配置层 |
| `next_llm` (provider 切换 slash 命令) | 中段 | s09 |
| `_handle_slash_cmd` | 中段 | 不教（UX） |
| `run` (主任务循环) | 中段 | s_full |
| `__main__` arg parser | 末尾 | s10 (--reflect 分支) |
| `--task` 一次性模式 | 末尾 | 不教 |
| **`--reflect` 模式** | 末尾 | s10 主对应 |
| 交互模式 (CLI input loop) | 末尾 | 不教 |

**先读建议**：`__main__` 的 `args.reflect` 分支（s10 已对照）；然后 `class GenericAgent.run`（s_full 心脏）。

### `TMWebDriver.py` (15 KB, ~400 LOC)

CDP-based Chrome 控制器。**本课程不教**。需要一个 Chrome 启动 `--remote-debugging-port`，然后 WebSocket 注入。

替代：用 [`chromedp`](https://github.com/chromedp/chromedp) Go 库。

## 小文件

### `agent_loop.py` (6.6 KB, ~100 LOC)

**核心 loop 文件**——是不是出乎意料的小？

整个文件内容覆盖了 s01 + s02 + s03。已经在每节的 Upstream Source Reading 段对照过。**强烈建议从头读到尾**——只有 100 行。

### `reflect/scheduler.py` (5 KB, ~120 LOC)

cron-style scheduler 示例，看 `check()` 函数怎么写。s10 详谈过。

### `reflect/goal_mode.py` (3.3 KB, ~80 LOC)

goal-driven autonomous mode 示例。s10 详谈过。

### `reflect/autonomous.py` (200 字节)

最小 reflect 脚本——5 行。读完你会发现 reflect 协议简单到几乎不像协议。

## memory/ 目录

| 文件 | 作用 | 对应章节 |
|------|------|---------|
| `global_mem.txt` | L2 | s07 |
| `global_mem_insight.txt` | L1 | s07 |
| `<topic>_sop.md` (~20 个) | L3 | s07 + s08 |
| `skill_search/SKILL.md` | L3 索引 | s08 |
| `L4_raw_sessions/` | L4 archive | s07 (L4) |

**值得人工读 2-3 个 SOP**：
- `memory_management_sop.md` — 元 SOP，告诉 agent 自己怎么演化
- `tmwebdriver_sop.md` — 浏览器自动化具体 recipe
- `verify_sop.md` — 验证清单

## 不教的东西汇总

| 上游路径 | 大小 | 不教原因 |
|---------|------|---------|
| `simphtml.py` | 42 KB | 域过专 |
| `TMWebDriver.py` | 15 KB | OS-coupled |
| `frontends/*.py` | ~ 200 KB | 跟课程无关 (UI 框架) |
| `assets/*.json` | 几十 KB | 工具 schema，自己生成 |
| `mykey_template.py` | 32 KB | 模板，看一眼就够 |
| `plugins/langfuse_tracing.py` | 5 KB | 可选 plugin |
| `memory/skill_search/skill_search/` | 嵌套 | meta-meta |
| `hub.pyw` / `launch.pyw` | 启动器 | UX 入口 |

加起来 ~300 KB，全是"实用但不教学"的东西。砍掉后剩 ~150 KB ≈ 3500 行核心，正好和 README 说的"3.3K-line seed"对得上。

## 推荐的"完整阅读路径"（一周）

| Day | 读什么 | 配合本课 |
|-----|-------|---------|
| 1 | `agent_loop.py` 全文 | s01-s03 已涵盖 |
| 2 | `agentmain.py` 全文 | s_full + s10 |
| 3 | `ga.py` 上半 (~code_run + file_*) | s05 + s06 |
| 4 | `ga.py` 下半 (callback + memory) | s07 |
| 5 | `llmcore.py` (NativeClaudeSession + parse_sse) | s04 |
| 6 | `llmcore.py` (MixinSession + adapters) | s09 |
| 7 | 3 个 SOP 文件 + `memory_management_sop.md` | s08 + 附录 A |

读完你能：
1. 一眼分辨出 GenericAgent vs LangGraph vs Autogen 的设计选择不同点
2. 把上游 fork 加自己的 Provider / 自己的工具 / 自己的 memory 形态
3. 给上游提 PR（已经是 lsdefine 自己人级别）

## 一个让你开始读的小练习

打开 `agent_loop.py` 第 60-87 行（`for ii, tc in enumerate(tool_calls):` 那个 block）。
**不看本课的对照**，自己讲一遍：

1. 如果 outcome 没有 next_prompt 也没有 should_exit，会发生什么？
2. 如果两个 tool_call 同时返回 next_prompt 但内容相同，会发生什么？
3. `client.last_tools = ''` 那一行的目的是什么？

答案在 s03 文档里。如果三道题都答对，你就读懂这个 loop 了。

完。
