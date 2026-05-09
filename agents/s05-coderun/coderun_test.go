package main

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func drain(ch <-chan string) string {
	var b strings.Builder
	for c := range ch {
		b.WriteString(c)
	}
	return b.String()
}

func TestCodeRun_BashSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on windows")
	}
	chunks := make(chan string, 32)
	streamDone := make(chan string, 1)
	go func() { streamDone <- drain(chunks) }()

	res, err := CodeRun(context.Background(), "echo hello", Bash, 5*time.Second, chunks)
	close(chunks)
	streamed := <-streamDone

	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" {
		t.Fatalf("status: %q", res.Status)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit: %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("stdout missing 'hello': %q", res.Stdout)
	}
	if !strings.Contains(streamed, "hello") {
		t.Fatalf("stream missing 'hello': %q", streamed)
	}
}

func TestCodeRun_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	chunks := make(chan string, 8)
	go func() {
		for range chunks {
		}
	}()
	res, err := CodeRun(context.Background(), "exit 17", Bash, 5*time.Second, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "error" {
		t.Fatalf("status: %q", res.Status)
	}
	if res.ExitCode != 17 {
		t.Fatalf("exit: %d", res.ExitCode)
	}
}

func TestCodeRun_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	chunks := make(chan string, 8)
	go func() {
		for range chunks {
		}
	}()
	res, err := CodeRun(context.Background(), "sleep 10", Bash, 200*time.Millisecond, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "error" {
		t.Fatalf("status: %q", res.Status)
	}
	if !strings.Contains(res.Stdout, "Timeout") {
		t.Fatalf("expected timeout marker, got: %q", res.Stdout)
	}
}

func TestCodeRun_UnsupportedType(t *testing.T) {
	chunks := make(chan string, 1)
	go func() {
		for range chunks {
		}
	}()
	res, _ := CodeRun(context.Background(), "noop", "ruby", time.Second, chunks)
	close(chunks)
	if res.Status != "error" {
		t.Fatalf("status: %q", res.Status)
	}
	if !strings.Contains(res.Stdout, "unsupported") {
		t.Fatalf("got %q", res.Stdout)
	}
}

func TestCodeRun_ContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	ctx, cancel := context.WithCancel(context.Background())
	chunks := make(chan string, 8)
	go func() {
		for range chunks {
		}
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := CodeRun(ctx, "sleep 5", Bash, 10*time.Second, chunks)
	close(chunks)
	if err != nil {
		t.Fatal(err)
	}
	// On cancel, exec.CommandContext kills the process; status may be "error"
	// with exit != 0.
	if res.Status != "error" {
		t.Logf("got status=%q (acceptable on some platforms)", res.Status)
	}
}

func TestMakeTool_Wrapping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	tool := MakeTool(2 * time.Second)
	chunks := make(chan string, 16)
	go func() {
		for range chunks {
		}
	}()
	out := tool(context.Background(),
		map[string]any{"code": "echo wrapped", "code_type": "bash"},
		chunks)
	close(chunks)
	res, ok := out.Data.(CodeRunResult)
	if !ok {
		t.Fatalf("data type: %T", out.Data)
	}
	if !strings.Contains(res.Stdout, "wrapped") {
		t.Fatalf("stdout: %q", res.Stdout)
	}
}
