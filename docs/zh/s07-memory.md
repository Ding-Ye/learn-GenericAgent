# s07 — 分层记忆与 working checkpoint

## Problem

Agent 跑久了会有"记忆问题"：
- 用户上次告诉过它"用 yarn 不要用 npm"——这一信息要怎么持久化？
- 它做了 100 轮决策——history 装不下，要怎么压缩？
- 它在第 5 轮记住的"key info"如何让第 50 轮还能看到？

把所有信息一股脑塞 system prompt 不行（200K context 也会打满）。
GenericAgent 的解法是**分层**——把信息按"变更频率 + 加载范围"切成 5 层。

## Solution

```
   L0 hardcoded                ◀── 永不变。本节定义 sysPromptBase 常量。
   L1 insight (routing index)  ◀── 偶尔人工编辑。存 memory/global_mem_insight.txt。
   L2 global_mem (stable kb)   ◀── agent 自己 append 增量事实。
   L3 SOP markdown             ◀── 一组"技能"recipe，按需加载。s08 详解。
   L4 archived sessions        ◀── 上一次跑完的完整 trace。可被 /resume。

   working checkpoint          ◀── 跨 turn 内、同一任务的 scratchpad
                                  （进程内变量，不落盘）
```

每轮发请求前，`AssembleSystemPrompt()` 拼装：

```
<L0>

[L1 Insight]
<L1 内容>

[L2 Global Memory]
<L2 内容>

[Working checkpoint, set 30s ago]
<checkpoint>
```

L3 不会全量塞进 system prompt——s08 会讲怎么按 query 选取相关 SOP。L4 也不进 prompt，
是给 `/resume` 命令用的归档区。

## How It Works

5 个 API + 1 个组装函数。核心是文件存储：

```go
type Memory struct {
    dir          string
    mu           sync.RWMutex
    checkpoint   string
    checkpointTS time.Time
}

func (m *Memory) UpdateGlobalMem(line string) error
func (m *Memory) L1Insight() (string, error)
func (m *Memory) L2Global() (string, error)
func (m *Memory) L3LoadSOP(topic string) (string, error)
func (m *Memory) L3ListSOPs() ([]string, error)
func (m *Memory) L4AppendSession(content string) error
func (m *Memory) SetCheckpoint(s string)
func (m *Memory) Checkpoint() (string, time.Time)
func (m *Memory) AssembleSystemPrompt() string
```

`SetCheckpoint` 加锁是因为：在 s_full 集成时 working checkpoint 会被 do_*
工具更新（多 goroutine 并发可能），同时 loop 在拼装 system prompt 时读它。

## What Changed

新增独立模块。本节没引入新 loop / 新工具，只引入 `Memory` 类型。

| 文件 | 内容 |
|------|------|
| `memory.go` | `Memory` 类型 + L0..L4 + 组装 |
| `main.go` | CLI: `go run . -op show / append / checkpoint` |
| `memory_test.go` | 7 个测试 |

## Try It

```bash
cd agents/s07-memory
go run . -dir /tmp/mem -op show
# 显示初始空 system prompt（L1 / L2 都是空文件）

go run . -dir /tmp/mem -op append -value "Rule: prefer pnpm over npm in this repo"
go run . -dir /tmp/mem -op append -value "Convention: tests live in *_test.go"
go run . -dir /tmp/mem -op show
# 看到 L2 Global Memory 段已经有内容了
```

实验：手工写一个 `/tmp/mem/browser_sop.md`，然后看 `L3ListSOPs()` 列出的内容
（s08 会用这个）。

## Upstream Source Reading

上游的 5 层在不同文件里：

| 层 | 上游位置 |
|----|---------|
| L0 | `assets/sys_prompt.txt` 文件，由 `agentmain.py:get_system_prompt` 读入 |
| L1 | `memory/global_mem_insight.txt` |
| L2 | `memory/global_mem.txt` |
| L3 | `memory/<topic>_sop.md`（约 20 个 SOP 文件，s08 详谈） |
| L4 | `temp/model_responses/<microsec>.txt` |

`agentmain.py:get_system_prompt` 拼装：

```python
def get_system_prompt():
    with open(f'assets/sys_prompt{lang_suffix}.txt') as f: prompt = f.read()
    prompt += f"\nToday: {time.strftime('%Y-%m-%d %a')}\n"
    prompt += get_global_memory()  # 读 global_mem.txt + global_mem_insight.txt
    return prompt
```

工作记忆注入在 `agent_loop.py:turn_end_callback`（`ga.py` 重写了 BaseHandler
的这一 hook）：

```python
# ga.py 中的 GenericAgentHandler.turn_end_callback (paraphrased)
def turn_end_callback(self, response, tool_calls, tool_results, turn, next_prompt, exit_reason):
    if 'key_info' in self.working:
        next_prompt = next_prompt + f"\n[Working memory]\n{self.working['key_info']}\n"
    return next_prompt
```

注意上游把 working checkpoint 注入到**下一轮的 user message**（next_prompt），
而不是 system prompt。我们 s07 的 `AssembleSystemPrompt` 把它放 system 里——
两种做法都行。Why upstream picks user message：因为 system 在 ToolClient 那里被缓存，
而 user message 每轮都新建，working memory 改了立刻生效。

`do_update_working_checkpoint` 是单行工具（`ga.py`）：

```python
def do_update_working_checkpoint(self, args, response):
    self.working['key_info'] = args['key_info']
    yield f'[Checkpoint] {args["key_info"][:60]}...\n'
    return StepOutcome(data='checkpoint updated')
```

下一节 [s08-skills](s08-skills.md)：技能树（L3）的 `skill_search` 怎么按 query 召回相关 SOP。
