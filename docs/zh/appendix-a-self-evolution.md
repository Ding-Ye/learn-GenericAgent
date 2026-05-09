# 附录 A — 自演化的本质

## "Self-Evolving Agent" 这个词到底是什么意思？

GenericAgent 的 README 第一句是：

> Self-evolving agent: grows skill tree from 3.3K-line seed, achieving full
> system control with 6x less token consumption.

"self-evolving" 听起来很玄。但代码读完会发现：**它是一种特定的、可观测的工程行为**。
本附录把它拆解成 3 步。

## Step 1：什么不是"自演化"

**不是**：
- 在线训练（fine-tuning during deploy）
- 模型权重自我修改
- 神经架构演化
- 任何机器学习意义上的"学习"

**是**：
- 一种 **prompt + 文件系统** 上的演化——**不动模型权重**

## Step 2："演化"的载体是什么

GenericAgent 的演化载体是 `memory/` 目录里的两类文件：

1. **L2 (`global_mem.txt`)**——增量事实积累
   - "用户偏好 pnpm 不是 npm"
   - "本机的 Python 是 /opt/homebrew/bin/python3 不是 /usr/bin/python3"
   - "公司内部的 jira webhook 在 hooks.example.com/jira"

2. **L3 (`<topic>_sop.md`)**——程序性知识结晶
   - "怎么用 chrome CDP 注入 JS"——10 步流程
   - "怎么写一个能通过我们 lint 的 Vue 3 组件"——20 行模板
   - "怎么验证一次 git commit 是否引入回归"——5 步检查表

L2 是事实，L3 是流程。两者**都是 markdown 文本文件**，不是数据库不是向量。
agent 修改它们的方式就是 `do_file_write` / `do_file_patch`——你已经在 s06 学过的两个工具。

## Step 3：演化是怎么"自动"发生的

最关键的设计：上游有一个特殊的 SOP 叫 `memory_management_sop.md`，告诉 agent：

> "每次解决了一个非平凡问题，回顾本次会话的关键决策；
> 如果某个决策可以泛化到未来类似任务，把它写进 L2；
> 如果某个解题路径可以打包成 SOP，写进 `<topic>_sop.md`。"

加上一个特殊工具 `do_start_long_term_update`——用户调用 `/update_memory` 后触发这个工具，
agent 启动一个**子会话**专门做 memory consolidation。

伪代码逻辑：

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

**结果**：用一周后，你的 GenericAgent 实例的 `memory/` 看起来跟我的 GenericAgent 实例
完全不同。它"长成"你的特化版本。

## 为什么这能省 6 倍 token

对比传统 agent harness（langchain / autogen / crewai）：

| 维度 | 传统 harness | GenericAgent |
|------|-------------|--------------|
| 工具规模 | 100+ 预定义工具 | 9 个原子工具 |
| Prompt 中工具描述 | 完整 100 个的 schema | 完整 9 个的 schema |
| Skill / capability | 写死在 framework 内 | L3 markdown 按需加载 |
| 个性化 | 在 system prompt 里堆 | 在 L1/L2 文件里增量积累 |
| 一次任务平均 token | 30K-200K | 5K-30K |

**省的不是模型自己变快**——是**不必要的上下文被砍掉了**。9 个原子工具配 `code_run` 几乎能干所有事
（要爬网页就 `code_run("python -c 'import requests; ...'")`），不需要预定义 100 个 wrapped 工具。
任务不需要的 SOP 也不会塞进来——s08 的按需召回。

## "自演化"的代价

不是没有：
- **冷启动慢**——新装的 GenericAgent 知识量为零，前几个任务都慢
- **依赖用户参与**——L2/L3 的 evolution 需要用户偶尔点头确认
- **跨用户不通用**——你的 SOP 不一定适合别人的 setup
- **memory 漂移**——长跑后会积累过时甚至矛盾的事实，需要定期 garbage collect

上游用一个 `memory_cleanup_sop.md` 处理最后一个问题——agent 自己定期清理 memory。
这是个有趣的递归：用 agent 维护 agent 自己的记忆。

## 我们的 learn 版做了什么、没做什么

| 自演化机制 | 我们 (s07/s08) | 上游 |
|-----------|---------------|------|
| L0-L4 分层结构 | ✅ Memory + SkillTree | ✅ |
| L2 增量 append | ✅ UpdateGlobalMem | ✅ |
| L3 SOP 文件 | ✅ SkillTree.Load | ✅ |
| L3 按需召回 | ✅ Search | ✅ |
| do_update_working_checkpoint | ✅ SetCheckpoint | ✅ |
| memory_management_sop 元 SOP | ❌ | ✅ |
| do_start_long_term_update 元工具 | ❌ | ✅ |
| /resume 命令读 L4 | ❌ | ✅ |

我们做了**机制**没做**meta**——meta 部分（agent 自己改 memory 的能力）需要先跑起来一个真 agent，
跟用户对话循环过几轮才有意义。学习版没法在静态测试里证明这一层有效，所以省略了。

但你只要把 s_full 跑起来 + 加几个 markdown 文件 + 教它写 L2，**自演化就开始了**。
这就是上游的 trick：**演化不是新机制，是把现有机制（写文件 / 读文件 / 工具调用）组合用**。

## 进一步阅读

- 上游的 `memory/memory_management_sop.md` —— 元 SOP 全文
- 上游的 `memory/autonomous_operation_sop.md` —— 何时该自演化
- ["The Bitter Lesson"](http://www.incompleteideas.net/IncIdeas/BitterLesson.html) by Sutton — 为什么 prompt + 文件 比预定义工具更"苦涩但有效"
