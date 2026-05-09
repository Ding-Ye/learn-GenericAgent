package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	dir := flag.String("dir", "./mem", "directory containing *_sop.md files")
	op := flag.String("op", "list", "list | search | load")
	q := flag.String("q", "", "query for search; name for load")
	flag.Parse()

	tree, err := NewSkillTree(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch *op {
	case "list":
		fmt.Print(tree.List())
	case "search":
		hits := tree.Search(*q)
		if len(hits) == 0 {
			fmt.Println("(no matches)")
			return
		}
		for _, s := range hits {
			fmt.Printf("- %s — %s\n", s.Name, s.Summary)
		}
	case "load":
		body, err := tree.Load(*q)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(body)
	default:
		fmt.Fprintln(os.Stderr, "unknown op")
		os.Exit(2)
	}
}
