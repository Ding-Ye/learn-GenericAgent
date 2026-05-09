# s03-outcome — StepOutcome control flow

Tools now return a `StepOutcome{Data, NextPrompt, ShouldExit}` instead of
`(any, error)`. The loop reads outcomes per turn and decides:

- any `ShouldExit=true` → loop exits with reason `EXITED`
- all `NextPrompt==""` → loop exits with reason `TASK_DONE`
- otherwise → next user message is the union of `NextPrompt`s

Three demo tools — `finish_task`, `think_again`, `done` — show all three paths.

```bash
go run .
go test -count=1 ./...
```
