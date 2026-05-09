package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	op := flag.String("op", "read", "read | write | append | prepend | patch")
	path := flag.String("path", "", "target file path")
	start := flag.Int("start", 1, "(read) start line, 1-indexed")
	count := flag.Int("count", 200, "(read) line count")
	keyword := flag.String("keyword", "", "(read) search keyword instead of range")
	content := flag.String("content", "", "(write/append/prepend) content")
	oldStr := flag.String("old", "", "(patch) old block")
	newStr := flag.String("new", "", "(patch) new block")
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "-path required")
		os.Exit(2)
	}

	switch *op {
	case "read":
		out, err := FileRead(*path, *start, *count, *keyword, true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(out)
	case "write":
		n, err := FileWrite(*path, *content, ModeWrite)
		check(err)
		fmt.Printf("[wrote] %d bytes\n", n)
	case "append":
		n, err := FileWrite(*path, *content, ModeAppend)
		check(err)
		fmt.Printf("[appended] %d bytes\n", n)
	case "prepend":
		n, err := FileWrite(*path, *content, ModePrepend)
		check(err)
		fmt.Printf("[prepended] %d bytes\n", n)
	case "patch":
		check(FilePatch(*path, *oldStr, *newStr))
		fmt.Println("[patched]")
	default:
		fmt.Fprintln(os.Stderr, "unknown op:", *op)
		os.Exit(2)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
