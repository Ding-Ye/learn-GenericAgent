package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill is one entry from the L3 SOP directory.
type Skill struct {
	Name    string // "tmwebdriver"
	Path    string // "memory/tmwebdriver_sop.md"
	Title   string // first H1 of the file, fallback to Name
	Summary string // first non-empty paragraph after the H1
}

// SkillTree scans an L3 directory once and indexes its SOP files.
//
// Upstream parallel:
//   memory/skill_search/SKILL.md is the index file (the agent reads it via
//   file_read, then issues a follow-up tool call to fetch the chosen skill).
//   We capture the same flow with two methods: List() returns the index and
//   Load(name) returns one skill body.
type SkillTree struct {
	dir    string
	skills []Skill
}

func NewSkillTree(dir string) (*SkillTree, error) {
	t := &SkillTree{dir: dir}
	if err := t.Scan(); err != nil {
		return nil, err
	}
	return t, nil
}

// Scan re-reads the directory; call after a new SOP is written.
func (t *SkillTree) Scan() error {
	entries, err := os.ReadDir(t.dir)
	if err != nil {
		return err
	}
	t.skills = nil
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_sop.md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), "_sop.md")
		path := filepath.Join(t.dir, e.Name())
		title, summary, err := readHead(path)
		if err != nil {
			return err
		}
		if title == "" {
			title = name
		}
		t.skills = append(t.skills, Skill{Name: name, Path: path, Title: title, Summary: summary})
	}
	sort.Slice(t.skills, func(i, j int) bool { return t.skills[i].Name < t.skills[j].Name })
	return nil
}

// List returns the index (one line per skill) suitable for embedding into
// the system prompt or returning from the `skill_search` tool.
func (t *SkillTree) List() string {
	var b strings.Builder
	b.WriteString("# Skill index\n\n")
	for _, s := range t.skills {
		fmt.Fprintf(&b, "- **%s** — %s\n", s.Name, s.Summary)
	}
	return b.String()
}

// Search returns skills whose name/title/summary contain the query
// (case-insensitive substring). Mirrors `skill_search` upstream — naive but
// sufficient for a learn version.
func (t *SkillTree) Search(query string) []Skill {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Skill, len(t.skills))
		copy(out, t.skills)
		return out
	}
	var out []Skill
	for _, s := range t.skills {
		hay := strings.ToLower(s.Name + " " + s.Title + " " + s.Summary)
		if strings.Contains(hay, q) {
			out = append(out, s)
		}
	}
	return out
}

// Load returns the full body of one skill, ready for prompt injection.
func (t *SkillTree) Load(name string) (string, error) {
	for _, s := range t.skills {
		if s.Name == name {
			b, err := os.ReadFile(s.Path)
			return string(b), err
		}
	}
	return "", fmt.Errorf("skill not found: %q", name)
}

// readHead returns (title, summary) — the first H1 and the first non-empty
// paragraph after it. Tolerates files without an H1 (returns empty title and
// the first non-empty line as summary).
func readHead(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var title, summary string
	sawTitle := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !sawTitle {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				sawTitle = true
				continue
			}
			if line == "" {
				continue
			}
			// File has no H1 — first non-empty line becomes summary.
			return "", line, nil
		}
		if line == "" {
			continue
		}
		summary = line
		break
	}
	return title, summary, scanner.Err()
}
