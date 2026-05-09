package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newMem(t *testing.T) *Memory {
	t.Helper()
	m, err := NewMemory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNewMemory_SeedsFiles(t *testing.T) {
	d := t.TempDir()
	if _, err := NewMemory(d); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"global_mem.txt", "global_mem_insight.txt", "sessions"} {
		if _, err := os.Stat(filepath.Join(d, want)); err != nil {
			t.Fatalf("missing %s: %v", want, err)
		}
	}
}

func TestUpdateGlobalMem_Appends(t *testing.T) {
	m := newMem(t)
	if err := m.UpdateGlobalMem("first fact"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateGlobalMem("second fact"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.L2Global()
	if !strings.Contains(got, "first fact") || !strings.Contains(got, "second fact") {
		t.Fatalf("missing facts: %q", got)
	}
}

func TestCheckpoint_Roundtrip(t *testing.T) {
	m := newMem(t)
	m.SetCheckpoint("doing X step 3")
	cp, ts := m.Checkpoint()
	if cp != "doing X step 3" {
		t.Fatalf("got: %q", cp)
	}
	if ts.IsZero() {
		t.Fatal("ts not set")
	}
}

func TestAssembleSystemPrompt_LayersAppear(t *testing.T) {
	m := newMem(t)
	if err := m.UpdateGlobalMem("rule: never delete files"); err != nil {
		t.Fatal(err)
	}
	m.SetCheckpoint("currently in step 2/5")

	out := m.AssembleSystemPrompt()
	for _, want := range []string{
		"You are an agent",                  // L0
		"L2 Global Memory",                  // L2 marker
		"rule: never delete files",          // L2 content
		"Working checkpoint",                // working
		"currently in step 2/5",             // working content
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestL3LoadSOP_FoundAndMissing(t *testing.T) {
	m := newMem(t)
	d := m.dir
	if err := os.WriteFile(filepath.Join(d, "browser_sop.md"), []byte("# Browser SOP"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := m.L3LoadSOP("browser")
	if err != nil {
		t.Fatal(err)
	}
	if got != "# Browser SOP" {
		t.Fatalf("got: %q", got)
	}

	if _, err := m.L3LoadSOP("nonexistent"); err == nil {
		t.Fatal("expected not-found")
	}
}

func TestL3ListSOPs(t *testing.T) {
	m := newMem(t)
	d := m.dir
	for _, n := range []string{"alpha_sop.md", "beta_sop.md", "not_a_sop.txt"} {
		os.WriteFile(filepath.Join(d, n), []byte("x"), 0o644)
	}
	list, err := m.L3ListSOPs()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0] != "alpha" || list[1] != "beta" {
		t.Fatalf("got %v", list)
	}
}

func TestL4AppendSession(t *testing.T) {
	m := newMem(t)
	if err := m.L4AppendSession("session content"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(m.dir, "sessions"))
	if len(entries) != 1 {
		t.Fatalf("want 1 file, got %d", len(entries))
	}
}
