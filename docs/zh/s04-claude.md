# s04 — 真实 Anthropic Claude provider

## Problem

s01–s03 用 `MockProvider` 把循环、工具、控制流都跑通了。但要做真 agent，必须接真模型。

接模型的 SDK 选择有两种：
- **官方 SDK**（`github.com/anthropics/anthropic-sdk-go` 之类）——开箱即用，但隐藏了协议细节
- **直接 HTTP**——多写 200 行，但你**完全控制**请求/SSE 解析/超时/重试

上游 GenericAgent 选了第二种（`llmcore.py:NativeClaudeSession.raw_ask` 用 `requests.post`
直连 endpoint）。原因：
1. 不用绑定 SDK 版本（API 改动你能立刻适配）
2. SSE 解析能加 langfuse / 自定义日志 / 流量重写
3. 多 provider mix（s09 会展开）需要统一的 raw HTTP 层

s04 用 Go 复刻这一层。

## Solution

```
┌────────────────────────────────────────────────────┐
│ AnthropicProvider                                  │
│  APIKey  string                                    │
│  Model   string  ("claude-haiku-4-5-20251001")     │
│  Endpoint string ("https://api.anthropic.com/...") │
│                                                    │
│  Chat(ctx, msgs, tools, chunks) (Response, error)  │
│   │                                                │
│   ├─ toAnthropic(msgs) → system + content blocks   │
│   ├─ POST /v1/messages stream=true                 │
│   └─ parseSSE(body, chunks) → assemble Response    │
└────────────────────────────────────────────────────┘
```

`toAnthropic` 是协议适配的精华——把我们的扁平 `Message{Role,Content,ToolCalls}`
转成 Anthropic 要求的 content-block 形态：

| 我们 | Anthropic |
|------|-----------|
| `Role:"system"` | 顶层 `system` 字段（多个 system 用 `\n` 拼接）|
| `Role:"user", Content:"hi"` | `{role:"user", content:[{type:"text", text:"hi"}]}` |
| `Role:"assistant", ToolCalls:[...]` | `{role:"assistant", content:[{type:"text",...}, {type:"tool_use",...}]}` |
| `Role:"tool"` | `{role:"user", content:[{type:"tool_result", tool_use_id:"...", content:"..."}]}` |

注意最后一行：**tool_result 必须装在 user 角色里**——这是 Anthropic 的协议要求。
扁平 `Role:"tool"` 是我们方便记忆的内部抽象，发请求前会被翻译。

## How It Works

`parseSSE` 是这一节最值得读的代码。Anthropic 的 SSE 协议事件名：

```
event: message_start         ← 序章，不带数据
event: content_block_start   ← 一个块（text 或 tool_use）开始
event: content_block_delta   ← 增量（text_delta 或 input_json_delta）
event: content_block_stop    ← 当前块结束
event: message_delta         ← 含 stop_reason / usage
event: message_stop          ← 流结束
event: ping                  ← 心跳，忽略
event: error                 ← 服务端错误
```

代码骨架（精简）：

```go
for scanner.Scan() {
    if !strings.HasPrefix(line, "data:") { continue }
    json.Unmarshal(payload, &ev)
    switch ev["type"] {
    case "content_block_start":
        // 记录 blockType, blockToolID, blockToolNm, 重置 builders
    case "content_block_delta":
        if delta.type == "text_delta":
            // 推到 chunks，累积到 blockText
        if delta.type == "input_json_delta":
            // 累积 partial_json 到 blockJSON（不推 chunks——是 tool 参数）
    case "content_block_stop":
        // 块结束：text 块累积到 out.Content；tool_use 块 unmarshal blockJSON 后入 out.ToolCalls
    case "message_stop":
        return out, nil
    case "error":
        return out, fmt.Errorf(...)
    }
}
```

两个关键点：

1. **`text_delta` 推 chunks，`input_json_delta` 不推**——前者是给用户看的可见文本，后者是
   工具参数 JSON 片段（`"text"` 在第一片，`":"` 在第二片，`"hi"`在第三片……），推给用户没意义。

2. **`partial_json` 是片段拼接**——不能每段都尝试 `json.Unmarshal`（中间状态非法 JSON）。
   攒到 `content_block_stop` 才整体 unmarshal。

## What Changed

s04 vs s03 的精确 diff：

| 文件 | 变更 |
|------|------|
| `types.go` | 不变（沿用 s03 的 `Message/StepOutcome/Provider`）|
| `loop.go` | 不变（s03 的 loop 一行不改！）|
| `registry.go` | 不变 |
| `mock.go` | **删除**——不再需要 |
| `anthropic.go` | **新增**——`AnthropicProvider`, `toAnthropic`, `parseSSE` |
| `main.go` | 改用 `NewAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY"), model)` |
| `anthropic_test.go` | 用 `httptest.NewServer` + 假 SSE 流测试，不需要真 API key |

**这是 s04 的最大教育意义**：因为 s01–s03 把抽象做对了，从 mock 切到 real LLM
**只改一个文件**。

## Try It

不需要 API key 的离线测试：

```bash
cd agents/s04-claude
go test -count=1 ./...
```

带 API key 的真实运行：

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run . -user "What's 2+2?"
```

可选用 `-model` 切换模型：

```bash
go run . -model claude-sonnet-4-6 -user "Write a haiku about goroutines"
```

## Upstream Source Reading

上游 `llmcore.py` 第 1-80 行是 mykey 加载 + history 压缩。真正的 SSE 解析在
`_parse_claude_sse`（向下搜该函数名）。结构对照：

```python
# llmcore.py:_parse_claude_sse （概略）
def _parse_claude_sse(resp_iter):
    for line in resp_iter:
        if not line.startswith(b'data:'): continue
        payload = json.loads(line[5:].strip())
        if payload['type'] == 'content_block_start':
            blk = payload['content_block']
            if blk['type'] == 'tool_use':
                cur_tool = {'id': blk['id'], 'name': blk['name'], 'input': ''}
        elif payload['type'] == 'content_block_delta':
            if payload['delta'].get('type') == 'text_delta':
                yield payload['delta']['text']    ← 推给 generator 消费者
                content_buf += payload['delta']['text']
            elif payload['delta'].get('type') == 'input_json_delta':
                cur_tool['input'] += payload['delta']['partial_json']
        elif payload['type'] == 'content_block_stop':
            if cur_tool: tool_calls.append(...); cur_tool = None
        elif payload['type'] == 'message_stop': break
```

跟我们 Go 版本几乎对照——Python `yield` ≈ Go `chunks <- text`。

上游的 [`_stream_with_retry`](https://github.com/lsdefine/GenericAgent/blob/main/llmcore.py)
还有指数退避重试和 `compress_history_tags` 调用——这些是 production 的优化，
我们 s04 暂不引入。s09 会处理 retry / failover 逻辑（但放在 MixinSession 层）。

下一节 [s05-coderun](s05-coderun.md)：第一个真工具——能在子进程里跑 Python/bash 并流式回传 stdout。
