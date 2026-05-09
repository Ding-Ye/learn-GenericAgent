# s10 — 反射模式与自动调度

## Problem

到这一步 agent 是**被动的**——必须有人输入 `prompt` 才能开始干活。但常见需求是：
- "每 5 分钟检查一次邮箱，有新邮件就总结"
- "Github CI 红了就告诉我"
- "桌面 idle 30 分钟自动整理今天的笔记"

这些场景里 agent 是被**事件**或**时间**唤醒的。GenericAgent 的解法叫 **reflect mode**：
启动 `python agentmain.py --reflect monitor.py`，monitor.py 里定义 `check()` 函数返回
"有任务" / "无任务"。Agent 每 INTERVAL 秒调一次 check()。

s10 用 Go 复刻这个机制——但有个有意思的本地化：
**Go 没法 hot-reload .py 源码**，所以我们用 **JSON config 文件 + mtime 检测**替代。

## Solution

```
   ┌────────────────────────┐
   │ ReflectLoop            │
   │   Check    CheckFunc   │ ◀── 用户提供
   │   Agent    AgentRunner │ ◀── 真正的 agent
   │   Interval time.Duration│
   │   OnDone   func(...)   │
   └──────────┬─────────────┘
              │ Run(ctx)
              ▼
        ┌──────────┐
        │ for {    │  every Interval (or override Sleep)
        │   tick++ │
        │   r := Check()
        │   if r.Exit: return
        │   if r.Task != "":
        │     out, err := Agent.Run(task)
        │     OnDone(task, out, err)
        │     if Once: return
        │ }        │
        └──────────┘
```

`CheckFunc` 签名：

```go
type CheckFunc func(ctx context.Context, tick int) (CheckResult, error)

type CheckResult struct {
    Task    string         // 非空 → 派任务
    Exit    bool           // true → 终止 reflect
    Sleep   time.Duration  // 覆盖下次 check 间隔
    SleepMS int            // JSON 友好版（unmarshal 后转 Sleep）
}
```

## How It Works

`time.Timer.Reset()` 让我们能在 check 返回 `Sleep` 时动态调间隔：

```go
timer := time.NewTimer(r.Interval)
for {
    select {
    case <-ctx.Done(): return ctx.Err()
    case <-timer.C:
        result, _ := r.Check(ctx, tick++)
        if result.Exit { return nil }
        if result.Task != "" {
            out, err := r.Agent.Run(ctx, result.Task)
            r.OnDone(result.Task, out, err)
            if r.once { return nil }
        }
        next := r.Interval
        if result.Sleep > 0 { next = result.Sleep }
        timer.Reset(next)
    }
}
```

`JSONCheck(path)` 是常用的 CheckFunc 工厂：每次调用读 mtime，没变就用缓存值，
变了就 unmarshal 新 JSON。这样**修改文件就能实时改变 agent 行为**——
等价于上游的"修改 .py 文件，agent 自动 reload"。

## What Changed

| 文件 | 内容 |
|------|------|
| `reflect.go` | `ReflectLoop` + `JSONCheck` + 类型 |
| `main.go` | CLI: 演示用 stub agent |
| `reflect_test.go` | 7 个测试 |

不引入新的 loop / provider 概念。本节是**外层调度器**——把 s01-s09 的 agent 套在
一个周期触发的外壳里。

## Try It

```bash
cd agents/s10-reflect
echo '{}' > /tmp/reflect.json   # 初始空配置
go run . -config /tmp/reflect.json -interval 1 -once &

# 在另一个 terminal：
echo '{"task":"hello there"}' > /tmp/reflect.json

# 1 秒内会看到:
# [agent #1] received: hello there
# [done] task="hello there" result="HELLO THERE"
```

实验：让 check 返回 `Sleep: 50*time.Millisecond` 抓快频；返回 `Sleep: 10*time.Second`
抓慢频。频率是动态可控的。

## Upstream Source Reading

`agentmain.py` 的 reflect 部分（搜 `args.reflect`）：

```python
elif args.reflect:
    agent.peer_hint = False
    import importlib.util
    spec = importlib.util.spec_from_file_location('reflect_script', args.reflect)
    mod = importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
    _mt = os.path.getmtime(args.reflect)
    print(f'[Reflect] loaded {args.reflect}')
    while True:
        if os.path.getmtime(args.reflect) != _mt:        ← 文件 mtime 变 → reload
            try: spec.loader.exec_module(mod); _mt = os.path.getmtime(args.reflect)
            except Exception as e: print(f'[Reflect] reload error: {e}')
        time.sleep(getattr(mod, 'INTERVAL', 5))           ← INTERVAL 模块全局变量
        try: task = mod.check()
        except Exception as e: print(f'[Reflect] check() error: {e}'); continue
        if task and task == '/exit': break
        if task is None: continue
        print(f'[Reflect] triggered: {task[:80]}')
        dq = agent.put_task(task, source='reflect')
        try:
            while 'done' not in (item := dq.get(timeout=180)): pass
            result = item['done']
        except Exception as e:
            if getattr(mod, 'ONCE', False): raise
            result = f'[ERROR] {e}'
        log_dir = os.path.join(script_dir, 'temp/reflect_logs'); os.makedirs(log_dir, exist_ok=True)
        open(os.path.join(log_dir, f'{script_name}_{datetime.now():%Y-%m-%d}.log'), 'a').write(...)
        if (on_done := getattr(mod, 'on_done', None)):
            try: on_done(result)
            except Exception as e: print(f'[Reflect] on_done error: {e}')
        if getattr(mod, 'ONCE', False): break
```

行为对照：

| 上游 | 我们 |
|------|------|
| 给 reflect.py 写 `INTERVAL = 5` | `NewReflectLoop(check, agent, 5*time.Second)` |
| 给 reflect.py 写 `ONCE = True` | `loop.Once()` |
| 给 reflect.py 写 `def check(): ...` 返回 None/str | `CheckFunc` 返回 `CheckResult` |
| 写 `def on_done(result): ...` | `loop.OnDone = func(...)` |
| 修改 .py 文件触发 reload | 修改 JSON 文件触发缓存失效 |
| `task == '/exit': break` | `CheckResult{Exit: true}` |

为什么用 JSON 而不是 plugin/RPC？因为 reflect 的本质是"低频触发"——
绝大多数 case 是"看一个状态文件 / 一个时间 / 一个 webhook 收件箱"。
JSON 文件已经能表达 99% 的需求。复杂场景下再上 RPC 不晚。

下一步：[s_full](../s_full-integration.md) 把 s01-s10 拼成一个能跑的最小 GenericAgent。
