# s09 — Multi-Provider Failover

## Problem

Once you wire in real LLMs, reality bites:
- Anthropic API 429s; you wait 30 seconds
- OpenAI 5xx flaps occasionally
- DeepSeek has regional outages
- Your Claude API key burns through; another key still has budget

Upstream GenericAgent supports multi-key + multi-provider — in `mykey.py`:
```python
anthropic_api = {'key': 'sk-ant-...', 'model': 'claude-haiku-4-5-20251001'}
oai_config = {'url': 'https://api.deepseek.com/v1', 'key': 'sk-...', 'model': 'deepseek-chat'}
mixin_config = {'mode': 'fallback', 'order': ['anthropic_api', 'oai_config']}
```

`MixinSession` walks the order, falls back when primary fails, springs back
to primary after a cooldown.

s09 ports this layer to Go.

## Solution

```
   ┌───────────────────────────────────┐
   │ MixinProvider implements Provider │
   │  primaries []Provider             │
   │  current  int                     │
   │  cooldown time.Time               │
   │  stickyMS int                     │
   └───────────────┬───────────────────┘
                   │ Chat()
                   ▼
       try providers[current]
        │
        ├── success → recordSuccess(current)
        │   if current != 0:
        │       cooldown = now + stickyMS
        │
        └── retryable error → try next; loop
                              if all fail → return wrapped error
```

The `Provider` interface gains a `Name()` method (for logging and
diagnostics).

## How It Works

Three key choices:

### 1. Composition, not inheritance

`MixinProvider` itself implements `Provider` — meaning the upstream `Run()`
has no idea whether it's talking to a single provider or a mixin. That's
the essence of Go interfaces.

### 2. Spring-back via cooldown timestamp

```go
func (m *MixinProvider) pickStart() int {
    if time.Now().After(m.cooldown) { m.current = 0 }  // spring back
    return m.current
}
```

After a fallback succeeds, set `cooldown = now + 500ms`. Within those 500ms,
every Chat starts directly from the fallback; after 500ms the next call
resets to primary. This gives the primary a recovery window without leaving
us stuck on the fallback forever.

### 3. Retryable errors via substring heuristic

```go
func isRetryable(err error) bool {
    msg := strings.ToLower(err.Error())
    for _, kw := range []string{"timeout", "rate limit", "429", "503", ...} {
        if strings.Contains(msg, kw) { return true }
    }
}
```

Crude but enough. Production code would use typed errors / HTTP-status
checks; the learn version picks minimum complexity.

## What Changed

| File | Change |
|------|--------|
| `types.go` | `Provider` interface gains `Name() string` method |
| `mixin.go` | New — `MixinProvider` type + 7 helpers |
| `main.go` | Two built-in fakeProviders for the demo |
| `mixin_test.go` | 7 tests |

Note the small breaking change to `Provider`: it gains `Name()`. That's a
**real breaking change** — when s_full integrates s04's `AnthropicProvider`
it'll also need this method.

## Try It

```bash
cd agents/s09-mixin
go run .
```

Output:

```
[mixin] try=primary
[mixin] primary failed (retryable): rate limit exceeded; falling over

[mixin] try=fallback
[fallback] hello

[final] [fallback] hello
```

Experiment: in `mixin_test.go::TestMixin_SpringBackAfterCooldown` raise
`stickyMS` to 2000 and lower `time.Sleep` to 100ms. The test should fail —
useful intuition for the spring-back threshold.

## Upstream Source Reading

`llmcore.py:MixinSession` is ~80 lines. Structural diff:

```python
class MixinSession:
    def __init__(self, all_sessions, cfg):
        # cfg = {'mode': 'fallback', 'order': ['anthropic_api', 'oai_config']}
        order = cfg['order']
        self._sessions = [all_sessions[k] for k in order]
        self._priority = [0] * len(self._sessions)  # spring-back accumulator

    def _pick(self):
        # find session with highest _priority that's not in cooldown
        candidates = [(i, s) for i, s in enumerate(self._sessions) if not s._cooled]
        if not candidates: candidates = [(i, s) for i, s in enumerate(self._sessions)]
        candidates.sort(key=lambda x: -self._priority[x[0]])
        return candidates[0]

    def _raw_ask(self, *args, **kwargs):
        for tried in range(len(self._sessions)):
            i, s = self._pick()
            try:
                ret = s.raw_ask(*args, **kwargs)
                self._priority[i] += 1                                  # success bumps
                self._priority[0] = max(self._priority[0], self._priority[i] - 1)  # spring back
                return ret
            except RateLimitError:
                self._priority[i] -= 5
                s._cooled = True
                threading.Timer(60, lambda: setattr(s, '_cooled', False)).start()
                continue
            except (NetworkError, ServerError):
                self._priority[i] -= 1
                continue
            raise
        raise Exception('all sessions failed')
```

Mapping:

| Upstream | Ours |
|----------|------|
| `_priority[]` accumulates successes/failures | Simplified to one `current` + `cooldown` |
| `_cooled` + threading.Timer | One `cooldown time.Time` field |
| Differentiates RateLimit / Network / Server / Other | substring match in `isRetryable()` |
| `_pick()` sorts by priority | Linear from `current` |

We trade priority-accumulation nuance for ~80 lines that stay ~80 lines and
**lock-free concurrency safety**. For real long-term stats you'd be better
off exporting metrics off the chunks and deciding outside — observation and
decision should decouple.

## Phase G — Multi-Model Addendum

Upstream GenericAgent is itself a "multi-model" framework. This curriculum
splits the "multi-model" concept across two chapters:
- s04 teaches how to wire one provider (Anthropic)
- s09 teaches how to orchestrate many (Mixin)

So **there is no separate Phase G addendum** — you've already seen it. To
add OpenAI / DeepSeek / Gemini, just follow s04's pattern to write a new
`Provider` impl and inject it into `MixinProvider`.

Next: [s10-reflect](s10-reflect.md) — Reflect mode: the agent stops waiting
for commands and starts being woken up by periodic triggers.
