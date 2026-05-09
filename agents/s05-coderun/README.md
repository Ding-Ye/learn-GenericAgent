# s05-coderun — Streaming code execution tool

`CodeRun(ctx, code, type, timeout, chunks)` spawns a child process
(`python3 -c` or `bash -c`), streams stdout into `chunks` line-by-line, and
returns a `CodeRunResult{Status, Stdout, ExitCode, Truncated}`.

```bash
go run . -type bash -code "for i in 1 2 3; do echo \$i; sleep 0.2; done"
go test -count=1 ./...
```

## Lessons

- `exec.CommandContext` carries cancellation
- Merging stderr into stdout via `cmd.Stderr = cmd.Stdout`
- `bufio.Scanner` with a 1 MiB buffer for tolerant line reading
- 16 KiB stdout budget with `Truncated` flag (mirrors upstream's smart_format)
- Backtick escaping (`compactLine`) so output never closes a code fence
