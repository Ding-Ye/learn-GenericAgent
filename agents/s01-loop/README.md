# s01-loop — The minimal agent loop

The kernel of every "agent" framework is shockingly small: a `for` loop that
sends a conversation history to an LLM and appends the reply.

In s01 we build exactly that, in ~70 lines of Go, with a `MockProvider` so you
can run the loop end-to-end with no API key.

## Run it

```bash
go run . -user "What can you do?"
```

You should see `--- Turn 1 ---` then a streamed mock reply, then
`[exit] reason=TASK_DONE turns=1`.

## Test it

```bash
go test -count=1 ./...
```

## What's here

| File | What it is |
|------|------------|
| `types.go` | `Message`, `Response`, `Provider` interface |
| `loop.go` | `Run(ctx, provider, sys, user, maxTurns, chunks) (ExitInfo, error)` |
| `mock.go` | `MockProvider` — canned replies, optional stream delay |
| `main.go` | CLI: `go run . -user "..."` |

## What's intentionally missing

The s01 loop has **no** tool dispatch, **no** real provider, **no** memory.
Every assistant reply ends the task. Future sessions add layers.

## Upstream lineage

This loop is a stripped version of
[`agent_loop.py:agent_runner_loop`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py)
in lsdefine/GenericAgent. See `docs/zh/s01-loop.md` (or `/en`) for the
six-section walk-through.
