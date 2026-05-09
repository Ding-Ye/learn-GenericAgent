# s10 — Reflect Mode & Autonomous Scheduling

## Problem

Up to here the agent is **passive** — somebody must type a prompt to start
work. Common needs:
- "Every 5 minutes check inbox; summarize new mail"
- "Tell me when GitHub CI goes red"
- "After 30 minutes idle, organize today's notes"

In these scenarios the agent is woken up by **events** or **time**.
GenericAgent's solution is **reflect mode**: launch
`python agentmain.py --reflect monitor.py` where monitor.py defines a
`check()` function that returns "task" / "no task." The agent calls
check() every INTERVAL seconds.

s10 ports this in Go — with one localization: **Go can't hot-reload .py
sources**, so we use **JSON config + mtime detection** instead.

## Solution

```
   ┌────────────────────────┐
   │ ReflectLoop            │
   │   Check    CheckFunc   │ ◀── user-supplied
   │   Agent    AgentRunner │ ◀── the real agent
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

`CheckFunc` signature:

```go
type CheckFunc func(ctx context.Context, tick int) (CheckResult, error)

type CheckResult struct {
    Task    string         // non-empty → dispatch
    Exit    bool           // true → terminate
    Sleep   time.Duration  // override next interval
    SleepMS int            // JSON-friendly form (unmarshal → Sleep)
}
```

## How It Works

`time.Timer.Reset()` lets us dynamically change interval when check returns
`Sleep`:

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

`JSONCheck(path)` is a common CheckFunc factory: each call reads mtime,
serves cached value if unchanged, otherwise unmarshals fresh JSON.
Editing the file changes agent behavior in real time — equivalent to
upstream's "edit .py and the agent auto-reloads."

## What Changed

| File | Content |
|------|---------|
| `reflect.go` | `ReflectLoop` + `JSONCheck` + types |
| `main.go` | CLI demo with a stub agent |
| `reflect_test.go` | 7 tests |

No new loop / provider concepts. This chapter is the **outer scheduler** —
it wraps the agent from s01-s09 in a periodic-trigger shell.

## Try It

```bash
cd agents/s10-reflect
echo '{}' > /tmp/reflect.json   # empty initially
go run . -config /tmp/reflect.json -interval 1 -once &

# In another terminal:
echo '{"task":"hello there"}' > /tmp/reflect.json

# Within 1 second:
# [agent #1] received: hello there
# [done] task="hello there" result="HELLO THERE"
```

Experiment: have check return `Sleep: 50*time.Millisecond` for fast polling;
`Sleep: 10*time.Second` for slow. Frequency is dynamically tunable.

## Upstream Source Reading

`agentmain.py`'s reflect block (search `args.reflect`):

```python
elif args.reflect:
    agent.peer_hint = False
    import importlib.util
    spec = importlib.util.spec_from_file_location('reflect_script', args.reflect)
    mod = importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
    _mt = os.path.getmtime(args.reflect)
    print(f'[Reflect] loaded {args.reflect}')
    while True:
        if os.path.getmtime(args.reflect) != _mt:        ← mtime change → reload
            try: spec.loader.exec_module(mod); _mt = os.path.getmtime(args.reflect)
            except Exception as e: print(f'[Reflect] reload error: {e}')
        time.sleep(getattr(mod, 'INTERVAL', 5))           ← INTERVAL module global
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

Behavior parallel:

| Upstream | Ours |
|----------|------|
| `INTERVAL = 5` in reflect.py | `NewReflectLoop(check, agent, 5*time.Second)` |
| `ONCE = True` in reflect.py | `loop.Once()` |
| `def check(): ...` returning None/str | `CheckFunc` returning `CheckResult` |
| `def on_done(result): ...` | `loop.OnDone = func(...)` |
| Edit .py to reload | Edit JSON to invalidate cache |
| `task == '/exit': break` | `CheckResult{Exit: true}` |

Why JSON instead of a plugin/RPC? Because reflect is fundamentally a
**low-frequency trigger** — most cases are "watch one state file / time /
webhook inbox." A JSON file expresses 99% of needs. Add RPC later only if
you actually need it.

Next: [s_full](../s_full-integration.md) — wire s01-s10 into a runnable
mini-GenericAgent.
