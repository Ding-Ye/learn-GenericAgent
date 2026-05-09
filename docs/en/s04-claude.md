# s04 — Real Anthropic Claude Provider

## Problem

s01–s03 used `MockProvider` to make the loop, tools, and control flow all
work. To build a real agent, we must wire in a real model.

Two integration paths:
- **Official SDK** (e.g. `github.com/anthropics/anthropic-sdk-go`) — works
  out of the box, but hides protocol details
- **Direct HTTP** — 200 extra lines, but you fully control request, SSE
  parsing, timeouts, retries

Upstream GenericAgent took door #2 (`llmcore.py:NativeClaudeSession.raw_ask`
posts directly to the endpoint with `requests`). Reasons:
1. No SDK version pin (you adapt to API changes immediately)
2. SSE parsing can be hooked for langfuse / custom logging / traffic rewriting
3. Multi-provider mixing (s09 expands this) needs a unified raw HTTP layer

s04 ports that layer to Go.

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

`toAnthropic` is the protocol adapter — it converts our flat
`Message{Role,Content,ToolCalls}` to the content-block shape Anthropic wants:

| Ours | Anthropic |
|------|-----------|
| `Role:"system"` | top-level `system` field (multiple `system` joined with `\n`) |
| `Role:"user", Content:"hi"` | `{role:"user", content:[{type:"text", text:"hi"}]}` |
| `Role:"assistant", ToolCalls:[...]` | `{role:"assistant", content:[{type:"text",...}, {type:"tool_use",...}]}` |
| `Role:"tool"` | `{role:"user", content:[{type:"tool_result", tool_use_id:"...", content:"..."}]}` |

Note the last row: **tool_result must live inside the user role** — that's
Anthropic's requirement. Our flat `Role:"tool"` is an internal convenience;
it gets translated before sending.

## How It Works

`parseSSE` is the must-read code in this chapter. Anthropic's SSE event
names:

```
event: message_start         ← preamble, no data
event: content_block_start   ← a block (text or tool_use) begins
event: content_block_delta   ← delta (text_delta or input_json_delta)
event: content_block_stop    ← current block ends
event: message_delta         ← carries stop_reason / usage
event: message_stop          ← stream ends
event: ping                  ← heartbeat, ignore
event: error                 ← server-side error
```

Code skeleton (compressed):

```go
for scanner.Scan() {
    if !strings.HasPrefix(line, "data:") { continue }
    json.Unmarshal(payload, &ev)
    switch ev["type"] {
    case "content_block_start":
        // record blockType, blockToolID, blockToolNm, reset builders
    case "content_block_delta":
        if delta.type == "text_delta":
            // push to chunks, append to blockText
        if delta.type == "input_json_delta":
            // append partial_json to blockJSON (don't push chunks — these are tool args)
    case "content_block_stop":
        // end of block: text → out.Content; tool_use → unmarshal blockJSON → out.ToolCalls
    case "message_stop":
        return out, nil
    case "error":
        return out, fmt.Errorf(...)
    }
}
```

Two key points:

1. **`text_delta` pushes to chunks; `input_json_delta` does not** — the first
   is user-visible text, the second is JSON fragments of tool arguments
   (`"text"` arrives first, then `":"`, then `"hi"`...). Streaming partial
   JSON to the user is meaningless.

2. **`partial_json` is fragment concatenation** — you cannot try
   `json.Unmarshal` on each fragment (intermediate states are invalid JSON).
   Buffer until `content_block_stop`, then unmarshal as a whole.

## What Changed

Precise diff vs s03:

| File | Change |
|------|--------|
| `types.go` | Unchanged (reuses s03's `Message/StepOutcome/Provider`) |
| `loop.go` | Unchanged (s03's loop, not one line touched!) |
| `registry.go` | Unchanged |
| `mock.go` | **Removed** — no longer needed |
| `anthropic.go` | **New** — `AnthropicProvider`, `toAnthropic`, `parseSSE` |
| `main.go` | Now uses `NewAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY"), model)` |
| `anthropic_test.go` | Tests use `httptest.NewServer` + fake SSE — no real API key needed |

**This is s04's biggest pedagogical takeaway**: because s01–s03 got the
abstractions right, swapping mock for real LLM **changes a single file**.

## Try It

Offline tests (no API key needed):

```bash
cd agents/s04-claude
go test -count=1 ./...
```

Live run (with API key):

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run . -user "What's 2+2?"
```

Optional `-model` to switch:

```bash
go run . -model claude-sonnet-4-6 -user "Write a haiku about goroutines"
```

## Upstream Source Reading

Upstream `llmcore.py` lines 1-80 are mykey loading + history compression.
The real SSE parser is `_parse_claude_sse` (search for that function name).
Structural parallel:

```python
# llmcore.py:_parse_claude_sse (paraphrased)
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
                yield payload['delta']['text']    ← pushed to generator consumer
                content_buf += payload['delta']['text']
            elif payload['delta'].get('type') == 'input_json_delta':
                cur_tool['input'] += payload['delta']['partial_json']
        elif payload['type'] == 'content_block_stop':
            if cur_tool: tool_calls.append(...); cur_tool = None
        elif payload['type'] == 'message_stop': break
```

Almost line-for-line with our Go version — Python `yield` ≈ Go
`chunks <- text`.

Upstream's [`_stream_with_retry`](https://github.com/lsdefine/GenericAgent/blob/main/llmcore.py)
adds exponential backoff retry and a `compress_history_tags` call. Production
optimizations we skip in s04. s09 will handle retry/failover (but at the
MixinSession layer).

Next: [s05-coderun](s05-coderun.md) — the first real tool: spawn a subprocess
to run Python/bash and stream stdout back live.
