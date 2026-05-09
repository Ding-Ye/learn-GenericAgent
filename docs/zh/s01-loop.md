# s01 — 最小 Agent 主循环

## Problem

GenericAgent 上游有 ~3,500 行代码、9 个工具、5 层记忆、4 种 provider。但你打开
`agent_loop.py` 会发现 **核心循环只有 50 行**。其它一切都是这 50 行的扩展。

如果我们一上来就读 `agentmain.py`（500 行）+ `ga.py`（900 行）+ `llmcore.py`（1500 行），
脑子会被流式 SSE、protocol-tool 解析、memory 注入这些细节淹没，看不到 agent 的本质。

**所以 s01 的任务：把"循环"剥离出来，单独跑通。** 没有工具，没有真模型，没有记忆。
只有一个 mock provider 加一个 `for` 循环。这是后面 9 节都要回到的脚手架。

## Solution

```
   ┌──────────────────────────────────────────┐
   │ messages = [system, user]                │
   └────────────┬─────────────────────────────┘
                ▼
        ┌────────────────┐    ◀───┐ turn += 1
        │ provider.Chat( │        │
        │   ctx, msgs,   │        │
        │   chunks)      │        │
        └────────┬───────┘        │
                 ▼                │
           response.Content       │
                 │                │
                 ▼                │
        append assistant msg ─────┘
                 │
                 ▼
            [TASK_DONE]
```

s01 永远只跑 1 轮。原因：mock provider 没有 tool_calls，模型一旦回复就视为任务完成。
后面 s02 加了 dispatch，循环才会真正多轮跑。

## How It Works

`Run` 函数签名：

```go
func Run(ctx context.Context,
    provider Provider,
    sysPrompt, userInput string,
    maxTurns int,
    chunks chan<- string,
) (ExitInfo, error)
```

三个职责：

1. **构造初始 messages**——`[{system, sysPrompt}, {user, userInput}]`。
   这一对 `(role, content)` 是几乎所有 LLM API 的通用格式，从这里开始就不要发明新结构。

2. **轮询 provider**——每轮发一次 `provider.Chat`。`chunks` 是一个 `chan<- string`，
   provider 边生成边往里塞文本片段，调用方可以实时打印（流式 UX）。

3. **判定退出**——s01 里"模型一回复就退出"。这是占位逻辑；真正的退出条件在
   s03 里由 `StepOutcome.next_prompt == ""` 决定。

`Provider` 是接口而不是结构体。这一点很关键：s01 用 `MockProvider`，s04 换成
`AnthropicProvider`，s09 换成 `MixinProvider`，**`Run` 一行不改**。

## What Changed

s01 是第一节，没有"上一节"可以 diff。但有几个**未来节会破坏的契约**值得记住：

| 契约 | 在哪一节被改写 |
|------|--------------|
| `Response` 只有 `Content` 一个字段 | s02：加 `ToolCalls []ToolCall` |
| 循环跑一轮就退出 | s02：循环根据 tool_calls 的存在与否决定 |
| `Provider.Chat` 不接收 tools 参数 | s02：签名加 `tools []ToolSpec` |
| `MockProvider` 一次性返回完整文本 | s02：mock 也要能模拟 tool_use 块 |

## Try It

```bash
cd agents/s01-loop
go run . -user "Hello?"
```

输出大概像：

```
--- Turn 1 ---
Hi! I'm a mock agent. You said: Hello?. (s01 has no tools, so I'm done.)

[exit] reason=TASK_DONE turns=1
```

跑测试：

```bash
go test -count=1 ./...
# ok  github.com/Ding-Ye/learn-GenericAgent/agents/s01-loop  0.3s
```

实验：

```bash
go run . -user "test" -max-turns 0
# [exit] reason=MAX_TURNS_EXCEEDED turns=0
```

`-max-turns 0` 让循环根本不进就退出，验证退出分支。

## Upstream Source Reading

把我们的 50 行 `Run` 跟 [`agent_loop.py:agent_runner_loop`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L33-L88)
对照看：

```python
# agent_loop.py:38-43  ── messages 初始化（和我们一样）
messages = [
    {"role": "system", "content": system_prompt},
    {"role": "user", "content": initial_user_content if initial_user_content is not None else user_input}
]
turn = 0; handler.max_turns = max_turns
```

```python
# agent_loop.py:44-52  ── 主循环骨架
while turn < handler.max_turns:
    turn += 1; turnstr = f'LLM Running (Turn {turn}) ...'
    ...
    response_gen = client.chat(messages=messages, tools=tools_schema)
    if verbose:
        response = yield from response_gen   # ← Python yield from = 我们的 chunks <-
        yield '\n\n'
```

注意 `yield from response_gen`：Python 的生成器组合器。把内层生成器的所有 yield 透传给外层。
Go 没有这个语法，所以我们用 `chan<- string` 显式传——provider 把 chunk 推进去，loop 不动地从外层透出。

```python
# agent_loop.py:54-58  ── tool_calls 判定（s01 砍掉了这段，s02 会加回来）
if not response.tool_calls:
    tool_calls = [{'tool_name': 'no_tool', 'args': {}}]
else:
    tool_calls = [...]
```

我们 s01 的 "立即 TASK_DONE" 对应的就是上面那个 `not response.tool_calls` 分支
——只是上游会再多走一步 `no_tool` 占位 tool。s03 里我们也会加这个 fallback。

**练习**：打开上面的链接，找出 `client.last_tools = ''` 那行（约第 47 行）。
猜猜为什么每 10 轮重置一次工具描述？答案在 s07。

---

下一节 [s02-tools](s02-tools.md)：给循环加工具。
