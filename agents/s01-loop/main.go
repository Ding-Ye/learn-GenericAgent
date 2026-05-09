package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	user := flag.String("user", "Hello, agent!", "user message to send")
	maxTurns := flag.Int("max-turns", 5, "loop turn cap")
	flag.Parse()

	provider := &MockProvider{
		Replies: []string{
			"Hi! I'm a mock agent. You said: " + *user + ". (s01 has no tools, so I'm done.)",
		},
	}

	chunks := make(chan string, 32)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()

	exit, err := Run(context.Background(), provider,
		"You are a helpful agent. (system prompt placeholder for s01)",
		*user, *maxTurns, chunks)
	close(chunks)

	if err != nil {
		fmt.Fprintln(os.Stderr, "\n[error]", err)
		os.Exit(1)
	}
	fmt.Printf("\n\n[exit] reason=%s turns=%d\n", exit.Reason, exit.Turns)
}
