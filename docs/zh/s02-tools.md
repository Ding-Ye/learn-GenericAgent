# s02 — 工具注册表与 dispatch

## Problem

s01 的循环跑一轮就结束。但真正的 agent 要能调用工具：读文件、执行代码、查网页。
模型怎么"调用"工具？答案在 OpenAI / Anthropic 的 API 里——response 中带一个 `tool_calls`
数组，每项含 `name` + `arguments`。

我们要：
1. 告诉 provider 有哪些工具可用
2. 拿到 `tool_calls` 后**调用对应的 Go 函数**
3. 把结果作为新一轮的输入回传给模型

上游 `agent_loop.py` 用 Python 反射做这件事——`do_<tool_name>` 的方法名约定。
Go 没有那么动态，但有更优雅的替代：**显式注册表**。

## Solution

```
   ┌─────────────────────┐
   │  Registry           │
   │  ───────────────    │
   │  map[name]ToolFunc  │ ◀──── reg.Register("echo", echoFn)
   │  map[name]ToolSpec  │
   └──────────┬──────────┘
              │
              ▼ Dispatch(name, args)
        ┌──────────┐
        │ ToolFunc │  func(ctx, args, chunks) (any, error)
        └──────────┘

   loop:
      resp := provider.Chat(msgs, reg.Specs(), chunks)
      if len(resp.ToolCalls) == 0: break
      for tc := range resp.ToolCalls:
          data, err := reg.Dispatch(tc.Name, tc.Args, chunks)
          msgs = append(msgs, Message{Role:"tool", ..., Content: marshal(data)})
```

`Registry` 是工具的所有真相：注册时同时落地它的 `ToolSpec`（schema）和 `ToolFunc`（实现），
loop 取 `Specs()` 给 provider，取 `Dispatch()` 执行。

## How It Works

`ToolFunc` 的签名是这一节最重要的一行：

```go
type ToolFunc func(ctx context.Context,
                  args map[string]any,
                  chunks chan<- string) (data any, err error)
```

为什么三个返回值（data + error + 通过 chunks 流式输出）而不是一个 `StepOutcome`？
因为 s02 还没引入 `StepOutcome`——s03 才把"流程控制"塞进 outcome 里。这一节我们只搞**调用**，
让形状尽量贴近"普通 Go 函数"。

`Dispatch` 实现就 4 行：

```go
fn, ok := r.funcs[name]
if !ok { return nil, fmt.Errorf("unknown tool: %q", name) }
return fn(ctx, args, chunks)
```

如果模型 hallucinate 一个不存在的工具名，我们**不报错给 loop**，而是把错误字符串作为
tool_result 喂回给模型，让它自我纠正。这是 agent harness 的常见 trick——错误也是输入。

## What Changed

s02 vs s01 的精确 diff：

| 文件 | 变更 |
|------|------|
| `types.go` | `Message` 加 `ToolCalls/ToolUseID/Name`；`Response` 加 `ToolCalls`；`Provider.Chat` 加 `tools []ToolSpec` 入参 |
| `loop.go` | for 循环不再单轮退出；调度 tool_calls，累积 tool_result 消息 |
| `mock.go` | `MockReply{Text, ToolCalls}` 取代纯字符串 |
| **`registry.go`** | 新增——所有 dispatch 的真相 |

## Try It

```bash
cd agents/s02-tools
go run .
```

输出大概像：

```
--- Turn 1 ---

🛠 echo(map[text:hello])
[echo] hello

--- Turn 2 ---

🛠 upper(map[text:loop])

--- Turn 3 ---
all done

[exit] reason=TASK_DONE turns=3
```

实验：把 `MockProvider` 的脚本改成调用 `nonexistent` 工具，看 loop 不会崩，而是把
"unknown tool" 错误回传给模型。

## Upstream Source Reading

上游用反射（`hasattr` + `getattr`）做 dispatch：

```python
# agent_loop.py:18-25 ── BaseHandler.dispatch
def dispatch(self, tool_name, args, response, index=0):
    method_name = f"do_{tool_name}"
    if hasattr(self, method_name):
        args['_index'] = index
        prer = yield from try_call_generator(self.tool_before_callback, tool_name, args, response)
        ret = yield from try_call_generator(getattr(self, method_name), args, response)
        _ = yield from try_call_generator(self.tool_after_callback, tool_name, args, response, ret)
        return ret
```

注意三个 callback：`tool_before_callback`、`tool_after_callback`、还有外层的
`turn_end_callback`。我们 s02 的 `Registry.Dispatch` 没有这些 hook。原因：上游的 hook 主要服务于
working checkpoint 注入和 plan-mode 检查——这些都是 s07 + s_full 才会出现的高级机制。
s02 的目标是先把"映射 + 调用"跑通。

工具注册的形态在上游是"加一个 method"：

```python
# ga.py（GenericAgentHandler 类内部）
class GenericAgentHandler(BaseHandler):
    def do_code_run(self, args, response): ...
    def do_file_read(self, args, response): ...
    def do_file_write(self, args, response): ...
    def do_web_scan(self, args, response): ...
    # 9 个 do_* method 就是 9 个工具
```

我们的 Go 翻译是显式 `reg.Register(spec, fn)`。两种风格各有优劣：
- Python 反射：写起来短，但工具列表分散，IDE 跳转难
- Go 注册表：多一行 `Register`，但工具列表集中可见，spec 与 fn 强绑定

下一节 [s03-outcome](s03-outcome.md)：把 `StepOutcome` 加进来，让工具能精细控制循环走向。
