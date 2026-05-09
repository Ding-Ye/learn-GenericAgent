# s09 — 多 provider 故障切换

## Problem

接入真实 LLM 之后会遇到不愉快的现实：
- Anthropic API 429 限流，要等 30 秒
- OpenAI 5xx 偶发抖动
- DeepSeek 区域性宕机
- 你的 Claude API key 用完了，自己的 key 还有额度

GenericAgent 上游配多 key + 多 provider：在 `mykey.py` 里写：
```python
anthropic_api = {'key': 'sk-ant-...', 'model': 'claude-haiku-4-5-20251001'}
oai_config = {'url': 'https://api.deepseek.com/v1', 'key': 'sk-...', 'model': 'deepseek-chat'}
mixin_config = {'mode': 'fallback', 'order': ['anthropic_api', 'oai_config']}
```

`MixinSession` 自动按 order 尝试，主路炸了用 fallback，fallback 一段时间后 spring back 主路。

s09 用 Go 复刻这一层。

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

`Provider` 接口加了 `Name()` 方法（用于日志和诊断）。

## How It Works

3 个关键决策：

### 1. 组合而非继承

`MixinProvider` 自身实现 `Provider` 接口——意味着上层 `Run()` 完全不知道下面是
单 provider 还是 mixin。这是 Go 接口的精髓。

### 2. Spring-back 通过冷却时间实现

```go
func (m *MixinProvider) pickStart() int {
    if time.Now().After(m.cooldown) { m.current = 0 }  // spring back
    return m.current
}
```

第一次 fallback 成功后设 `cooldown = now + 500ms`。这 500ms 内每次 Chat 直接从 fallback 开始；
500ms 后下次 Chat 重置回 primary。这给 primary "缓冲恢复时间"，又不让我们一直黏在 fallback。

### 3. 可重试错误用 substring 启发式

```go
func isRetryable(err error) bool {
    msg := strings.ToLower(err.Error())
    for _, kw := range []string{"timeout", "rate limit", "429", "503", ...} {
        if strings.Contains(msg, kw) { return true }
    }
}
```

简陋但够用。生产代码会换成 typed errors / HTTP status code 检查；学习版选最低复杂度。

## What Changed

| 文件 | 变更 |
|------|------|
| `types.go` | `Provider` 接口加 `Name() string` 方法 |
| `mixin.go` | 新增——`MixinProvider` 类型 + 7 个 helper |
| `main.go` | 用两个内置 fakeProvider 演示 |
| `mixin_test.go` | 7 个测试 |

注意 `Provider` 接口的小变更：加了 `Name()`。这是**真实的破坏性变更**——s04 的
`AnthropicProvider` 在 s_full 集成时也得加这一方法。

## Try It

```bash
cd agents/s09-mixin
go run .
```

输出：

```
[mixin] try=primary
[mixin] primary failed (retryable): rate limit exceeded; falling over

[mixin] try=fallback
[fallback] hello

[final] [fallback] hello
```

实验：在 `mixin_test.go::TestMixin_SpringBackAfterCooldown` 里把 `stickyMS` 调到 2000，
把 `time.Sleep` 调到 100ms，看测试期望失败——能直观感受 spring-back 阈值的作用。

## Upstream Source Reading

`llmcore.py:MixinSession` ~80 行。结构对比：

```python
class MixinSession:
    def __init__(self, all_sessions, cfg):
        # cfg = {'mode': 'fallback', 'order': ['anthropic_api', 'oai_config']}
        order = cfg['order']
        self._sessions = [all_sessions[k] for k in order]
        self._priority = [0] * len(self._sessions)  # spring-back accumulator

    def _pick(self):
        # 找 _priority 最高、且不在冷却的 session
        candidates = [(i, s) for i, s in enumerate(self._sessions) if not s._cooled]
        if not candidates: candidates = [(i, s) for i, s in enumerate(self._sessions)]
        # 按 priority 倒排
        candidates.sort(key=lambda x: -self._priority[x[0]])
        return candidates[0]

    def _raw_ask(self, *args, **kwargs):
        for tried in range(len(self._sessions)):
            i, s = self._pick()
            try:
                ret = s.raw_ask(*args, **kwargs)
                self._priority[i] += 1   # 成功增加优先级
                self._priority[0] = max(self._priority[0], self._priority[i] - 1)  # spring-back
                return ret
            except RateLimitError:
                self._priority[i] -= 5
                s._cooled = True
                threading.Timer(60, lambda: setattr(s, '_cooled', False)).start()
                continue
            except (NetworkError, ServerError):
                self._priority[i] -= 1
                continue
            # other errors: re-raise
            raise
        raise Exception('all sessions failed')
```

跟我们 Go 版本对照：

| 上游 | 我们 |
|------|------|
| `_priority[]` 累计成功失败 | 简化为单值 `current` + `cooldown` |
| `_cooled` + threading.Timer | `cooldown time.Time` 单字段 |
| 区分 RateLimit / Network / Server / Other | substring match in `isRetryable()` |
| `_pick()` 按优先级排序 | 从 `current` 开始线性试 |

我们的 Go 版本牺牲了 priority 累积的细腻性，换来了 ~80 行变 ~80 行的代码量持平和**无锁的并发安全**。
真要做长期统计的话，加个 `metric exporter` 监听 chunks 比内嵌 priority 更好——
观测和决策应该解耦。

## Phase G — 多模型 addendum

GenericAgent 上游本身已经是"多模型"框架。本课程把"多模型"概念分给了两节：
- s04 学单 provider 怎么接（Anthropic）
- s09 学多 provider 怎么编排（Mixin）

所以**没有单独的 Phase G addendum**——你已经看完了。要换 OpenAI/DeepSeek/Gemini 任一 provider，
只要照 s04 的样子写个新的 `Provider` 实现，再注入到 `MixinProvider` 即可。

下一节 [s10-reflect](s10-reflect.md)：反射模式——agent 不再被动等命令，而是被周期性 trigger 唤醒。
