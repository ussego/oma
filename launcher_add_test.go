package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// `oma launcher add` must upsert into oma.json (creating it when missing,
// replacing same-name entries) and materialize the .desktop entry.
func TestLauncherAddUpsert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	project := filepath.Join(home, "proj")
	if _, err := scaffoldWithOptions(project, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}

	// add from scratch — oma.json has no launchers[] yet
	if err := runLauncherAdd(project, []string{"Play Next", "--action", "toggle", "--icon", "media-playback-start"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	var cfg struct {
		Schema    string             `json:"$schema"`
		Launchers []launcherEntryDef `json:"launchers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("oma.json invalid: %v\n%s", err, data)
	}
	if len(cfg.Launchers) != 1 || cfg.Launchers[0].Name != "Play Next" || cfg.Launchers[0].Action != "toggle" {
		t.Fatalf("launchers = %+v", cfg.Launchers)
	}
	if cfg.Schema == "" {
		t.Fatal("created oma.json is missing $schema")
	}
	desktop := filepath.Join(home, ".local", "share", "applications", "tester.proj.desktop")
	body, _ := os.ReadFile(desktop)
	if !strings.Contains(string(body), "omarchy-shell shell toggle tester.proj") {
		t.Fatalf("desktop entry not materialized:\n%s", body)
	}

	// same name replaces in place instead of duplicating
	if err := runLauncherAdd(project, []string{"Play Next", "--action", "summon"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(project, "oma.json"))
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Launchers) != 1 || cfg.Launchers[0].Action != "summon" {
		t.Fatalf("upsert did not replace: %+v", cfg.Launchers)
	}

	// a differently-named second entry appends
	if err := runLauncherAdd(project, []string{"Quit"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(project, "oma.json"))
	json.Unmarshal(data, &cfg)
	if len(cfg.Launchers) != 2 || cfg.Launchers[1].Name != "Quit" || cfg.Launchers[1].Action != "summon" {
		t.Fatalf("append failed: %+v", cfg.Launchers)
	}

	// invalid action rejected before touching oma.json
	before, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	if err := runLauncherAdd(project, []string{"Bad", "--action", "explode"}); err == nil {
		t.Fatal("expected error for invalid action")
	}
	after, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	if string(before) != string(after) {
		t.Fatal("invalid action mutated oma.json")
	}
}

// --exec without --action persists exec only: the schema says exec overrides
// action, so no stale action key is written alongside it.
func TestLauncherAddExecOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	project := filepath.Join(home, "proj")
	if _, err := scaffoldWithOptions(project, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}

	if err := runLauncherAdd(project, []string{"Custom", "--exec", "my-cmd --flag"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	var cfg struct {
		Launchers []launcherEntryDef `json:"launchers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Launchers) != 1 {
		t.Fatalf("launchers = %+v", cfg.Launchers)
	}
	l := cfg.Launchers[0]
	if l.Exec != "my-cmd --flag" || l.Action != "" {
		t.Fatalf("expected exec-only entry, got %+v", l)
	}
	desktop := filepath.Join(home, ".local", "share", "applications", "tester.proj.desktop")
	body, _ := os.ReadFile(desktop)
	if !strings.Contains(string(body), "Exec=my-cmd --flag\n") {
		t.Fatalf("desktop Exec not overridden:\n%s", body)
	}
}

// --- headless drives of the add wizard model ---

func pressKey(t *testing.T, m launcherWizardModel, msg tea.Msg) launcherWizardModel {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(launcherWizardModel)
}

func typeRunes(m launcherWizardModel, s string) launcherWizardModel {
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(launcherWizardModel)
	}
	return m
}

// answerName commits the name prompt and lands on the command step.
func answerName(t *testing.T, m launcherWizardModel, name string) launcherWizardModel {
	t.Helper()
	m = typeRunes(m, name)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepCommand {
		t.Fatalf("expected command step after name, got step %d (err=%q)", m.step, m.err)
	}
	return m
}

// Regression: 'q' used to match the quit binding and abort mid-typing.
func TestLauncherWizardAcceptsQInName(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		defaultAction: "summon",
	})
	m = typeRunes(m, "Quit Next")
	if m.aborted {
		t.Fatal("typing 'q' aborted the wizard")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // commit name
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // summon preset
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // blank icon -> finish
	if !m.done || m.aborted {
		t.Fatalf("flow should finish: done=%v aborted=%v", m.done, m.aborted)
	}
	def := m.result()
	if def.Name != "Quit Next" || def.Action != "summon" {
		t.Fatalf("result = %+v", def)
	}
}

// Sequential reveal: before answering, later prompts are not on screen.
func TestLauncherWizardRevealsOnePromptAtATime(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		defaultAction: "summon",
	})
	view := stripCodes(m.View())
	if strings.Contains(view, "command:") || strings.Contains(view, "icon:") {
		t.Fatalf("later prompts leaked onto the first screen:\n%s", view)
	}
	m = answerName(t, m, "Play")
	view = stripCodes(m.View())
	if !strings.Contains(view, "name: Play") {
		t.Fatalf("answered name not frozen into history:\n%s", view)
	}
	if strings.Contains(view, "icon:") {
		t.Fatalf("icon prompt shown before its turn:\n%s", view)
	}
}

// Summon on a plugin without panel/overlay warns live and saves as toggle.
func TestLauncherWizardInvalidSummonWarnsAndFallsBack(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.clock",
		kinds:         []string{"bar-widget"},
		defaultAction: "summon",
	})
	m = answerName(t, m, "Clock")
	if view := stripCodes(m.View()); !strings.Contains(view, "saved as toggle") {
		t.Fatalf("missing fallback warning:\n%s", view)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // summon preset (falls back)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // blank icon -> finish
	def := m.result()
	if def.Action != "toggle" {
		t.Fatalf("summon must fall back to toggle, got %q", def.Action)
	}
}

// The custom… row opens a command input; a filled one saves exec-only.
func TestLauncherWizardCustomCommand(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		defaultAction: "summon",
	})
	m = answerName(t, m, "Open")
	for i := 0; i < 3; i++ { // walk down to custom…
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // open input
	if !m.editing {
		t.Fatal("enter on custom… should open the command input")
	}
	m = typeRunes(m, "my-cmd --flag")
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // save command
	if m.step != stepIcon {
		t.Fatalf("expected icon step after custom command, got %d", m.step)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // blank icon -> finish
	if !m.done {
		t.Fatal("flow should finish at the icon step")
	}
	def := m.result()
	if def.Exec != "my-cmd --flag" || def.Action != "" {
		t.Fatalf("custom must save exec-only, got %+v", def)
	}
}

// Enter on an empty custom input returns to the list; esc then aborts.
func TestLauncherWizardEmptyCustomBouncesBack(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		defaultAction: "summon",
	})
	m = answerName(t, m, "Open")
	for i := 0; i < 3; i++ {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.editing {
		t.Fatal("expected editing state")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // empty -> back to list
	if m.editing || m.done {
		t.Fatalf("empty custom must bounce back: editing=%v done=%v", m.editing, m.done)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if !m.aborted {
		t.Fatal("esc outside editing aborts")
	}
}

// A name matching a declared entry announces the update instead of silently
// replacing; the icon prefill from oma.json shows up on its turn.
func TestLauncherWizardReplaceNoteAndPrefill(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		existingNames: []string{"Play Next"},
		prefillIcon:   "media-playback-start",
		defaultAction: "summon",
	})
	m = answerName(t, m, "Play Next")
	view := stripCodes(m.View())
	if !strings.Contains(view, "name: Play Next") {
		t.Fatalf("name not frozen:\n%s", view)
	}
	if !strings.Contains(view, "updates existing entry") {
		t.Fatalf("missing replace note:\n%s", view)
	}
	if !strings.Contains(view, "omarchy-shell shell summon usse.music") {
		t.Fatalf("preset commands not shown:\n%s", view)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // keep summon
	if view := stripCodes(m.View()); !strings.Contains(view, "media-playback-start") {
		t.Fatalf("icon prefill missing:\n%s", view)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // keep icon prefill
	def := m.result()
	if def.Name != "Play Next" || def.Icon != "media-playback-start" {
		t.Fatalf("answers lost: %+v", def)
	}
}

// The name prompt starts empty — no manifest-derived default.
func TestLauncherWizardNoNameDefault(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		defaultAction: "summon",
	})
	if v := m.ti.Value(); v != "" {
		t.Fatalf("name input should start empty, got %q", v)
	}
}

// A seeded --exec starts the flow's selection on the custom… row.
func TestLauncherWizardPrefillCustomSeedsEditor(t *testing.T) {
	m := newTestWizard(launcherWizardInput{
		id:            "usse.music",
		kinds:         []string{"panel"},
		prefillCustom: "my-cmd --flag",
		defaultAction: "summon",
	})
	m = answerName(t, m, "Open")
	if m.command != launcherCustomRow {
		t.Fatalf("custom row should be preselected, got %d", m.command)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // open editor (seeded)
	if view := stripCodes(m.View()); !strings.Contains(view, "my-cmd --flag") {
		t.Fatalf("editor not seeded:\n%s", view)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // save command
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // blank icon -> finish
	def := m.result()
	if def.Exec != "my-cmd --flag" || def.Action != "" {
		t.Fatalf("seeded custom lost: %+v", def)
	}
}

// Esc cancels from the initial screen.
func TestLauncherWizardEscAborts(t *testing.T) {
	m := newTestWizard(launcherWizardInput{kinds: []string{"panel"}, defaultAction: "summon"})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if !m.aborted || m.done {
		t.Fatalf("esc should abort: aborted=%v done=%v", m.aborted, m.done)
	}
}

// Enter with an empty name refuses to advance and says why.
func TestLauncherWizardNameRequired(t *testing.T) {
	m := newTestWizard(launcherWizardInput{kinds: []string{"panel"}, defaultAction: "summon"})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepName || m.done {
		t.Fatalf("empty name must not advance: step=%d done=%v", m.step, m.done)
	}
	if view := stripCodes(m.View()); !strings.Contains(view, "name is required") {
		t.Fatalf("missing hint:\n%s", view)
	}
}

func newTestWizard(in launcherWizardInput) launcherWizardModel {
	return newLauncherWizardModel(in)
}

// stripCodes removes escape sequences (SGR colors and CSI edits like
// \x1b[2K) so assertions see plain text.
func stripCodes(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			if i+1 < len(s) && s[i+1] == '[' {
				// CSI: ESC [ params... final (0x40-0x7E)
				i += 2
				for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e) {
					i++
				}
				if i < len(s) {
					i++
				}
				continue
			}
			i++ // lone escape: drop and keep scanning
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
