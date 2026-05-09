# s_full — 集成端到端 use case

## 把 s01-s10 接起来跑一条真实路径

s01-s10 每节都是独立 Go module，相互不 import。这种"重复的好处"是教学上每节自包含，
**坏处是少看了一次"它们怎么一起工作"**。这一节用 ASCII 图 + 一条 16 步执行轨迹补上。

## 全栈架构

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

每一节贡献的"零件"在图上标注了。注意：

- **Provider** 是接口；s04 写一个具体实现，s09 写一个 Mixin 组合实现，s_full 把它们串起来。
- **Registry** 是 s02 的产物；s05/s06/s07/s08 各贡献几个 ToolFunc 注册进去。
- **Memory** 是 s07 的产物；s08 的 SkillTree 装进 Memory 的 L3 接口里。

## 16 步执行轨迹（一个真实 use case）

**用户场景**：reflect 模式下，agent 监控一个 GitHub repo 的 CI 状态。CI 红了，agent 自动
fetch 错误日志、定位问题、改 fix、跑 tests 验证、commit + push。

```
Step  Layer        Detail                                                上游对应
────  ──────────   ──────────────────────────────────────────────────────────────
 1   reflect      JSONCheck("/etc/agent/ci.json") 读到 status=red          s10
 2   reflect      ReflectLoop 派 task = "CI is red on commit abc; fix"    s10
 3   loop         Run(ctx, task) 进入主循环 turn 1                        s01
 4   memory       AssembleSystemPrompt() 组装 L0+L1+L2 + 当前 checkpoint  s07
 5   provider     MixinProvider.Chat() 优先试 AnthropicProvider           s09→s04
 6   provider     SSE: 模型返回 tool_call: skill_search(query="CI debug") s04
 7   loop         resp.ToolCalls 非空，dispatch 第一个                    s02
 8   tool         skill_search 在 SkillTree 里找 ci_sop.md，返回索引行   s08
 9   loop         outcome.NextPrompt = "use load_skill to read..."        s03
10   loop         turn 2 开始；msgs 累积了 tool_result + next_prompt     s01
11   provider     Claude 返回 tool_call: load_skill("ci_sop")             s04
12   tool         读 memory/ci_sop.md 全文返回                           s08
13   loop         turn 3：模型理解 SOP 后调 code_run("git log ...")      s05
14   tool         code_run 流式打印 git log 结果到 chunks                s05
15   loop         turn N：file_patch 修复一行 → code_run 跑 tests        s06+s05
16   loop         模型不再调 tool，最终 text reply → ExitInfo{TASK_DONE} s03
```

注意第 8 步：**SOP 不是预先塞 system prompt 的**，而是在第 6 步模型自己判断"我需要 debug 技能"
后通过 tool 取来的。这就是 s08 的 skill 树设计——按需加载。

## 你看到了什么"小东西的力量"？

10 个 module 加起来 ~2700 行 Go。每一节都很小：
- s01 只有 50 行 loop
- s02 只有 60 行 registry
- s03 只有 4 行 dataclass + 30 行 outcome 聚合

但它们组合起来，s_full 是一个**完整能跑的 agent harness**——能调真模型、能调真工具、
能记忆、能自动触发。这就是 GenericAgent 上游"3K 行能干事"的本质。

## 我们 deliberately 没有做的事

| 上游 feature | 为什么没做 | 你想加该怎么入手 |
|-------------|-----------|----------------|
| simphtml.py | 1000 行 HTML 处理器，超出教学范围 | golang.org/x/net/html 实现一个 mini 版 |
| TMWebDriver | OS-coupled，依赖真 Chrome | 用 chromedp 库重写一个 do_browser |
| 多模态 vision_api | 域特定 | 加一个 VisionProvider 接口 |
| frontends (qt/streamlit/...) | 我们只做 CLI + Next.js docs | 你自己挑一个框架接 |
| code_run_header.py | 太特定于 GenericAgent UX | 不用做 |
| compress_history_tags | 上游优化 | 在 loop 里加，但要小心改 Provider 状态 |
| Plan mode | 高级特性 | 加一个 PlanHandler 拦截 do_* 调用 |

你能照着图把这些零件一个一个补回去——这就是上游的代码。

## 怎么真正跑 s_full

s_full 没有写成一个 module（避免 go.work 的 mod 数量爆炸）。要"实战"集成请：

```bash
# 自己开一个新 Go module
mkdir my-agent && cd my-agent
go mod init github.com/me/my-agent

# 把 s01..s10 当作单独 module 加进 vendor 或者 go.work
# 或者：把每节的核心文件复制进来（最直观）
```

或者参考 [`appendix-b-upstream-map.md`](appendix-b-upstream-map.md) 看 GenericAgent 上游
怎么把这些组件粘起来——它的 `agentmain.py:GenericAgent` 类就是 s_full 的 Python 版。

下一节：[appendix A — 自演化的本质](appendix-a-self-evolution.md)
