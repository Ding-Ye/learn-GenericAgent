package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileRead implements `do_file_read` from upstream.
//
// Two modes:
//   1. Range mode:  start..start+count-1 lines (1-indexed)
//   2. Keyword mode: lines containing keyword, with 5-line context window
//
// `showLineNos` prefixes each line with "<n>: " (matches the upstream output
// shape that the model has learned to consume).
//
// Upstream: ga.py:file_read (search for `def file_read`)
func FileRead(path string, start, count int, keyword string, showLineNos bool) (string, error) {
	abs, err := absPath(path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var keep []int
	if keyword != "" {
		hits := map[int]struct{}{}
		for i, ln := range lines {
			if strings.Contains(ln, keyword) {
				for k := i - 2; k <= i+2; k++ { // 5-line context
					if k >= 0 && k < len(lines) {
						hits[k] = struct{}{}
					}
				}
			}
		}
		for i := 0; i < len(lines); i++ {
			if _, ok := hits[i]; ok {
				keep = append(keep, i)
			}
		}
	} else {
		if start < 1 {
			start = 1
		}
		if count <= 0 {
			count = 200
		}
		end := start + count - 1
		for i := start - 1; i < len(lines) && i < end; i++ {
			keep = append(keep, i)
		}
	}

	var out strings.Builder
	prev := -2
	for _, i := range keep {
		if prev >= 0 && i-prev > 1 {
			out.WriteString("...\n")
		}
		if showLineNos {
			fmt.Fprintf(&out, "%d: %s\n", i+1, lines[i])
		} else {
			out.WriteString(lines[i])
			out.WriteByte('\n')
		}
		prev = i
	}
	return out.String(), nil
}

// FileWrite implements `do_file_write` with three modes: write, append, prepend.
type WriteMode string

const (
	ModeWrite   WriteMode = "write"
	ModeAppend  WriteMode = "append"
	ModePrepend WriteMode = "prepend"
)

func FileWrite(path string, content string, mode WriteMode) (int, error) {
	abs, err := absPath(path)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return 0, err
	}
	switch mode {
	case ModeWrite, "":
		err = os.WriteFile(abs, []byte(content), 0o644)
		return len(content), err
	case ModeAppend:
		f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		n, err := f.WriteString(content)
		return n, err
	case ModePrepend:
		old, err := os.ReadFile(abs)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		joined := content + string(old)
		err = os.WriteFile(abs, []byte(joined), 0o644)
		return len(content), err
	}
	return 0, fmt.Errorf("unknown mode: %q", mode)
}

// FilePatch finds exactly one occurrence of `old` in the file and replaces it
// with `new`. Mirrors upstream `file_patch` with its strict-uniqueness rule —
// the most useful invariant: ambiguous matches fail loudly so the LLM rewrites
// its diff with more context.
func FilePatch(path string, oldContent, newContent string) error {
	abs, err := absPath(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	s := string(data)
	count := strings.Count(s, oldContent)
	switch count {
	case 0:
		return fmt.Errorf("file_patch: old block not found in %s", path)
	case 1:
		updated := strings.Replace(s, oldContent, newContent, 1)
		return os.WriteFile(abs, []byte(updated), 0o644)
	default:
		return fmt.Errorf("file_patch: old block matches %d places in %s; supply more context lines", count, path)
	}
}

// absPath converts a path to absolute. We don't sandbox here — a real agent
// harness must — but we always work against a resolved path for tests.
func absPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

// reading helpers used by tests
func mustReadAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
