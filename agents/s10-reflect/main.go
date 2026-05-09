package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// printAgent is a stub agent that just prints the task. Replace with a real
// loop in s_full.
type printAgent struct{ count atomic.Int32 }

func (p *printAgent) Run(_ context.Context, task string) (string, error) {
	c := p.count.Add(1)
	fmt.Printf("[agent #%d] received: %s\n", c, task)
	return strings.ToUpper(task), nil
}

func main() {
	cfgPath := flag.String("config", "/tmp/reflect.json", "JSON config to poll for tasks")
	intervalSec := flag.Int("interval", 2, "polling interval seconds")
	once := flag.Bool("once", false, "exit after one task")
	flag.Parse()

	loop := NewReflectLoop(
		JSONCheck(*cfgPath),
		&printAgent{},
		time.Duration(*intervalSec)*time.Second,
	)
	if *once {
		loop.Once()
	}
	loop.OnDone = func(task, result string, err error) {
		if err != nil {
			fmt.Printf("[done] task=%q err=%v\n", task, err)
			return
		}
		fmt.Printf("[done] task=%q result=%q\n", task, result)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("[reflect] polling %s every %ds; write JSON like {\"task\":\"hello\"}\n", *cfgPath, *intervalSec)
	if err := loop.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
