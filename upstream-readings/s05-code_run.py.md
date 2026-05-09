# Upstream reading for s05 — `code_run`

[`ga.py:code_run`](https://github.com/lsdefine/GenericAgent/blob/main/ga.py) is the most important tool in upstream and worth reading end-to-end.

Search the file for `def code_run(` (around line 13). It's ~100 lines.

## Anatomy

1. **Lines 1-15** — yield a `[Action] Running ...` preview line; pick the cmd:
   - python → `[sys.executable, "-X", "utf8", "-u", tmp_path]` with a header
     prepended that imports `os, sys, json, ...` so the agent's code can use
     them without imports.
   - bash/powershell → 1-liner via `-c`.

2. **Lines 16-40** — set `STARTUPINFO` for Windows (hide console), spawn
   `subprocess.Popen` with merged stderr, set up a stream reader thread.

3. **Lines 41-60** — the polling loop:
   ```python
   while t.is_alive():
       istimeout = time.time() - start_t > timeout
       if istimeout or stop_signal:
           process.kill()
           full_stdout.append("\n[Timeout Error] ..." if istimeout else "\n[Stopped] ...")
           break
       time.sleep(1)
   ```
   This is a 1-second poll, not a waitpid. The Go `context.WithTimeout`
   version is strictly better latency-wise.

4. **Lines 61-80** — output formatting:
   ```python
   output_snippet = smart_format(stdout_str, max_str_len=600, omit_str='\n[omitted]\n')
   output_snippet = re.sub(r'`{4,}', lambda m: m.group(0)[:3] + '​' + m.group(0)[3:], output_snippet)
   yield f"[Status] {status_icon} Exit Code: {exit_code}\n[Stdout]\n{output_snippet}\n"
   ```
   That regex inserts a zero-width space (U+200B) into any run of 4+ backticks
   so they stop closing the agent's display markdown. Our `compactLine`
   function does the same.

5. **Lines 81-100** — return the dict:
   ```python
   return {
       "status": status,
       "stdout": smart_format(stdout_str, max_str_len=10000),
       "exit_code": exit_code,
   }
   ```
   Note: the displayed stdout (`max_str_len=600`) is shorter than the stored
   stdout (`max_str_len=10000`). Our Go version conflates these for
   simplicity; consider splitting them as an exercise.

## Why upstream uses a temp file for Python

```python
tmp_file = tempfile.NamedTemporaryFile(suffix=".ai.py", delete=False, ...)
cr_header = os.path.join(script_dir, 'assets', 'code_run_header.py')
if os.path.exists(cr_header):
    tmp_file.write(open(cr_header, ...).read())
tmp_file.write(code)
```

Two reasons:
1. The header (`code_run_header.py`) pre-imports common modules so the agent
   can write `os.listdir(...)` without `import os`. Saves one round-trip per
   tool call.
2. Long code via `-c` runs into ARG_MAX on some systems.

We skip both in the Go version — too specific to GenericAgent's UX.

## What's omitted on purpose

- `stop_signal` list as a thread-shared kill switch — Go's `context.Context`
  replaces this primitive cleanly.
- `STARTUPINFO` for Windows console hiding — out of scope for a Linux/macOS
  learn version.
- `code_cwd` separate from `cwd` — micro-optimization for cleanup.

## Read these line ranges

- `ga.py:def code_run` — entire 100-line function (search for the def)
- `ga.py:def smart_format` — the truncation helper (search the file)
