package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runSurfaceAdd adds surfaces to an existing project: updates manifest.json
// (kinds + entryPoints) and generates ui/<Kind>.qml skeletons for kinds whose
// file doesn't exist yet. Idempotent - already-present kinds are skipped.
func runSurfaceAdd(dir string, args []string) error {
	path := filepath.Join(filepath.Clean(dir), "manifest.json")
	m, err := readManifest(path)
	if err != nil {
		return err
	}

	// extract --panel-mode flag before parsing kinds
	panelMode := ""
	filtered := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--panel-mode" || a == "-p" {
			if i+1 >= len(args) {
				return fmt.Errorf("flag %s needs a value (attached, window or both)", a)
			}
			panelMode = strings.ToLower(strings.TrimSpace(args[i+1]))
			switch panelMode {
			case "attached", "window", "both":
			default:
				return fmt.Errorf("unknown --panel-mode %q (attached, window or both)", args[i+1])
			}
			i++
			continue
		}
		if strings.HasPrefix(a, "--panel-mode=") {
			panelMode = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(a, "--panel-mode=")))
			switch panelMode {
			case "attached", "window", "both":
			default:
				return fmt.Errorf("unknown --panel-mode %q (attached, window or both)", panelMode)
			}
			continue
		}
		filtered = append(filtered, a)
	}
	// fall back to oma.json panel.mode, default attached
	if panelMode == "" {
		if cfg, err := loadOMAConfig(dir); err == nil && cfg.Panel != nil && cfg.Panel.Mode != "" {
			panelMode = cfg.Panel.Mode
		} else {
			panelMode = "attached"
		}
	}

	requested := parseKinds(strings.Join(filtered, ","))
	for _, k := range requested {
		if !isSurface(k) {
			return fmt.Errorf("unknown kind %q (choose from %s)", k, strings.Join(surfaces, ", "))
		}
	}

	existing := map[string]bool{}
	for _, k := range m.Kinds {
		existing[k] = true
	}

	// detect existing panel (any mode) to enforce single mode (force one but not both)
	hasExistingPanel := false
	oldPanelMode := ""
	if _, err := os.Stat(filepath.Join(dir, "ui/Panel.qml")); err == nil {
		hasExistingPanel = true
	}
	if existing["panel"] {
		hasExistingPanel = true
	}
	if hasExistingPanel {
		if existing["panel"] && existing["bar-widget"] {
			oldPanelMode = "both"
		} else if existing["panel"] {
			oldPanelMode = "window"
		} else {
			oldPanelMode = "attached"
		}
		if cfg, err := loadOMAConfig(dir); err == nil && cfg.Panel != nil && cfg.Panel.Mode != "" {
			oldPanelMode = cfg.Panel.Mode
		}
	}

	var missing []string
	interactivePickedPanel := false
	if len(requested) == 0 {
		// interactive: offer every kind the project doesn't have yet
		for _, s := range surfaces {
			if !existing[s] {
				missing = append(missing, s)
			}
		}
		if len(missing) == 0 {
			noteLine("all surfaces already present")
			return nil
		}
		picked, aborted := promptSurfaceAdd(missing)
		if aborted || len(picked) == 0 {
			return nil
		}
		missing = picked
		if contains(missing, "panel") {
			interactivePickedPanel = true
		}
	} else {
		var add []string
		for _, k := range requested {
			if k == "panel" && hasExistingPanel {
				// panel already present in some mode — treat as mode switch if mode differs
				if panelMode != oldPanelMode {
					add = append(add, k)
					continue
				}
				noteLine("already present: " + k)
				continue
			}
			if existing[k] {
				noteLine("already present: " + k)
				continue
			}
			add = append(add, k)
		}
		if len(add) == 0 {
			return nil
		}
		missing = add
	}
	// if panel was picked interactively and no explicit --panel-mode flag was passed, prompt for mode
	flagPassed := false
	for _, a := range args {
		if a == "--panel-mode" || a == "-p" || strings.HasPrefix(a, "--panel-mode=") {
			flagPassed = true
			break
		}
	}
	if interactivePickedPanel && !flagPassed {
		mode, aborted := promptPanelMode(panelMode)
		if aborted {
			return nil
		}
		panelMode = mode
	}

	name := capitalize(m.Name)
	if name == "" {
		name = capitalize(filepath.Base(filepath.Clean(dir)))
	}
	bridge := bridgeBaseName(name, m.Kinds)
	icon := "\uf013"
	if cfg, err := loadOMAConfig(dir); err == nil && cfg.Icon != "" {
		icon = cfg.Icon
	}

	var added []string
	for _, k := range missing {
		added = append(added, k)
	}
	// panel ships as a bar-widget button + anchored popup pair (mode-dependent)
	hasNewPanel := contains(missing, "panel")
	isPanelSwitch := hasExistingPanel && hasNewPanel && panelMode != oldPanelMode
	if isPanelSwitch {
		// force one mode, not both: rebuild kinds exclusively from non-panel base + new panel
		var baseKinds []string
		for _, k := range m.Kinds {
			if k == "panel" {
				continue
			}
			if k == "bar-widget" && (oldPanelMode == "attached" || oldPanelMode == "both") {
				continue
			}
			baseKinds = append(baseKinds, k)
		}
		m.Kinds = manifestKindsFor(append(baseKinds, "panel"), panelMode)
	} else {
		m.Kinds = manifestKindsFor(append(m.Kinds, missing...), panelMode)
	}
	if m.EntryPoints == nil {
		m.EntryPoints = map[string]string{}
	}
	if isPanelSwitch {
		// rebuild entryPoints for panel modes exclusively
		delete(m.EntryPoints, "panel")
		delete(m.EntryPoints, "barWidget")
		for _, k := range m.Kinds {
			ep := entryPointKey[k]
			if k == "panel" && qmlName[k] == "" {
				continue
			}
			m.EntryPoints[ep] = "ui/" + qmlName[k]
		}
		if panelMode == "both" {
			m.EntryPoints["panel"] = "ui/PanelWindow.qml"
		}
	} else {
		for _, k := range m.Kinds {
			ep := entryPointKey[k]
			if _, exists := m.EntryPoints[ep]; !exists {
				if k == "panel" && qmlName[k] == "" {
					continue
				}
				if k == "panel" && hasNewPanel && panelMode == "both" {
					// handled below
					continue
				}
				m.EntryPoints[ep] = "ui/" + qmlName[k]
			}
		}
		if hasNewPanel && panelMode == "both" {
			m.EntryPoints["barWidget"] = "ui/BarWidget.qml"
			m.EntryPoints["panel"] = "ui/PanelWindow.qml"
		} else if hasNewPanel && panelMode == "window" {
			// ensure panel entry; barWidget may be absent
			if _, ok := m.EntryPoints["panel"]; !ok {
				m.EntryPoints["panel"] = "ui/Panel.qml"
			}
		}
	}

	for _, k := range missing {
		var files []genFile
		if k == "panel" {
			files = panelSurfaceFiles(panelMode, m.ID, bridge, icon)
		} else {
			files = surfaceFiles(k, m.ID, bridge, icon)
		}
		for _, f := range files {
			qmlPath := filepath.Join(dir, filepath.FromSlash(f.rel))
			if isPanelSwitch && k == "panel" {
				// force one mode: overwrite Panel.qml (attached vs floating) directly
				if err := writeFile(qmlPath, f.content); err != nil {
					return err
				}
				continue
			}
			if _, err := os.Stat(qmlPath); os.IsNotExist(err) {
				if err := writeFile(qmlPath, f.content); err != nil {
					return err
				}
			}
		}
	}
	if isPanelSwitch {
		// clean up obsolete file for exclusive mode
		if panelMode == "attached" {
			_ = os.Remove(filepath.Join(dir, "ui/PanelWindow.qml"))
		} else if panelMode == "window" {
			_ = os.Remove(filepath.Join(dir, "ui/PanelWindow.qml"))
			// window mode orphans the attached host; drop it if it's still the generated one
			if oldPanelMode == "attached" {
				if data, err := os.ReadFile(filepath.Join(dir, "ui/BarWidget.qml")); err == nil {
					if strings.Contains(string(data), "panelLoader") {
						_ = os.Remove(filepath.Join(dir, "ui/BarWidget.qml"))
					}
				}
			}
		}
	}
	// persist panel mode in oma.json when a panel was added
	if hasNewPanel {
		if err := persistPanelMode(dir, panelMode); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update oma.json panel mode: %v\n", err)
		}
	}

	if err := writeJSON(path, m); err != nil {
		return err
	}

	okLine("added " + strings.Join(added, ", "))
	nextHint("oma build && oma restart")
	return nil
}

func persistPanelMode(dir, mode string) error {
	path := filepath.Join(filepath.Clean(dir), "oma.json")
	data, err := os.ReadFile(path)
	if err != nil {
		// missing file - create stub via omaJSONStubFor
		if os.IsNotExist(err) {
			// create file with schema + panel mode if not attached
			if mode != "attached" {
				return os.WriteFile(path, []byte(omaJSONStubFor(true, mode)), 0o644)
			}
			return nil
		}
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if mode == "attached" {
		// attached is the default — drop a mode-only panel key to keep the file minimal
		if raw["panel"] != nil {
			if m, ok := raw["panel"].(map[string]any); ok && len(m) == 1 && m["mode"] == "attached" {
				delete(raw, "panel")
			} else if m, ok := raw["panel"].(map[string]any); ok {
				m["mode"] = mode
			}
		}
	} else {
		panel := map[string]any{"mode": mode}
		if existing, ok := raw["panel"].(map[string]any); ok {
			for k, v := range existing {
				if k != "mode" {
					panel[k] = v
				}
			}
		}
		raw["panel"] = panel
	}
	if _, ok := raw["$schema"]; !ok {
		if schema, err := ensureSchema(); err == nil {
			raw["$schema"] = schema
		}
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

// --- surface add TUI (single clack-style screen) ---

type surfaceAddModel struct {
	options []string
	cursor  int
	chosen  map[int]bool
	help    help.Model
	theme   theme
	aborted bool
}

func promptSurfaceAdd(options []string) ([]string, bool) {
	m := surfaceAddModel{options: options, chosen: map[int]bool{}, help: help.New(), theme: currentTheme()}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, true
	}
	sm := final.(surfaceAddModel)
	if sm.aborted {
		return nil, true
	}
	var picked []string
	for i, s := range sm.options {
		if sm.chosen[i] {
			picked = append(picked, s)
		}
	}
	return picked, false
}

func (m surfaceAddModel) Init() tea.Cmd { return nil }

func (m surfaceAddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.aborted = true
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Toggle):
			m.chosen[m.cursor] = !m.chosen[m.cursor]
		case key.Matches(msg, keys.Confirm):
			has := false
			for _, v := range m.chosen {
				if v {
					has = true
					break
				}
			}
			if !has {
				return m, nil
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m surfaceAddModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	diamond := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(omaLogoFrame(m.theme, logoDoneFrame))
	b.WriteString("\n\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma surface add"))
	b.WriteString("\n")
	writeBarLine(&b, bar)
	b.WriteString(diamond.Render("◆  "))
	b.WriteString(fg.Render("select surfaces to add"))
	b.WriteString("\n")
	writeBarLine(&b, bar)

	for i, s := range m.options {
		mark := muted.Render("○")
		if m.chosen[i] {
			mark = accent.Render("●")
		}
		raw := "  " + s
		if m.cursor == i {
			raw = "› " + s
		}
		label := fg.Render(fmt.Sprintf("%-14s", raw))
		if m.cursor == i {
			label = accent.Render(fmt.Sprintf("%-14s", raw))
		}
		b.WriteString(bar.Render("│  "))
		b.WriteString(mark)
		b.WriteString(" ")
		b.WriteString(label)
		b.WriteString(" ")
		b.WriteString(muted.Render(surfaceDesc[s]))
		b.WriteString("\n")
	}

	writeBarLine(&b, bar)
	b.WriteString(bar.Render("└  "))
	b.WriteString(muted.Render(m.help.View(keys)))
	return b.String()
}

// --- panel mode picker (when panel is added interactively) ---

type panelModeModel struct {
	cursor int
	help   help.Model
	theme  theme
}

func promptPanelMode(current string) (string, bool) {
	modes := []string{"attached", "window", "both"}
	cur := 0
	for i, m := range modes {
		if m == current {
			cur = i
			break
		}
	}
	m := panelModeModel{cursor: cur, help: help.New(), theme: currentTheme()}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", true
	}
	sm := final.(panelModeModel)
	if sm.cursor < 0 {
		return "", true
	}
	return modes[sm.cursor], false
}

func (m panelModeModel) Init() tea.Cmd { return nil }

func (m panelModeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.cursor = -1
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < 2 {
				m.cursor++
			}
		case key.Matches(msg, keys.Confirm):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m panelModeModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	diamond := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	descs := map[string]string{
		"attached": "bar-anchored popup + widget (panel+bar-widget)",
		"window":   "draggable FloatingWindow, no widget (only-window)",
		"both":     "both: anchored + floating + widget",
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(omaLogoFrame(m.theme, logoDoneFrame))
	b.WriteString("\n\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma surface add - panel mode"))
	b.WriteString("\n")
	writeBarLine(&b, bar)
	b.WriteString(diamond.Render("◆  "))
	b.WriteString(fg.Render("how should the panel be presented?"))
	b.WriteString("\n")
	writeBarLine(&b, bar)
	for i, opt := range []string{"attached", "window", "both"} {
		mark := muted.Render("○")
		if m.cursor == i {
			mark = accent.Render("●")
		}
		raw := "  " + opt
		if m.cursor == i {
			raw = "› " + opt
		}
		label := fg.Render(fmt.Sprintf("%-14s", raw))
		if m.cursor == i {
			label = accent.Render(fmt.Sprintf("%-14s", raw))
		}
		b.WriteString(bar.Render("│  "))
		b.WriteString(mark)
		b.WriteString(" ")
		b.WriteString(label)
		b.WriteString(" ")
		b.WriteString(muted.Render(descs[opt]))
		b.WriteString("\n")
	}
	writeBarLine(&b, bar)
	b.WriteString(bar.Render("└  "))
	b.WriteString(muted.Render(m.help.View(keys)))
	return b.String()
}
