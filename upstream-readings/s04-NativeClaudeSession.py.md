# Upstream reading for s04 — `NativeClaudeSession` & SSE parsing

[`llmcore.py`](https://github.com/lsdefine/GenericAgent/blob/main/llmcore.py) is the largest file in upstream (~1500 LOC). Most of it is provider plumbing. The slices that map to our s04 are:

## `NativeClaudeSession`

Search the file for `class NativeClaudeSession`. Skeleton:

```python
class NativeClaudeSession:
    def __init__(self, cfg):
        self.api_key = cfg['key']
        self.url     = cfg.get('url', 'https://api.anthropic.com/v1/messages')
        self.model   = cfg['model']
        self.max_tokens = cfg.get('max_tokens', 4096)
        self.headers = {
            'x-api-key': self.api_key,
            'anthropic-version': '2023-06-01',
            'content-type': 'application/json',
            # browser-like extras for context-1m + interleaved-thinking betas
            'anthropic-beta': 'context-1m-2025-08-07,interleaved-thinking-2025-05-14',
        }
        self.history = []   # ← the *real* conversation log, not what's sent each turn

    def raw_ask(self, messages):
        body = {
            "model": self.model, "max_tokens": self.max_tokens,
            "system": <extracted system>,
            "messages": <converted>,
            "tools": <openai_tools_to_claude(...)>,
            "stream": True,
        }
        resp = requests.post(self.url, headers=self.headers, json=body, stream=True, verify=False)
        return _parse_claude_sse(resp.iter_lines())
```

Note `self.history` — the session keeps its own log. The loop only passes
the *new turn's* payload to `raw_ask`. The session is responsible for
appending the new user/assistant messages, doing `compress_history_tags`,
and rotating cache markers.

## `_parse_claude_sse`

The line-by-line SSE walker. Already shown in our `docs/zh|en/s04-claude.md`.
Worth reading:

- The **`message_delta`** branch we don't emit chunks for: it carries
  `stop_reason` (`end_turn`, `tool_use`, `max_tokens`, ...) and final usage.
  Useful for billing/tracing; not strictly needed for our learn version.
- The **`ping`** event: every ~10s. We just `continue`.
- The **`error`** event: server-side errors mid-stream. Stop and surface.

## Headers worth knowing about

```
anthropic-beta: context-1m-2025-08-07,interleaved-thinking-2025-05-14,prompt-caching-2024-07-31
```

Each comma-separated value is a beta opt-in. Our s04 omits all of these to
keep the example minimal. In production you may want at least
`prompt-caching-2024-07-31` for cache-control on the system prompt.

## Recommended reading order

1. Find `class NativeClaudeSession` → read `__init__` + `raw_ask`
2. Find `_parse_claude_sse` → compare to our `parseSSE`
3. Find `_stream_with_retry` → see exponential backoff (we skip this)
4. Find `compress_history_tags` (top of file) → see history compression
5. Find `class MixinSession` → preview for s09
