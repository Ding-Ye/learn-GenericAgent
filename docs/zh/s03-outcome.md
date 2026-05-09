# s03 — StepOutcome 控制流

## Problem

s02 的 `ToolFunc` 返回 `(data any, err error)`。但这表达不了：
- **"我做完了，loop 你停下"**——比如 `finish_task` 工具
- **"做完了一步，loop 你接着跑，下一轮 user 消息用这个 hint"**——比如 `code_run` 失败后想让模型重试
- **"我处理出了语义错误（不是 panic），把这段错误回灌给模型让它纠正"**——比如 `file_patch` 找不到 old block

如果都用 `error` 表达，调用方无法区分"程序错误"和"语义信号"。
上游 GenericAgent 的解决方案是 `StepOutcome`：**3 个字段编码 3 种语义**。

## Solution

```go
type StepOutcome struct {
    Data       any    // 给模型看的 tool_result
    NextPrompt string // 非空 → 当作下一轮 user 消息；空 → 任务完成
    ShouldExit bool   // true → 立即终止 loop
}
```

三个字段对应 3 个 loop 出口：

```
   tool returns StepOutcome
            │
            ├── ShouldExit=true ─────────► EXITED
            │
            ├── NextPrompt == "" ────────► TASK_DONE (current micro-task done)
            │
            └── NextPrompt != "" ────────► loop continues, user msg = NextPrompt
```

## How It Works

`Registry.Dispatch` 现在返回 `StepOutcome`。未知工具不再返回 `error`，而是返回
`StepOutcome{NextPrompt: "unknown tool: <name>"}`——这样模型在下一轮就会看到这个错误信息。

`Run` 在 dispatch 完每个 tool_call 后做 3 件事：

```go
// 1. 收集所有 outcomes
for _, tc := range resp.ToolCalls {
    oc := registry.Dispatch(...)
    outcomes = append(outcomes, oc)
}

// 2. 任何一个 ShouldExit=true 立刻退出
for _, oc := range outcomes {
    if oc.ShouldExit { return ExitInfo{Reason:"EXITED", Data: oc.Data}, nil }
}

// 3. 收集所有非空 NextPrompt（dedup），当作下一轮 user 消息
nextPrompt := joinUnique(outcomes, "\n")
if nextPrompt == "" { return ExitInfo{Reason: "TASK_DONE"}, nil }
msgs = append(msgs, Message{Role:"user", Content: nextPrompt})
```

第 3 步是**多工具调用合并语义**的关键：如果模型一轮调了 3 个工具，每个都返回了不同的
NextPrompt，loop 会把它们用 `\n` 合并成一条 user 消息。如果 3 个都返回相同的 NextPrompt，
dedup 后只保留一份——避免无意义的重复 hint。

## What Changed

s03 vs s02 的精确 diff：

| 文件 | 变更 |
|------|------|
| `types.go` | 新增 `StepOutcome` 类型；`ToolFunc` 签名从 `(any, error)` 变成 `StepOutcome` |
| `registry.go` | `Dispatch` 返回 `StepOutcome`；未知工具变成 `NextPrompt` 错误，不返回 `error` |
| `loop.go` | 主循环加了 outcome 聚合（exit / done / next_prompt 三分支） |

## Try It

```bash
cd agents/s03-outcome
go run .
```

输出：

```
--- Turn 1 ---

🛠 think_again(map[hint:consider edge case])
--- Turn 2 ---

🛠 finish_task(map[summary:all checks passed])

[finish] all checks passed

[exit] reason=EXITED turns=2 data=all checks passed
```

实验：把 `finish_task` 工具的 `ShouldExit: true` 改成 `false`，看 loop 怎么走。
你会发现它进入"NextPrompt 为空 → TASK_DONE"分支。

## Upstream Source Reading

上游的 `StepOutcome` 在 `agent_loop.py` 第 5-8 行——4 行 dataclass：

```python
# agent_loop.py:5-8
@dataclass
class StepOutcome:
    data: Any
    next_prompt: Optional[str] = None
    should_exit: bool = False
```

并在 loop 里被这样消费（`agent_loop.py:60-79`）：

```python
for ii, tc in enumerate(tool_calls):
    ...
    gen = handler.dispatch(tool_name, args, response, index=ii)
    try:
        v = next(gen)
        def proxy(): yield v; return (yield from gen)
        outcome = (yield from proxy()) if verbose else exhaust(proxy())
    except StopIteration as e: outcome = e.value

    if outcome.should_exit:
        exit_reason = {'result': 'EXITED', 'data': outcome.data}; break
    if not outcome.next_prompt:
        exit_reason = {'result': 'CURRENT_TASK_DONE', 'data': outcome.data}; break
    if outcome.next_prompt.startswith('未知工具'): client.last_tools = ''
    if outcome.data is not None and tool_name != 'no_tool':
        datastr = ...
        tool_results.append({'tool_use_id': tid, 'content': datastr})
    next_prompts.add(outcome.next_prompt)
```

注意几个细节：

1. **`StopIteration.value` 取 outcome**——Python 生成器结束时把返回值塞在异常里。
   Go 用 channel 拉流式 chunk + 直接返回 outcome，比 Python 干净。

2. **`if outcome.next_prompt.startswith('未知工具'): client.last_tools = ''`**
   ——遇到未知工具时清空 client 的工具描述缓存，强制下一轮重发完整 tool spec。
   这是上游的小细节优化；我们 s03 没做，留到 s07 (memory & cache) 时聊。

3. **`next_prompts.add(...)`**——上游用 Python `set` 自动 dedup。我们用 Go 的
   `map[string]struct{}` 做同样的事。

下一节 [s04-claude](s04-claude.md)：终于换上真模型——Anthropic Claude 原生 HTTP 调用。
