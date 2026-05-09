package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	dir := flag.String("dir", "./memdir", "memory directory")
	op := flag.String("op", "show", "show | append | checkpoint")
	value := flag.String("value", "", "value for append/checkpoint")
	flag.Parse()

	mem, err := NewMemory(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch *op {
	case "show":
		fmt.Println(mem.AssembleSystemPrompt())
	case "append":
		if err := mem.UpdateGlobalMem(*value); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("[ok] appended to global_mem.txt")
	case "checkpoint":
		mem.SetCheckpoint(*value)
		fmt.Println("[ok] checkpoint set; rerun with -op show to see it surfaced")
	default:
		fmt.Fprintln(os.Stderr, "unknown op")
		os.Exit(2)
	}
}
