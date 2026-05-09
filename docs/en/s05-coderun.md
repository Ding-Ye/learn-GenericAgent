# s05 — Streaming Code Execution Tool

## Problem

The single most-used agent tool is "run code." Upstream's `code_run` is its
most powerful tool — it runs arbitrary Python/bash, streams stdout back, and
the user can interrupt it.

Doing this right means solving four problems:

1. **Streaming output** — a 30-second subprocess shouldn't make the user wait
   30 seconds; they should see each stdout line as it appears.
2. **Timeout** — an infinite loop must be killed after a budget.
3. **Cancellation** — Ctrl-C should kill the subprocess immediately.
4. **Output budget** — LLM context is finite; you can't stuff 10 MB of
   stdout back into the model.

## Solution

```
   ctx ──┐                   ┌── stdout pipe ──► bufio.Scanner ──► chunks <- line
         │                   │                               └─► buf (capped 16KB)
         ▼                   │
   exec.CommandContext ──► cmd.Start()
         ▲                   │
         │                   ├── timeout (context.WithTimeout)
         │                   └── cmd.Wait()
   user cancel ──┘                                              │
                                                                ▼
                              CodeRunResult{Status, Stdout, ExitCode, Truncated}
```

`exec.CommandContext` solves three of the four problems at once: pass a
`context.WithTimeout` and both timeout and cancellation share one ctx; on
ctx.Done the process is auto-SIGKILL'd.

The remaining problem is the output budget: `bufio.Scanner` accumulates as it
scans, stops appending past `stdoutBudget` but keeps pushing to chunks (so
the user can still watch it), and marks `Truncated: true` on the result.

## How It Works

The core loop is 30 lines:

```go
runCtx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
cmd := exec.CommandContext(runCtx, cmdName, cmdArgs...)
cmd.Stderr = cmd.Stdout    // merge stderr into stdout (matches upstream ga.py)
stdoutPipe, _ := cmd.StdoutPipe()
cmd.Start()

scanner := bufio.NewScanner(stdoutPipe)
scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)  // default 64K too small; bump to 1 MiB
for scanner.Scan() {
    line := scanner.Text() + "\n"
    chunks <- line                    // streaming
    if buf.Len()+len(line) <= 16*1024 {
        buf.WriteString(line)
    } else {
        truncated = true
    }
}
cmd.Wait()
```

Four details worth knowing:

1. **`cmd.Stderr = cmd.Stdout`** — redirects stderr into stdout so we read
   from one pipe. Otherwise we'd need two goroutines, increasing complexity.
2. **scanner.Buffer expansion** — `bufio.Scanner`'s default 64 KiB makes a
   100 KiB-line tool blow up with `bufio.ErrTooLong`. 1 MiB is a safer cap.
3. **`compactLine` escapes 4-backticks** — LLM markdown output often gets
   garbled by the subprocess's own stdout. When we see ```` we insert a
   zero-width space to break the fence (mirrors `ga.py:code_run` ~line 70).
4. **Timeout branch returns separately** — on timeout, `cmd.Wait` also
   returns non-zero exit, but we want the LLM to know "this was a timeout,
   not a script error" — so we check `runCtx.Err() == DeadlineExceeded` and
   surface a distinct message.

## What Changed

s05 is a **standalone tool module** — no new loop/types concepts. It only
becomes useful when mounted on the s_full registry. So the diff is mostly
new files:

| File | Content |
|------|---------|
| `coderun.go` | `CodeRun()` function + `MakeTool()` adapter |
| `main.go` | CLI: `go run . -type bash -code "..."` |
| `coderun_test.go` | 6 tests: bash/non-zero/timeout/unsupported/cancel/MakeTool |

`StepOutcome` re-appears in this module (a small standalone copy) — s_full
will unify it across sessions.

## Try It

```bash
cd agents/s05-coderun
go run . -type bash -code "for i in 1 2 3; do echo \$i; sleep 0.2; done"
```

Output:

```
1
2
3

[result] status=success exit=0 truncated=false len(stdout)=6
```

Experiment: run `sleep 10` with `-timeout 1` to hit the timeout branch:

```bash
go run . -type bash -code "sleep 10" -timeout 1
```

## Upstream Source Reading

`ga.py:code_run` is a 100-line function. Almost line-for-line:

```python
# ga.py:code_run (paraphrased)
def code_run(code, code_type="python", timeout=60, cwd=None, code_cwd=None, stop_signal=None):
    if code_type in ["python", "py"]:
        cmd = [sys.executable, "-X", "utf8", "-u", tmp_path]
    elif code_type in ["powershell", "bash", "sh", ...]:
        cmd = ["bash", "-c", code] if not windows else ["powershell", ...]

    process = subprocess.Popen(cmd, stdout=PIPE, stderr=STDOUT, ...)
    full_stdout = []
    def stream_reader():
        for line_bytes in iter(proc.stdout.readline, b''):
            line = line_bytes.decode('utf-8', errors='ignore')
            full_stdout.append(line)
            print(line, end="")    ← streaming
    t = threading.Thread(target=stream_reader); t.start()

    while t.is_alive():
        if time.time() - start_t > timeout or stop_signal:
            process.kill()
            full_stdout.append("[Timeout Error] ..." or "[Stopped] ...")
            break
        time.sleep(1)

    return {
        "status": "success" if exit_code == 0 else "error",
        "stdout": smart_format(stdout_str, max_str_len=10000),
        "exit_code": exit_code,
    }
```

Exact correspondence:

| Upstream | Our Go |
|----------|--------|
| `subprocess.Popen(cmd, stdout=PIPE, stderr=STDOUT)` | `exec.CommandContext` + `cmd.Stderr = cmd.Stdout` |
| Background `threading.Thread(target=stream_reader)` | `bufio.Scanner` on main goroutine |
| Shared `stop_signal` list | `context.Context.Done()` |
| `time.time() - start_t > timeout` polling | `context.WithTimeout` |
| `smart_format(..., max_str_len=10000)` | `stdoutBudget` + `Truncated` |
| `code_run_header.py` written to a temp file | (skipped — too specific to GenericAgent's sys.path tricks) |

Note that upstream **polls for timeout** (`sleep(1)` in a loop), while we use
`context.WithTimeout` for kernel-precise triggers. Go does this strictly
better than Python here.

Next: [s06-fileops](s06-fileops.md) — `file_read/write/patch`, the three
tools that let an agent edit code.
