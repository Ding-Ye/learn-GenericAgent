package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupSkillsDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	files := map[string]string{
		"browser_sop.md": "# Browser SOP\n\nUse this when you need to drive a real Chrome via CDP.\n\nDetails follow.\n",
		"plan_sop.md":    "# Planning SOP\n\nWrite a plan file before any multi-step task.\n",
		"verify_sop.md":  "# Verify SOP\n\nAfter changes, run tests and check exit codes.\n",
		"unrelated.txt":  "not a skill",
	}
	for n, body := range files {
		if err := os.WriteFile(filepath.Join(d, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestSkillTree_Scan_PicksOnlySOP(t *testing.T) {
	tree, err := NewSkillTree(setupSkillsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.skills) != 3 {
		t.Fatalf("want 3 skills, got %d", len(tree.skills))
	}
}

func TestSkillTree_List_AlphabeticalAndContainsSummaries(t *testing.T) {
	tree, _ := NewSkillTree(setupSkillsDir(t))
	out := tree.List()
	pos := func(s string) int { return strings.Index(out, s) }
	if pos("browser") > pos("plan") {
		t.Fatalf("alphabetical order broken")
	}
	if !strings.Contains(out, "drive a real Chrome") {
		t.Fatalf("summary missing in list: %q", out)
	}
}

func TestSkillTree_Search_CaseInsensitiveSubstring(t *testing.T) {
	tree, _ := NewSkillTree(setupSkillsDir(t))
	hits := tree.Search("CHROME")
	if len(hits) != 1 || hits[0].Name != "browser" {
		t.Fatalf("got %+v", hits)
	}
	hits = tree.Search("test")
	if len(hits) != 1 || hits[0].Name != "verify" {
		t.Fatalf("got %+v", hits)
	}
	all := tree.Search("")
	if len(all) != 3 {
		t.Fatalf("empty query should return all, got %d", len(all))
	}
}

func TestSkillTree_Load_FoundAndMissing(t *testing.T) {
	tree, _ := NewSkillTree(setupSkillsDir(t))
	body, err := tree.Load("browser")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Use this when") {
		t.Fatalf("body content lost: %q", body)
	}
	if _, err := tree.Load("ghost"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestSkillTree_Rescan(t *testing.T) {
	d := setupSkillsDir(t)
	tree, _ := NewSkillTree(d)
	os.WriteFile(filepath.Join(d, "new_sop.md"), []byte("# New\n\nFresh skill.\n"), 0o644)
	if err := tree.Scan(); err != nil {
		t.Fatal(err)
	}
	if len(tree.skills) != 4 {
		t.Fatalf("rescan didn't pick up new skill, got %d", len(tree.skills))
	}
}

func TestReadHead_NoTitle(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "no_title_sop.md")
	os.WriteFile(p, []byte("just a note\nmore text\n"), 0o644)
	tree, _ := NewSkillTree(d)
	if len(tree.skills) != 1 {
		t.Fatal("scan failed")
	}
	if tree.skills[0].Title != "no_title" {
		t.Fatalf("expected fallback title, got %q", tree.skills[0].Title)
	}
}
