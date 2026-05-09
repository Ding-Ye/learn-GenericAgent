# Upstream reading for s09 — `MixinSession`

Search [`llmcore.py`](https://github.com/lsdefine/GenericAgent/blob/main/llmcore.py) for `class MixinSession`.

## What it does

Wraps multiple `BaseSession` instances behind one. The agent loop calls
`mixin.raw_ask(messages)` and gets back whichever session succeeded, with
rotation/spring-back logic invisible to the caller.

## Implementation outline

```python
class MixinSession:
    def __init__(self, all_sessions, cfg):
        # all_sessions = dict-by-name
        # cfg = {'mode': 'fallback', 'order': ['anthropic_api', 'oai_config']}
        order = cfg['order']
        self._sessions = [all_sessions[k] for k in order]
        self._priority = [0] * len(self._sessions)
        # __getattr__/__setattr__ proxy to current session for transparency

    def _pick(self):
        # priority-sorted, cooldown-aware
        ...

    def _raw_ask(self, *args, **kwargs):
        for _ in range(len(self._sessions)):
            i, s = self._pick()
            try: return s.raw_ask(*args, **kwargs)
            except RateLimitError: cool down s; continue
            except (Network, Server): demote priority; continue
        raise
```

## Subtleties worth knowing

1. **`__getattr__` proxies non-existent attribute access to the current
   session**. So `mixin.history` returns `self._sessions[current].history`.
   This makes the mixin **transparent**: code that used a `NativeClaudeSession`
   keeps working when handed a `MixinSession`. Our Go version doesn't replicate
   this; the loop only ever calls `Chat()`, so the mixin only needs to be a
   `Provider`.

2. **Spring-back via priority**. When session 0 (primary) is in cooldown and
   session 1 succeeds, `_priority[1] += 1`. Then `_priority[0] = max(_priority[0], _priority[1] - 1)` 
   *bumps the primary up too* so it can be retried sooner. Subtle and
   pretty.

3. **Cooldown is timer-based, not call-count-based.** A 60-second cooldown
   uses `threading.Timer(60, lambda: setattr(s, '_cooled', False))`. Our Go
   version uses a single `time.Time` cooldown, which is even simpler.

## What's hard about this in production

- **History shared across providers** — if you've been talking to provider A
  and fall over to provider B, B sees the conversation built up via A. Most
  models tolerate this fine, but cache markers and provider-specific tokens
  may not transfer cleanly.
- **Tool descriptions also need translating** — Anthropic's `tools` schema
  vs OpenAI's `tools` schema are similar but not identical. Upstream's
  `openai_tools_to_claude` and `_msgs_claude2oai` handle this.

For the learn version, we keep the schema unified at the `ToolSpec` level
and let each provider's `Chat()` adapter translate.

## Read these

- `class MixinSession` (~80 lines)
- `_msgs_claude2oai` (search for that function name) — message-shape adapter
- `openai_tools_to_claude` (search) — tool-spec adapter
