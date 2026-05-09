package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	codeType := flag.String("type", "bash", "python | bash")
	code := flag.String("code", "echo hello && date", "code to run")
	timeoutSec := flag.Int("timeout", 30, "timeout seconds")
	flag.Parse()

	chunks := make(chan string, 64)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()
	res, err := CodeRun(context.Background(), *code, CodeType(*codeType),
		time.Duration(*timeoutSec)*time.Second, chunks)
	close(chunks)

	if err != nil {
		fmt.Fprintln(os.Stderr, "[err]", err)
		os.Exit(1)
	}
	fmt.Printf("\n[result] status=%s exit=%d truncated=%v len(stdout)=%d\n",
		res.Status, res.ExitCode, res.Truncated, len(res.Stdout))
}
