# s05 — 流式代码执行工具

## Problem

LLM agent 最常用的工具是"执行代码"。GenericAgent 上游的 `code_run` 是它最强的工具——
能跑任意 Python / bash，能流式回传 stdout，能被用户中断。

要做对这个工具，必须解决 4 个问题：

1. **流式输出**——子进程跑 30 秒，用户不能干等 30 秒，要看到每一行 stdout 实时打印。
2. **超时**——脚本死循环你不能让它无限跑。
3. **取消**——用户按 Ctrl-C 要能立刻杀掉子进程。
4. **输出预算**——LLM 的 context 有限，10 MB 的 stdout 不能整个塞回模型。

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

`exec.CommandContext` 把 4 个问题中的 3 个一起搞定：传一个 `context.WithTimeout`
进去，超时和取消都走同一个 ctx；进程退出时它自动 SIGKILL。

剩下一个问题是输出预算：`bufio.Scanner` 边扫边累积，越过 `stdoutBudget` 就停止累积
但继续 push chunks（让用户能看完），最后在结果上标 `Truncated: true`。

## How It Works

核心循环 30 行：

```go
runCtx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
cmd := exec.CommandContext(runCtx, cmdName, cmdArgs...)
cmd.Stderr = cmd.Stdout    // 合并 stderr 到 stdout（上游 ga.py 同样做法）
stdoutPipe, _ := cmd.StdoutPipe()
cmd.Start()

scanner := bufio.NewScanner(stdoutPipe)
scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)  // 默认 64K 太小，扩到 1MB
for scanner.Scan() {
    line := scanner.Text() + "\n"
    chunks <- line                    // 流式
    if buf.Len()+len(line) <= 16*1024 {
        buf.WriteString(line)
    } else {
        truncated = true
    }
}
cmd.Wait()
```

四个细节值得注意：

1. **`cmd.Stderr = cmd.Stdout`**——把 stderr 重定向到 stdout，让我们一根 pipe 拿全部输出。
   不然得开两个 goroutine 分别读，加复杂度。
2. **scanner.Buffer 扩容**——`bufio.Scanner` 默认 64K，遇到一行 100K 直接报 `bufio.ErrTooLong`。
   扩到 1MB 兜底。
3. **`compactLine` 转义 4 反引号**——LLM 的 markdown 输出经常被自己的 stdout 干扰。
   遇到 ```` 就插入零宽字符断开，避免出问题（上游 `ga.py:code_run` 第 70 行附近做同样的事）。
4. **超时分支独立返回**——超时时 `cmd.Wait` 也会返回非零 exit，但我们要让 LLM 知道
   "是超时不是脚本本身错"，所以判断 `runCtx.Err() == DeadlineExceeded` 后单独返回。

## What Changed

s05 是一个**独立工具模块**——不引入新的 loop/types 概念。它要被装到 s_full 的 registry 上才有用。
所以 diff 主要是新增：

| 文件 | 内容 |
|------|------|
| `coderun.go` | `CodeRun()` 函数 + `MakeTool()` 适配器 |
| `main.go` | CLI: `go run . -type bash -code "..."` |
| `coderun_test.go` | 6 个测试：bash/non-zero/timeout/unsupported/cancel/MakeTool |

`StepOutcome` 类型在本模块再次出现（独立的小副本）——s_full 才会把它和前几节统一。

## Try It

```bash
cd agents/s05-coderun
go run . -type bash -code "for i in 1 2 3; do echo \$i; sleep 0.2; done"
```

输出：

```
1
2
3

[result] status=success exit=0 truncated=false len(stdout)=6
```

实验：跑一个 `sleep 10`，用 `-timeout 1` 看到超时分支。

```bash
go run . -type bash -code "sleep 10" -timeout 1
```

## Upstream Source Reading

上游 `ga.py:code_run` 是 100 行的函数。完整流程几乎一一对应：

```python
# ga.py:code_run（精简）
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
            print(line, end="")    ← 流式
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

精确的对应关系：

| 上游 | 我们的 Go |
|------|-----------|
| `subprocess.Popen(cmd, stdout=PIPE, stderr=STDOUT)` | `exec.CommandContext` + `cmd.Stderr = cmd.Stdout` |
| 后台 `threading.Thread(target=stream_reader)` | `bufio.Scanner` 在主 goroutine 读 |
| `stop_signal` 列表共享 | `context.Context` 的 `<-ctx.Done()` |
| `time.time() - start_t > timeout` 轮询 | `context.WithTimeout` |
| `smart_format(..., max_str_len=10000)` | `stdoutBudget` + `Truncated` |
| `code_run_header.py` 写入临时文件 | （我们没做，太特定于 GenericAgent 的 sys.path 黑魔法） |

注意上游是**轮询超时**（每秒 sleep 1 检查），我们用 `context.WithTimeout` 内核级精确触发。
Go 这一档表现优于 Python。

下一节 [s06-fileops](s06-fileops.md)：`file_read/write/patch` 三个工具——agent 编辑代码的基础。
