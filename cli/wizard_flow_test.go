package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

// driveWizardUpdate feeds a key to a wizard model and returns the result.
func driveWizardUpdate(t *testing.T, m wizardModel, key tea.KeyType) wizardModel {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: key})
	return next.(wizardModel)
}

func TestWizardOverwriteGate(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	// pre-existing folder with user content
	if err := os.MkdirAll("collide", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("collide", "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := wizardModel{
		theme:        currentTheme(),
		help:         help.New(),
		chosen:       map[int]bool{},
		preConfirmed: "",
		name:         "collide",
		step:         4,
		maxStep:      4,
		desc:         "d",
	}
	m.chosen[0] = true // 'bar' selected

	// confirming surfaces must NOT scaffold; it must open the overwrite step
	gated := driveWizardUpdate(t, m, tea.KeyEnter)
	if gated.step != 7 {
		t.Fatalf("step = %d, want 7 (overwrite confirm)", gated.step)
	}
	if len(gated.created) != 0 {
		t.Fatalf("scaffold ran before confirmation: %v", gated.created)
	}
	if _, err := os.Stat(filepath.Join("collide", "manifest.json")); !os.IsNotExist(err) {
		t.Fatal("files were written before the overwrite was accepted")
	}

	// No -> cancelled outro, nothing written
	no := driveWizardUpdate(t, gated, tea.KeyEnter)
	if no.step != 6 || !no.aborted {
		t.Fatalf("decline should cancel: step=%d aborted=%v", no.step, no.aborted)
	}
	if strings.Contains(no.View(), "You're all set!") {
		t.Fatal("cancelled outro must not claim success")
	}
	if rec := recoveryCommand(no); rec == "" || !strings.Contains(no.View(), rec) {
		t.Fatal("declined cancel view missing recovery command")
	}
	if _, err := os.Stat(filepath.Join("collide", "keep.txt")); err != nil {
		t.Fatal("existing file lost on decline")
	}

	// Yes -> scaffolds into it and shows the success outro
	yes := driveWizardUpdate(t, gated, tea.KeyRight) // move to Yes
	yes = driveWizardUpdate(t, yes, tea.KeyEnter)
	if yes.step != 5 || yes.done != true {
		t.Fatalf("accept should reach outro: step=%d done=%v", yes.step, yes.done)
	}
	if len(yes.created) == 0 {
		t.Fatal("accept did not scaffold")
	}
	if _, err := os.Stat(filepath.Join("collide", "keep.txt")); err != nil {
		t.Fatal("overwrite removed pre-existing unrelated file")
	}
	if _, err := os.Stat(filepath.Join("collide", "manifest.json")); err != nil {
		t.Fatal("scaffold missing after accept")
	}
}

func TestWizardPreConfirmedSkipsGate(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("mine", 0o755); err != nil {
		t.Fatal(err)
	}

	m := wizardModel{
		theme:        currentTheme(),
		help:         help.New(),
		chosen:       map[int]bool{},
		preConfirmed: "mine",
		name:         "mine",
		step:         4,
		maxStep:      4,
	}
	m.chosen[2] = true // panel

	next := driveWizardUpdate(t, m, tea.KeyEnter)
	if next.step != 5 {
		t.Fatalf("pre-confirmed name should scaffold straight through, got step %d", next.step)
	}
	if len(next.created) == 0 {
		t.Fatal("expected scaffold output")
	}
}

// The panel-mode picker (step 8): arrows move the cursor, enter commits the
// mode and scaffolds.
func TestWizardPanelModeStep(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	newModel := func(cursor int) wizardModel {
		return wizardModel{
			theme:       currentTheme(),
			help:        help.New(),
			chosen:      map[int]bool{3: true}, // panel
			name:        "app",
			desc:        "d",
			step:        8,
			maxStep:     4,
			panelCursor: cursor,
		}
	}

	// arrows move the selection
	m := newModel(0)
	m = driveWizardUpdate(t, m, tea.KeyDown)
	if m.panelCursor != 1 {
		t.Fatalf("panelCursor = %d, want 1", m.panelCursor)
	}
	m = driveWizardUpdate(t, m, tea.KeyDown)
	if m.panelCursor != 2 {
		t.Fatalf("panelCursor = %d, want 2", m.panelCursor)
	}
	m = driveWizardUpdate(t, m, tea.KeyUp)
	if m.panelCursor != 1 {
		t.Fatalf("panelCursor = %d, want 1", m.panelCursor)
	}

	// enter commits the selected mode and scaffolds
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.step != 5 || !m.done {
		t.Fatalf("enter did not finish: step=%d done=%v", m.step, m.done)
	}
	if m.panelMode != "window" {
		t.Fatalf("panelMode = %q, want window", m.panelMode)
	}
	if len(m.created) == 0 {
		t.Fatal("no files created")
	}

	// a fresh run picking "both" persists that mode
	m = newModel(2)
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.panelMode != "both" {
		t.Fatalf("panelMode = %q, want both", m.panelMode)
	}
}

// Text steps validate: empty name is rejected with a reason, invalid values
// show the validator message, and a valid name advances to the next gap.
func TestWizardTextStepValidation(t *testing.T) {
	newModel := func(step int) wizardModel {
		return wizardModel{
			theme:   currentTheme(),
			help:    help.New(),
			chosen:  map[int]bool{},
			step:    step,
			maxStep: step,
			input:   newWizardInput(step, ""),
		}
	}

	// empty name stays on the step with a reason
	m := newModel(0)
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.step != 0 || !strings.Contains(m.err, "required") {
		t.Fatalf("empty name not rejected: step=%d err=%q", m.step, m.err)
	}

	// invalid name shows the validator error and stays
	m = newModel(0)
	m.input.SetValue("Bad Name!")
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.step != 0 || !strings.Contains(m.err, "lowercase") {
		t.Fatalf("invalid name not rejected: step=%d err=%q", m.step, m.err)
	}

	// valid name advances to the description gap (step 1)
	m = newModel(0)
	m.input.SetValue("my-plugin")
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.step != 1 {
		t.Fatalf("step = %d, want 1", m.step)
	}

	// invalid author shows the validator error
	m = newModel(3)
	m.input.SetValue("Bad Author!")
	m = driveWizardUpdate(t, m, tea.KeyEnter)
	if m.step != 3 || !strings.Contains(m.err, "lowercase") {
		t.Fatalf("invalid author not rejected: step=%d err=%q", m.step, m.err)
	}
}

// Quit keys: esc aborts from a text step, "q" aborts from the surface list.
func TestWizardQuitKeys(t *testing.T) {
	m := wizardModel{
		theme:   currentTheme(),
		help:    help.New(),
		chosen:  map[int]bool{},
		step:    0,
		maxStep: 0,
		input:   newWizardInput(0, ""),
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(wizardModel)
	if !nm.aborted || nm.step != 6 {
		t.Fatalf("esc from text step: aborted=%v step=%d", nm.aborted, nm.step)
	}

	m = wizardModel{
		theme:   currentTheme(),
		help:    help.New(),
		chosen:  map[int]bool{1: true},
		step:    4,
		maxStep: 4,
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	nm = next.(wizardModel)
	if !nm.aborted || nm.step != 6 {
		t.Fatalf("q from surface list: aborted=%v step=%d", nm.aborted, nm.step)
	}
}
