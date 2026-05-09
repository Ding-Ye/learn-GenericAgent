# learn-GenericAgent

> 一个 10 章渐进式 Go 实现，用来读懂 [lsdefine/GenericAgent](https://github.com/lsdefine/GenericAgent)（10K⭐ 自演化 agent 框架）的核心。
>
> A 10-chapter progressive Go re-implementation that lets you read your way through [lsdefine/GenericAgent](https://github.com/lsdefine/GenericAgent) (10K⭐ self-evolving agent framework).

[English version](README.en.md) · [上游 / Upstream](https://github.com/lsdefine/GenericAgent)

---

## 这是什么

GenericAgent 上游号称"3.3K 行核心 + 6× 更省 token"。但代码读起来仍然有 ~30 个文件、5 层抽象，
新手很难一眼看到本质。

本仓库把上游拆成 10 个 Go 学习模块。每节都是**独立可跑的小程序**：

- 自带 `go.mod` / `main.go` / 单元测试
- 不依赖前节的代码（每节复用同样的核心类型，但是文件级 copy + diff，不是 import）
- 配套**双语六段式**文档（Problem · Solution · How It Works · What Changed · Try It · Upstream Source Reading）
- 每节末尾的 **Upstream Source Reading** 都对应到上游真实文件、真实行号

> 教学法借鉴 [shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)：
> 心智模型优先 → ASCII 图 → 30–60 行核心代码 → 与上一节的 diff → 动手试 → 上游源码导读。

## 课程目录

| #  | 模块 | 教什么 | 上游对应 | 状态 |
|----|------|-------|----------|------|
| s01 | [s01-loop](agents/s01-loop) | 最小 Agent 主循环 | `agent_loop.py:agent_runner_loop` | ✅ |
| s02 | [s02-tools](agents/s02-tools) | 工具注册表与 dispatch | `agent_loop.py:BaseHandler` | ✅ |
| s03 | [s03-outcome](agents/s03-outcome) | StepOutcome 控制流 | `agent_loop.py:StepOutcome` | ✅ |
| s04 | [s04-claude](agents/s04-claude) | 真实 Anthropic Claude provider | `llmcore.py:NativeClaudeSession` | ✅ |
| s05 | [s05-coderun](agents/s05-coderun) | 流式代码执行工具 | `ga.py:code_run` | ✅ |
| s06 | [s06-fileops](agents/s06-fileops) | 文件读写补丁工具 | `ga.py:file_read/write/patch` | ✅ |
| s07 | [s07-memory](agents/s07-memory) | 分层记忆与 working checkpoint | `memory/`, `ga.py:do_update_working_checkpoint` | ✅ |
| s08 | [s08-skills](agents/s08-skills) | 技能树与技能搜索 | `memory/skill_search/SKILL.md` | ✅ |
| s09 | [s09-mixin](agents/s09-mixin) | 多 provider 故障切换 | `llmcore.py:MixinSession` | ✅ |
| s10 | s10-reflect | 反射模式与自动调度 | `agentmain.py --reflect`, `reflect/scheduler.py` | ⏳ |
| s_full | 集成 | 把上面 10 节拼起来跑一条端到端 use case | `agentmain.py:GenericAgent` | ⏳ |
| 附录 A | 自演化的本质 | "skill crystallization" 的心智模型 | README + memory/ | ⏳ |
| 附录 B | 上游源码导读地图 | 文件级 → 课程节的对照表 | 全仓库 | ⏳ |

✅ = 可读可跑；⏳ = 即将到来。

## Quickstart

```bash
git clone https://github.com/Ding-Ye/learn-GenericAgent.git
cd learn-GenericAgent
cd agents/s01-loop
go run . -user "Hello?"
```

跑测试：

```bash
go test -count=1 ./agents/...
```

启动文档站（可选）：

```bash
cd web
npm install
npm run dev
# open http://localhost:3000
```

## 为什么读这个

- **要理解"自演化 agent"**——上游核心思想是 *skill crystallization*：把求解过程沉淀成可复用的 SOP markdown。这一思想会在 s07 + s08 + 附录 A 出现。
- **要写自己的 agent harness**——每节 ~250 行 Go，照着抄就能拼出一个能跑的最小 agent harness。
- **要选型**——读完后你能判断你是该 fork GenericAgent，还是基于 LangGraph / claude-code-sdk 自建。
- **要练 Go**——把 Python generator → Go channel 的翻译练 10 遍，你的并发感会被强化。

## 适用人群

- 看过 LangGraph / autogen / claude-code 的 README 但没真的读过它们源码
- 想做 agent 但被 OOTB 框架的"约定优于配置"绑住手脚
- Python 出身，正在转 Go，想找个对照系练手项目

## 致谢

- 上游：[lsdefine/GenericAgent](https://github.com/lsdefine/GenericAgent) (MIT)
- 教学法：[shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)

## License

[MIT](LICENSE)
