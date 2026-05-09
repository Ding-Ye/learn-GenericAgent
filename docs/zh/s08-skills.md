# s08 — 技能树与技能搜索

## Problem

s07 的 L3 是一组 `*_sop.md`——浏览器自动化 SOP、Vue 3 组件 SOP、记忆管理 SOP……
GenericAgent 上游有 ~20 个。如果每轮都把 20 个 SOP 全塞 system prompt：
- 浪费 token（大部分跟当前任务无关）
- 干扰信号（无关 SOP 让模型分心）
- 上下文窗口不够用

正确做法是**按需加载**：
1. 系统 prompt 只放**索引**（每个 SOP 一行摘要）
2. 模型决定"我需要 X 这个 SOP" → 调 `skill_search` 工具
3. 工具返回相关 SOP 全文
4. 模型基于 SOP 内容继续工作

这就是上游 `memory/skill_search/SKILL.md` 的设计。

## Solution

```
   /tmp/skills/                          ┌─────────────────┐
   ├── browser_sop.md      ──── scan ──► │ SkillTree       │
   ├── plan_sop.md                       │ skills []Skill  │
   ├── verify_sop.md                     └─────────────────┘
                                                   │
                                                   ▼
                              List() ── 索引 ── 系统 prompt
                              Search(query) ── tool: skill_search → 命中列表
                              Load(name)   ── tool: load_skill   → 单个 SOP 全文
```

`Skill` 结构记 4 个字段：

```go
type Skill struct {
    Name    string  // "browser"
    Path    string  // "/tmp/skills/browser_sop.md"
    Title   string  // "Browser SOP" (取文件第一行 H1，否则用 Name)
    Summary string  // H1 后第一段非空文本
}
```

## How It Works

`Scan()` 读目录、过滤 `_sop.md` 后缀、解析头部得到 Title/Summary。一次扫描的产物是一个排序的 `[]Skill`。

`Search()` 用最简单的 substring match——大写忽略、name+title+summary 拼起来一起匹配。
**真实场景**可换成 BM25 / 向量搜索；本节追求"够用就行"，因为这一抽象本身就允许将来替换。

`Load()` 在内存查到名字之后才读文件——避免 Scan 时把 20 个 SOP 全部读进内存。

## What Changed

新增模块。3 个 API：List / Search / Load。

| 文件 | 内容 |
|------|------|
| `skills.go` | `SkillTree` 类型 + `Scan/List/Search/Load` |
| `main.go` | CLI |
| `skills_test.go` | 6 个测试 |

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
# 看到索引：每个 SOP 一行

go run . -dir /tmp/skills -op search -q chrome
# 命中 browser

go run . -dir /tmp/skills -op load -q browser
# 输出 browser_sop.md 全文
```

实验：写一个 SOP 主题包含两个关键词（"chrome browser planning"），看 search 不同关键词都能召回。

## Upstream Source Reading

上游的 skill_search 设计在 `memory/skill_search/SKILL.md`：

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

注意上游**没有专门的 `skill_search` 工具**——它直接让模型用 `file_read memory/skill_search/SKILL.md`
读索引，看清楚有哪些技能后，再 `file_read memory/<topic>_sop.md` 加载具体那一个。

我们 s08 把这两步抽象成 `tree.Search()` / `tree.Load()` 两个方法，但**实质上跟上游做的是同一件事**：
"把按需加载的层放到工具调用 + 文件层面，而不是塞 system prompt"。

`memory/skill_search/skill_search/` 子目录里还有元元数据（怎么搜技能本身的 SOP）。
我们 s08 不复刻这层嵌套。

下一节 [s09-mixin](s09-mixin.md)：多 provider 故障切换（MixinSession 的 Go 实现）。
