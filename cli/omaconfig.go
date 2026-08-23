package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// omaConfig is the opt-in project configuration (oma.json, next to
// manifest.json). Configuration is data, not code: the Go CLI reads it today
// and omarchy itself can consume it natively later without running anything.
type omaConfig struct {
	Icon      string             `json:"icon,omitempty"`
	Panel     *panelConfig       `json:"panel,omitempty"`
	Launchers []launcherEntryDef `json:"launchers,omitempty"`
}

type panelConfig struct {
	Mode string `json:"mode,omitempty"` // attached | window | both
}

type launcherEntryDef struct {
	Name        string   `json:"name"`                  // required
	Action      string   `json:"action,omitempty"`      // summon | toggle | hide (default summon)
	Exec        string   `json:"exec,omitempty"`        // full Exec= override
	Icon        string   `json:"icon,omitempty"`        // overrides the global icon
	GenericName string   `json:"genericName,omitempty"` // default: kind label
	Comment     string   `json:"comment,omitempty"`     // default: manifest description
	Terminal    bool     `json:"terminal,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// defaultIcon is used when neither the entry nor oma.json sets an icon.
const defaultIcon = "application-x-executable"

// loadOMAConfig reads dir/oma.json. A missing file is a valid empty config —
// absent launchers[] means nothing is ever created.
func loadOMAConfig(dir string) (*omaConfig, error) {
	path := filepath.Join(filepath.Clean(dir), "oma.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &omaConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var c omaConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if c.Panel != nil {
		switch c.Panel.Mode {
		case "", "attached", "window", "both":
		default:
			return nil, fmt.Errorf("%s: panel.mode must be attached, window or both (got %q)", filepath.Base(path), c.Panel.Mode)
		}
	}
	for i, d := range c.Launchers {
		if strings.TrimSpace(d.Name) == "" && strings.TrimSpace(d.Exec) == "" {
			return nil, fmt.Errorf("%s: launchers[%d] needs a \"name\"", filepath.Base(path), i)
		}
		switch d.Action {
		case "", "summon", "toggle", "hide":
		default:
			return nil, fmt.Errorf("%s: launchers[%d] has unknown action %q (summon, toggle or hide)", filepath.Base(path), i, d.Action)
		}
	}
	return &c, nil
}

func (c *omaConfig) panelMode() string {
	if c.Panel != nil && c.Panel.Mode != "" {
		return c.Panel.Mode
	}
	return "attached"
}

// effectiveIcon resolves the icon for one entry: entry-level override, then
// the global icon, then the built-in default. Pass-through string today
// (freedesktop name or path); richer formats are a planned extension.
func (c *omaConfig) effectiveIcon(def launcherEntryDef) string {
	if def.Icon != "" {
		return def.Icon
	}
	if c.Icon != "" {
		return c.Icon
	}
	return defaultIcon
}
