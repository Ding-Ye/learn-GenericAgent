package main

import (
	"context"
	"errors"
	"fmt"
)

// fakeProvider is a tiny stand-in we ship in main.go so `go run .` works
// without a real API key. The tests have richer fakes.
type fakeProvider struct {
	name string
	fail bool
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Chat(_ context.Context, _ []Message, _ []ToolSpec, chunks chan<- string) (Response, error) {
	if f.fail {
		return Response{}, errors.New("rate limit exceeded")
	}
	chunks <- "[" + f.name + "] hello"
	return Response{Content: "[" + f.name + "] hello"}, nil
}

func main() {
	// Primary fails; fallback works → mixin should spring to fallback.
	primary := &fakeProvider{name: "primary", fail: true}
	fallback := &fakeProvider{name: "fallback", fail: false}
	mix := NewMixinProvider(2000, primary, fallback)

	chunks := make(chan string, 32)
	go func() {
		for c := range chunks {
			fmt.Print(c)
		}
	}()
	resp, err := mix.Chat(context.Background(), nil, nil, chunks)
	close(chunks)

	if err != nil {
		fmt.Println("\n[err]", err)
		return
	}
	fmt.Printf("\n[final] %s\n", resp.Content)
}
