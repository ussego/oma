package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

func seedLaunchers(t *testing.T) (project string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	project = filepath.Join(home, "proj")
	if _, err := scaffoldWithOptions(project, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"Alpha"}, {"Beta", "--action", "toggle"}} {
		if err := runLauncherAdd(project, args); err != nil {
			t.Fatal(err)
		}
	}
	return project
}

func readLauncherNames(t *testing.T, project string) []string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	var raw struct {
		Launchers []launcherEntryDef `json:"launchers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range raw.Launchers {
		names = append(names, l.Name)
	}
	return names
}

func TestLauncherRemoveByName(t *testing.T) {
	project := seedLaunchers(t)
	if err := runLauncherRemove(project, []string{"Alpha"}); err != nil {
		t.Fatal(err)
	}
	names := readLauncherNames(t, project)
	if len(names) != 1 || names[0] != "Beta" {
		t.Fatalf("names = %v", names)
	}
	// survivors' .desktop files are regenerated; removed one is gone
	apps := filepath.Join(home0(t), ".local", "share", "applications")
	if _, err := os.Stat(filepath.Join(apps, "tester.proj-alpha.desktop")); !os.IsNotExist(err) {
		t.Fatal("alpha desktop still present")
	}
	body, _ := os.ReadFile(filepath.Join(apps, "tester.proj.desktop"))
	if len(body) == 0 {
		t.Fatal("beta desktop missing")
	}

	// unknown name errors without touching config
	before, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	if err := runLauncherRemove(project, []string{"Nope"}); err == nil {
		t.Fatal("expected unknown-name error")
	}
	after, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	if string(before) != string(after) {
		t.Fatal("unknown name mutated oma.json")
	}
}

// home0 returns the (fake) HOME set by seedLaunchers via t.Setenv.
func home0(t *testing.T) string {
	t.Helper()
	h, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestLauncherRemoveAllDropsKey(t *testing.T) {
	project := seedLaunchers(t)
	if err := runLauncherRemove(project, []string{"--all"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(project, "oma.json"))
	if strings.Contains(string(data), `"launchers"`) {
		t.Fatalf("empty launchers key should be dropped:\n%s", data)
	}
	apps := filepath.Join(home0(t), ".local", "share", "applications")
	matches, _ := filepath.Glob(filepath.Join(apps, "tester.proj*"))
	if len(matches) != 0 {
		t.Fatalf("managed desktop files remain: %v", matches)
	}
}

func TestLauncherRemoveModel(t *testing.T) {
	m := launcherRemoveModel{
		options: []string{"Alpha", "Beta", "Gamma"},
		chosen:  map[int]bool{},
		help:    help.New(),
		theme:   currentTheme(),
	}
	step := func(m launcherRemoveModel, msg tea.Msg) (launcherRemoveModel, tea.Cmd) {
		next, cmd := m.Update(msg)
		return next.(launcherRemoveModel), cmd
	}

	// enter with nothing selected stays put
	if _, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("enter without selection should not finish")
	}

	// move down, toggle Beta, move back up
	var cmd tea.Cmd
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeySpace})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter with selection should finish")
	}
	if !m.chosen[1] || m.chosen[0] || m.chosen[2] {
		t.Fatalf("chosen = %v, want only Beta", m.chosen)
	}

	// view lists every option
	view := stripCodes(m.View())
	for _, opt := range []string{"Alpha", "Beta", "Gamma"} {
		if !strings.Contains(view, opt) {
			t.Fatalf("view missing %q:\n%s", opt, view)
		}
	}

	// q aborts
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.aborted {
		t.Fatal("q should abort")
	}
}
