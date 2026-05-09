# s10-reflect — Reflect mode & autonomous scheduling

`ReflectLoop` polls a user-supplied `CheckFunc` on an interval. When the
check returns a non-empty `Task`, that task is handed to an `AgentRunner`.

```bash
echo '{}' > /tmp/reflect.json   # empty initially
go run . -config /tmp/reflect.json -interval 1 -once &
sleep 1.5
echo '{"task":"hello there"}' > /tmp/reflect.json
# Within 1s the agent fires; you'll see [agent #1] received: hello there
```

## API

- `NewReflectLoop(check, agent, interval) *ReflectLoop`
- `loop.Once()` — exit after first task completes
- `loop.OnDone = func(task, result, err)` — observer callback
- `JSONCheck(path) CheckFunc` — file-based config with mtime hot-reload

## Tests

7 cases: fires on task, exits on `Exit:true`, ctx cancellation propagates,
check error terminates, override sleep, JSON hot-reload, `OnDone` invoked.
