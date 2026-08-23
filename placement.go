package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// shellConfigPath returns the running shell's config file - the same file
// oma install enables plugins through.
func shellConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "omarchy", "shell.json")
}

// stuckBarWidget reports whether a bar-widget plugin id sits in plugins[]
// without any bar.layout entry. That state is unmigratable by the shell
// itself: setEnabled skips the widget branch when the id is already known
// (findEntryLocation finds it in plugins[]), and putBarWidget's moveBarEntry
// error is silently discarded, so both report success while placing nothing.
func stuckBarWidget(shellJSON []byte, id string) bool {
	var cfg struct {
		Plugins []json.RawMessage `json:"plugins"`
		Bar     struct {
			Layout map[string][]json.RawMessage `json:"layout"`
		} `json:"bar"`
	}
	if err := json.Unmarshal(shellJSON, &cfg); err != nil {
		return false
	}
	inPlugins := false
	for _, raw := range cfg.Plugins {
		if entryID(raw) == id {
			inPlugins = true
			break
		}
	}
	if !inPlugins {
		return false
	}
	for _, entries := range cfg.Bar.Layout {
		for _, raw := range entries {
			if entryID(raw) == id {
				return false
			}
		}
	}
	return true
}

// entryID extracts the id from a shell.json placement entry. The shell stores
// entries as {"id": ...} objects, but barEntryId also tolerates bare strings,
// so read both shapes.
func entryID(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.ID
	}
	return ""
}

// placeBarWidget migrates a bar-widget stuck in plugins[] into bar.layout,
// using only shell IPC: disabling splices the entry out wherever it lives,
// then re-enabling takes the fresh-enable path and places the widget at its
// manifest default section (the shell's own default is "center"). It is a
// no-op unless the stuck state is detected, so installs stay idempotent.
func placeBarWidget(m manifest) error {
	data, err := os.ReadFile(shellConfigPath())
	if err != nil || !stuckBarWidget(data, m.ID) {
		return nil // not stuck (or the shell isn't running - enable already warned)
	}
	section := "center"
	if m.BarWidget != nil {
		switch m.BarWidget.DefaultSection {
		case "left", "center", "right":
			section = m.BarWidget.DefaultSection
		}
	}
	disable := exec.Command("omarchy-shell", "shell", "setEnabled", m.ID, "false")
	if out, err := disable.CombinedOutput(); err != nil {
		return fmt.Errorf("setEnabled %s false: %v: %s", m.ID, err, strings.TrimSpace(string(out)))
	}
	enable := exec.Command("omarchy-shell", "shell", "enablePlugin", m.ID, fmt.Sprintf(`{"section":%q}`, section))
	if out, err := enable.CombinedOutput(); err != nil {
		return fmt.Errorf("enablePlugin %s: %v: %s", m.ID, err, strings.TrimSpace(string(out)))
	}
	okLine("placed " + m.ID + " in bar.layout." + section)
	return nil
}
