package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpFile(t *testing.T, body string) string {
	t.Helper()
	d := t.TempDir()
	p := filepath.Join(d, "f.txt")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileRead_Range(t *testing.T) {
	p := tmpFile(t, "a\nb\nc\nd\ne\n")
	out, err := FileRead(p, 2, 2, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != "2: b\n3: c" {
		t.Fatalf("got: %q", got)
	}
}

func TestFileRead_Keyword(t *testing.T) {
	body := "alpha\nbravo\ncharlie\ndelta\necho\nfoxtrot\n"
	p := tmpFile(t, body)
	out, err := FileRead(p, 0, 0, "delta", true)
	if err != nil {
		t.Fatal(err)
	}
	// 5-line context window around delta (line 4) → lines 2..6
	for _, want := range []string{"2: bravo", "3: charlie", "4: delta", "5: echo", "6: foxtrot"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestFileRead_NoLineNos(t *testing.T) {
	p := tmpFile(t, "x\ny\n")
	out, err := FileRead(p, 1, 2, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ":") {
		t.Fatalf("unexpected colon: %q", out)
	}
}

func TestFileWrite_AllModes(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "w.txt")

	if _, err := FileWrite(p, "hello", ModeWrite); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "hello" {
		t.Fatalf("after write: %q", string(got))
	}

	if _, err := FileWrite(p, " world", ModeAppend); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "hello world" {
		t.Fatalf("after append: %q", string(got))
	}

	if _, err := FileWrite(p, "BEGIN ", ModePrepend); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "BEGIN hello world" {
		t.Fatalf("after prepend: %q", string(got))
	}
}

func TestFilePatch_HappyPath(t *testing.T) {
	p := tmpFile(t, "package x\n\nfunc Foo() { print(\"old\") }\n")
	err := FilePatch(p, `print("old")`, `print("new")`)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), `print("new")`) {
		t.Fatalf("patch did not apply: %s", string(got))
	}
	if strings.Contains(string(got), `print("old")`) {
		t.Fatalf("old still present: %s", string(got))
	}
}

func TestFilePatch_AmbiguousMatchFails(t *testing.T) {
	p := tmpFile(t, "x = 1\nx = 1\nx = 1\n")
	err := FilePatch(p, "x = 1", "x = 2")
	if err == nil {
		t.Fatal("expected ambiguous-match error")
	}
	if !strings.Contains(err.Error(), "matches 3") {
		t.Fatalf("error wording regression: %v", err)
	}
}

func TestFilePatch_NotFound(t *testing.T) {
	p := tmpFile(t, "abc\n")
	err := FilePatch(p, "missing", "something")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestFileWrite_CreatesDirs(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a/b/c/file.txt")
	if _, err := FileWrite(p, "x", ModeWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}
