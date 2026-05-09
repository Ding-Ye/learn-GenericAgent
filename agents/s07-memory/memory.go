package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory implements GenericAgent's L0..L4 layered memory model on top of a
// disk directory. Layers:
//
//   L0  hardcoded behavior rules         → embedded in code (sysPromptBase)
//   L1  routing index / insight          → memory/global_mem_insight.txt
//   L2  stable accumulated knowledge     → memory/global_mem.txt
//   L3  per-topic SOP markdown           → memory/<topic>_sop.md
//   L4  archived session records         → memory/sessions/<ts>.txt
//
// Plus a working checkpoint that persists across turns (lives only in-process
// in this learn version; upstream optionally persists to temp/<task>/).
//
// Upstream parallel:
//   memory/global_mem.txt, memory/global_mem_insight.txt, memory/<topic>_sop.md,
//   temp/model_responses/*.txt, ga.py:do_update_working_checkpoint
type Memory struct {
	dir          string
	mu           sync.RWMutex
	checkpoint   string
	checkpointTS time.Time
}

const sysPromptBase = `You are an agent. Use tools when needed. Be brief.`

func NewMemory(dir string) (*Memory, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		return nil, err
	}
	for _, f := range []string{"global_mem.txt", "global_mem_insight.txt"} {
		p := filepath.Join(dir, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			seed := "# " + strings.ReplaceAll(strings.TrimSuffix(f, ".txt"), "_", " ") + "\n"
			if err := os.WriteFile(p, []byte(seed), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return &Memory{dir: dir}, nil
}

// L0SystemPrompt returns the hardcoded base. Constant per build.
func (m *Memory) L0SystemPrompt() string { return sysPromptBase }

// L1Insight reads the routing index. Loaded on every turn.
func (m *Memory) L1Insight() (string, error) {
	b, err := os.ReadFile(filepath.Join(m.dir, "global_mem_insight.txt"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// L2Global reads the stable accumulated knowledge. Loaded on every turn.
func (m *Memory) L2Global() (string, error) {
	b, err := os.ReadFile(filepath.Join(m.dir, "global_mem.txt"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// L3LoadSOP returns the contents of memory/<topic>_sop.md, or an error if missing.
func (m *Memory) L3LoadSOP(topic string) (string, error) {
	b, err := os.ReadFile(filepath.Join(m.dir, topic+"_sop.md"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// L3ListSOPs lists available skill names (filenames ending in _sop.md).
func (m *Memory) L3ListSOPs() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, "_sop.md") {
			continue
		}
		out = append(out, strings.TrimSuffix(n, "_sop.md"))
	}
	sort.Strings(out)
	return out, nil
}

// L4AppendSession archives session content with a timestamped filename.
// Mirrors upstream's temp/model_responses/<microsec>.txt.
func (m *Memory) L4AppendSession(content string) error {
	name := fmt.Sprintf("sessions/%d.txt", time.Now().UnixMicro())
	return os.WriteFile(filepath.Join(m.dir, name), []byte(content), 0o644)
}

// SetCheckpoint stores a snapshot of "the agent's mid-task scratchpad."
// Upstream: do_update_working_checkpoint replaces handler.working['key_info'].
func (m *Memory) SetCheckpoint(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoint = s
	m.checkpointTS = time.Now()
}

// Checkpoint returns the current working checkpoint.
func (m *Memory) Checkpoint() (string, time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkpoint, m.checkpointTS
}

// AssembleSystemPrompt builds the full system prompt for one turn:
//
//   <L0>
//   <L1: insight>
//   <L2: global memory>
//   <working checkpoint, if any>
//
// Mirrors agentmain.py:get_system_prompt + the working-memory injection in
// turn_end_callback.
func (m *Memory) AssembleSystemPrompt() string {
	var b strings.Builder
	b.WriteString(m.L0SystemPrompt())
	b.WriteString("\n\n")
	if s, err := m.L1Insight(); err == nil && strings.TrimSpace(s) != "" {
		b.WriteString("[L1 Insight]\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	if s, err := m.L2Global(); err == nil && strings.TrimSpace(s) != "" {
		b.WriteString("[L2 Global Memory]\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	if cp, ts := m.Checkpoint(); cp != "" {
		fmt.Fprintf(&b, "[Working checkpoint, set %s ago]\n%s\n", time.Since(ts).Truncate(time.Second), cp)
	}
	return b.String()
}

// UpdateGlobalMem appends to L2.
func (m *Memory) UpdateGlobalMem(line string) error {
	p := filepath.Join(m.dir, "global_mem.txt")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	_, err = f.WriteString(line)
	return err
}
