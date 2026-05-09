# Upstream reading for s10 — reflect mode

## Files

- [`agentmain.py`](https://github.com/lsdefine/GenericAgent/blob/main/agentmain.py) — search for `args.reflect` to find the loop (~25 lines)
- [`reflect/scheduler.py`](https://github.com/lsdefine/GenericAgent/blob/main/reflect/scheduler.py) — example reflect script: time-based scheduler
- [`reflect/goal_mode.py`](https://github.com/lsdefine/GenericAgent/blob/main/reflect/goal_mode.py) — example reflect script: goal-driven autonomous run
- [`reflect/autonomous.py`](https://github.com/lsdefine/GenericAgent/blob/main/reflect/autonomous.py) — minimal seed: just a `check()` returning `None`

## Anatomy of a reflect script

```python
# Module globals checked by agentmain.py
INTERVAL = 5      # seconds between check() calls; default 5 if absent
ONCE = False      # exit after first task completes

# Required:
def check():
    """Return a task string, or None to skip this tick, or '/exit' to bail."""
    return None

# Optional:
def on_done(result):
    """Called once per completed task with the agent's final response."""
    pass
```

That's the whole contract. `agentmain.py` calls `check()`, dispatches to
`agent.put_task(task)`, waits for the response, calls `on_done(result)`.

## Two real reflect scripts to read

### `reflect/scheduler.py` (~120 lines)

A cron-style scheduler stored in `temp/scheduled.json`. Each entry has a
cron expression and a task. `check()` checks if any entry's next-fire-time
has passed and returns its task. After firing, it bumps the entry's next
fire to the next match.

### `reflect/goal_mode.py` (~80 lines)

A "stay-alive autonomously toward a goal" mode. `check()` returns
`'continue toward goal: <goal>'` until external state says the goal is
reached. The agent works through tools, you just have to set a goal and
walk away.

These are great examples of the pattern. Both are < 200 lines. Both are
specific scripts a user wrote — not built into the framework.

## What we changed in the Go version

| Upstream | Ours | Why |
|----------|------|-----|
| Module-global `INTERVAL` & `ONCE` | `loop.Interval` field & `.Once()` method | Go has no module-level mutability convention |
| Module-global `def check()` | `CheckFunc` typed callback | Type-safe and testable |
| Hot-reload via `spec.loader.exec_module(mod)` | mtime-aware JSON reload | Go has no Python-style re-exec |

The structural ideas — periodic check + task dispatch + optional on_done +
once mode — are preserved.

## Read order

1. `agentmain.py` reflect block (~25 lines) — the loop
2. `reflect/autonomous.py` (200 bytes) — minimal viable script
3. `reflect/scheduler.py` (~120 lines) — real cron-style example
4. `reflect/goal_mode.py` (~80 lines) — real goal-driven example
