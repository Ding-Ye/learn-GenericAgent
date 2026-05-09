package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// StepOutcome — same shape as s03/s04. Re-declared per session.
type StepOutcome struct {
	Data       any
	NextPrompt string
	ShouldExit bool
}

// CodeRunResult is what `code_run` puts into outcome.Data.
//
// Upstream parallel: ga.py:code_run returns
//   {"status": "success"|"error", "stdout": "...", "exit_code": N}
// We add `truncated` for honesty about long outputs.
type CodeRunResult struct {
	Status    string `json:"status"`
	Stdout    string `json:"stdout"`
	ExitCode  int    `json:"exit_code"`
	Truncated bool   `json:"truncated,omitempty"`
}

// CodeRun spawns a child process, streams its stdout into `chunks`, and
// returns a CodeRunResult. The two interesting controls:
//
//   - `timeout` is enforced via context.WithTimeout; on expiry we
//     SIGKILL the process and report a [Timeout Error] line.
//   - cancellation via ctx (the loop already wraps Run in a ctx; we
//     forward it).
//
// We support two CodeType values:
//   - "python":  cmd = python3 -c <code>
//   - "bash":    cmd = bash -c <code>
//
// In s_full we'll mount this on the registry as the `code_run` tool.
type CodeType string

const (
	Python CodeType = "python"
	Bash   CodeType = "bash"
)

const stdoutBudget = 16 * 1024 // bytes; mirrors upstream's smart_format max_str_len=10000

func CodeRun(ctx context.Context, code string, t CodeType, timeout time.Duration, chunks chan<- string) (CodeRunResult, error) {
	var cmdName string
	var cmdArgs []string
	switch t {
	case Python:
		cmdName, cmdArgs = "python3", []string{"-c", code}
	case Bash:
		cmdName, cmdArgs = "bash", []string{"-c", code}
	default:
		return CodeRunResult{Status: "error", Stdout: "unsupported code_type: " + string(t)}, nil
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, cmdName, cmdArgs...)
	cmd.Stderr = nil // merged into stdout below
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return CodeRunResult{}, err
	}
	cmd.Stderr = cmd.Stdout // merge

	if err := cmd.Start(); err != nil {
		return CodeRunResult{}, err
	}

	// Stream stdout line-by-line. Push every line to `chunks` and accumulate
	// into a buffer (capped at stdoutBudget; mark truncated if we drop bytes).
	var (
		buf       strings.Builder
		truncated bool
		preview   string
	)
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		preview = compactLine(line)
		select {
		case chunks <- preview:
		default:
		}
		if buf.Len()+len(line) <= stdoutBudget {
			buf.WriteString(line)
		} else {
			truncated = true
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = cmd.Process.Kill()
	}

	werr := cmd.Wait()
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}

	res := CodeRunResult{Stdout: buf.String(), ExitCode: exit, Truncated: truncated}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.Stdout += "\n[Timeout Error] killed after " + timeout.String()
		res.Status = "error"
		select {
		case chunks <- res.Stdout:
		default:
		}
		return res, nil
	}
	if werr != nil && exit != 0 {
		res.Status = "error"
		return res, nil
	}
	res.Status = "success"
	return res, nil
}

// compactLine prevents 4-or-more backticks from ever closing a code fence
// upstream prints inside; matches the trick at ga.py:code_run line ~70.
func compactLine(s string) string {
	if !strings.Contains(s, "````") {
		return s
	}
	return strings.ReplaceAll(s, "````", "```​`")
}

// MakeTool returns a ToolFunc usable in the s_full registry. We export it
// here so this package is consumable from s_full without code duplication.
func MakeTool(timeout time.Duration) func(ctx context.Context, args map[string]any, chunks chan<- string) StepOutcome {
	return func(ctx context.Context, args map[string]any, chunks chan<- string) StepOutcome {
		code, _ := args["code"].(string)
		ct, _ := args["code_type"].(string)
		if ct == "" {
			ct = "python"
		}
		select {
		case chunks <- fmt.Sprintf("[code_run] %s (%d chars)\n", ct, len(code)):
		default:
		}
		res, err := CodeRun(ctx, code, CodeType(ct), timeout, chunks)
		if err != nil {
			return StepOutcome{Data: map[string]any{"status": "error", "msg": err.Error()}}
		}
		return StepOutcome{Data: res}
	}
}
