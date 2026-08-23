package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
)

// The npx-facing discovery stub (skills/oma/SKILL.md) is the ecosystem entry
// point: it must carry the frontmatter the skills CLI requires and route only
// to skills that actually exist in skill-data.
func TestSkillStubRoutesToRealSkills(t *testing.T) {
	stub, err := os.ReadFile(filepath.Join("skills", "oma", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(stub)

	meta := parseFrontmatter(s)
	if meta.name != "oma" {
		t.Fatalf("stub name = %q, want oma", meta.name)
	}
	if meta.description == "" {
		t.Fatal("stub missing description (agents can't discover it)")
	}

	// every skill-data skill must be reachable through `oma skills get <name>`
	entries, err := os.ReadDir(filepath.Join("assets", "skill-data"))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		seen++
		if !strings.Contains(s, "oma skills get "+e.Name()) {
			t.Errorf("stub does not route to skill %q", e.Name())
		}
	}
	if seen < 6 {
		t.Fatalf("only %d skills in skill-data", seen)
	}
}

// The create wizard outro advertises the agent-skill install command.
func TestWizardOutroHasSkillsHint(t *testing.T) {
	m := wizardModel{
		theme:   currentTheme(),
		help:    help.New(),
		chosen:  map[int]bool{},
		name:    "x",
		step:    5,
		done:    true,
		created: []string{"manifest.json"},
	}
	view := stripCodes(m.View())
	if !strings.Contains(view, "npx skills add ussego/oma") {
		t.Fatalf("wizard outro missing skills hint:\n%s", view)
	}
	if !strings.Contains(view, "agent skills for this project") {
		t.Fatalf("wizard outro hint unclear:\n%s", view)
	}
}

// The non-interactive create prints the same hint after the file list.
func TestCreatePrintsSkillsHint(t *testing.T) {
	out := stripCodes(captureStdout(t, printSkillsHint))
	if !strings.Contains(out, "npx skills add ussego/oma") {
		t.Fatalf("non-interactive hint missing: %q", out)
	}
}
