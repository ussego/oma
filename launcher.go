package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// launcher entry materialization: oma.json launchers[] entries become
// ~/.local/share/applications/<id>.desktop files so panels/overlays are
// launchable from the omarchy menu (omarchy-shell shell summon <id>).

const managedMarker = "X-Oma-Managed=true"

func applicationsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "applications"), nil
}

// kindLabel picks a human label from the manifest kinds for GenericName.
func kindLabel(kinds []string) string {
	for _, k := range kinds {
		switch k {
		case "panel":
			return "Panel"
		case "overlay":
			return "Overlay"
		case "menu":
			return "Launcher"
		case "bar-widget":
			return "Bar widget"
		case "bar":
			return "Status bar"
		case "service":
			return "Service"
		}
	}
	return "Plugin"
}

// slug lowercases and joins non-alphanumerics with dashes for file names.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// defaultLauncherAction picks the sensible launcher default for a plugin:
// summon needs a summonable surface (panel/overlay); bar-widget-only plugins
// toggle instead.
func defaultLauncherAction(kinds []string) string {
	if contains(kinds, "panel") || contains(kinds, "overlay") {
		return "summon"
	}
	return "toggle"
}

// resolveLauncherIcon returns the Icon= value for one entry. Theme names
// pass through unchanged; URL and existing-file refs are materialized into
// the user hicolor theme (mirroring omarchy-tui-install) when materialize
// is true, so the .desktop can reference a plain theme name. materialize
// is false for read-only display (oma status).
func resolveLauncherIcon(ref, name, dir string, materialize bool) string {
	if !materialize || ref == "" {
		return ref
	}
	src := ref
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		if !filepath.IsAbs(src) {
			src = filepath.Join(dir, src)
		}
		if _, err := os.Stat(src); err != nil {
			return ref // theme name (or path that does not exist): pass through
		}
	}
	iconName, err := materializeIcon(ref, name, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: icon %q: %v\n", ref, err)
		return ref
	}
	return iconName
}

// materializeIcon installs an icon URL or file into the user hicolor theme
// and returns the theme name to reference. SVG refs go to scalable/apps
// (freedesktop hicolor spec); everything else to 256x256/apps. Already
// installed icons are skipped. The icon cache refresh is best-effort, like
// omarchy-tui-install.
func materializeIcon(ref, name, dir string) (string, error) {
	iconName := safeIconName(name)
	if iconName == "" {
		iconName = "oma"
	}
	base := filepath.Join(iconsHome(), "hicolor")
	ext := "png"
	sub := "256x256"
	if strings.HasSuffix(strings.ToLower(ref), ".svg") {
		ext, sub = "svg", "scalable"
	} else if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		if e := strings.TrimPrefix(filepath.Ext(ref), "."); e != "" {
			ext = e
		}
	}
	targetDir := filepath.Join(base, sub, "apps")
	target := filepath.Join(targetDir, iconName+"."+ext)
	if _, err := os.Stat(target); err == nil {
		return iconName, nil // already installed
	}

	data, err := iconData(ref, dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if err := writeFile(target, string(data)); err != nil {
		return "", err
	}
	if err := exec.Command("gtk-update-icon-cache", filepath.Join(base)).Run(); err != nil {
		// cache refresh is best-effort; the icon still resolves by name
	}
	return iconName, nil
}

// iconData fetches a URL or reads a local file (absolute or relative to dir).
func iconData(ref, dir string) ([]byte, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(ref)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", ref, resp.Status)
		}
		return io.ReadAll(resp.Body)
	}
	src := ref
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, src)
	}
	return os.ReadFile(src)
}

// safeIconName mirrors omarchy-tui-install's sanitizer: lowercase,
// non-alphanumeric runs become dashes, leading/trailing dashes trimmed.
func safeIconName(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			lastDash = true
			continue
		}
		b.WriteRune(r)
		lastDash = false
	}
	return b.String()
}

func iconsHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".icons"
	}
	return filepath.Join(home, ".local", "share", "icons")
}

// renderLauncherEntry produces the .desktop file body for one configured entry.
// filename is <id>.desktop for the first entry, <id>-<slug(name)>.desktop after.
func renderLauncherEntry(m manifest, def launcherEntryDef, globalIcon string, index int, dir string, materialize bool) (string, string) {
	execLine := def.Exec
	tryExec := ""
	if execLine == "" {
		action := def.Action
		if action == "" {
			action = defaultLauncherAction(m.Kinds)
		}
		execLine = fmt.Sprintf("omarchy-shell shell %s %s", action, m.ID)
		tryExec = "TryExec=omarchy-shell\n"
	}

	icon := def.Icon
	if icon == "" {
		icon = globalIcon
	}
	if icon == "" {
		icon = defaultIcon
	}
	icon = resolveLauncherIcon(icon, def.Name, dir, materialize)

	generic := def.GenericName
	if generic == "" {
		generic = kindLabel(m.Kinds)
	}

	comment := def.Comment
	if comment == "" {
		comment = m.Description
	}

	categories := def.Categories
	if len(categories) == 0 {
		categories = []string{"Utility"}
	}

	keywords := def.Keywords
	if len(keywords) == 0 {
		keywords = strings.Fields(strings.ToLower(def.Name))
		for _, part := range strings.Split(m.ID, ".") {
			keywords = append(keywords, part)
		}
	}

	filename := m.ID + ".desktop"
	if index > 0 {
		filename = fmt.Sprintf("%s-%s.desktop", m.ID, slug(def.Name))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[Desktop Entry]\n")
	fmt.Fprintf(&b, "Type=Application\n")
	fmt.Fprintf(&b, "Version=1.0\n")
	fmt.Fprintf(&b, "Name=%s\n", def.Name)
	if generic != "" {
		fmt.Fprintf(&b, "GenericName=%s\n", generic)
	}
	if comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", comment)
	}
	fmt.Fprintf(&b, "Exec=%s\n", execLine)
	fmt.Fprintf(&b, "%s", tryExec)
	fmt.Fprintf(&b, "Icon=%s\n", icon)
	fmt.Fprintf(&b, "Terminal=%t\n", def.Terminal)
	fmt.Fprintf(&b, "Categories=%s;\n", strings.Join(categories, ";"))
	fmt.Fprintf(&b, "Keywords=%s;\n", strings.Join(keywords, ";"))
	fmt.Fprintf(&b, "StartupNotify=false\n")
	fmt.Fprintf(&b, "%s\n", managedMarker)
	fmt.Fprintf(&b, "X-Oma-Id=%s\n", m.ID)

	return filename, b.String()
}

// writeLauncherEntries materializes every oma.json launchers[] entry. A missing
// oma.json or empty launchers[] writes nothing; config errors are returned so
// explicit commands fail loudly (install warns-and-skips instead).
func writeLauncherEntries(dir string) ([]string, error) {
	m, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	cfg, err := loadOMAConfig(dir)
	if err != nil {
		return nil, err
	}
	if len(cfg.Launchers) == 0 {
		return nil, nil
	}
	apps, err := applicationsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for i, def := range cfg.Launchers {
		filename, content := renderLauncherEntry(m, def, cfg.Icon, i, dir, true)
		path := filepath.Join(apps, filename)
		if err := writeFile(path, content); err != nil {
			return written, fmt.Errorf("write %s: %w", filepath.Base(path), err)
		}
		written = append(written, path)
	}
	return written, nil
}

// removeLauncherEntries deletes only launcher files this project created —
// they must match the plugin id AND carry the X-Oma-Managed marker.
func removeLauncherEntries(dir string) ([]string, error) {
	m, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	// still validate oma.json so a broken config surfaces here too
	if _, err := loadOMAConfig(dir); err != nil {
		return nil, err
	}
	apps, err := applicationsDir()
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(apps, m.ID+"*.desktop")
	matches, _ := filepath.Glob(pattern)
	var removed []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), managedMarker) {
			continue // not ours — never touch foreign entries
		}
		if err := os.Remove(path); err == nil {
			removed = append(removed, path)
		}
	}
	return removed, nil
}

// hasDeclaredLauncherEntries reports whether dir/oma.json declares any entries.
func hasDeclaredLauncherEntries(dir string) bool {
	cfg, err := loadOMAConfig(dir)
	return err == nil && cfg != nil && len(cfg.Launchers) > 0
}

// --- launcher add: upsert an entry into oma.json, then materialize it ---

// upsertLauncherDef appends def to dir/oma.json launchers[] (replacing an
// entry with the same name in place). A missing oma.json is created with the
// $schema stub so editors keep autocompleting. Reports whether an existing
// entry was replaced.
func upsertLauncherDef(dir string, def launcherEntryDef) (bool, error) {
	path := filepath.Join(filepath.Clean(dir), "oma.json")
	raw := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	} else if schema, err := ensureSchema(); err == nil && schema != "" {
		raw["$schema"] = schema
	}

	entries, _ := raw["launchers"].([]any)
	replaced := false
	for i, e := range entries {
		if m, ok := e.(map[string]any); ok && m["name"] == def.Name {
			entries[i] = def
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, def)
	}
	raw["launchers"] = entries

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return replaced, err
	}
	return replaced, os.WriteFile(path, append(data, '\n'), 0o644)
}

// runLauncherAdd implements `oma launcher add`: flags define the entry
// non-interactively; without a name the wizard prompts for name, command and
// icon. Either way the entry lands in oma.json and its .desktop file is
// written.
func runLauncherAdd(dir string, args []string) error {
	def := launcherEntryDef{Action: "summon"}
	var execOverride string
	explicitAction := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s needs a value", a)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--action" || a == "-a":
			v, err := next()
			if err != nil {
				return err
			}
			def.Action = strings.ToLower(v)
			explicitAction = true
		case strings.HasPrefix(a, "--action="):
			def.Action = strings.ToLower(strings.TrimPrefix(a, "--action="))
			explicitAction = true
		case a == "--exec" || a == "-e":
			v, err := next()
			if err != nil {
				return err
			}
			execOverride = v
		case strings.HasPrefix(a, "--exec="):
			execOverride = strings.TrimPrefix(a, "--exec=")
		case a == "--icon" || a == "-i":
			v, err := next()
			if err != nil {
				return err
			}
			def.Icon = v
		case strings.HasPrefix(a, "--icon="):
			def.Icon = strings.TrimPrefix(a, "--icon=")
		case a == "--comment" || a == "-c":
			v, err := next()
			if err != nil {
				return err
			}
			def.Comment = v
		case strings.HasPrefix(a, "--comment="):
			def.Comment = strings.TrimPrefix(a, "--comment=")
		case a == "--terminal":
			def.Terminal = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			def.Name = a
		}
	}

	switch def.Action {
	case "", "summon", "toggle", "hide":
	default:
		return fmt.Errorf("unknown action %q (summon, toggle or hide)", def.Action)
	}
	if execOverride != "" {
		def.Exec = execOverride
		if !explicitAction {
			// exec overrides action — don't persist a stale one alongside it
			def.Action = ""
		}
	}

	if def.Name == "" {
		m, err := readManifest(filepath.Join(dir, "manifest.json"))
		if err != nil {
			return err
		}
		cfg, err := loadOMAConfig(dir)
		if err != nil {
			return err
		}
		existing := make([]string, 0, len(cfg.Launchers))
		for _, l := range cfg.Launchers {
			existing = append(existing, l.Name)
		}
		in := launcherWizardInput{
			id:            m.ID,
			kinds:         m.Kinds,
			existingNames: existing,
			prefillIcon:   firstNonEmpty(def.Icon, cfg.Icon),
			prefillCustom: execOverride,
			defaultAction: defaultLauncherAction(m.Kinds),
		}
		if explicitAction {
			in.defaultAction = def.Action // an explicit flag steers the start selection
		}
		wdef, aborted, err := promptLauncherWizard(in)
		if err != nil {
			return err
		}
		if aborted {
			return nil
		}
		// the wizard owns name/command/icon; flag-only fields survive
		def.Name = wdef.Name
		def.Action = wdef.Action
		def.Exec = wdef.Exec
		def.Icon = wdef.Icon
	}
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("launcher name cannot be empty")
	}

	replaced, err := upsertLauncherDef(dir, def)
	if err != nil {
		return err
	}

	written, err := writeLauncherEntries(dir)
	if err != nil {
		return err
	}
	action := def.Action
	if def.Exec != "" {
		action = "custom exec"
	}
	verb := "added"
	if replaced {
		verb = "updated"
	}
	okLine(fmt.Sprintf("%s %q (%s)", verb, def.Name, action))
	for _, p := range written {
		okLine("launcher " + displayPath(p))
	}
	return nil
}

// firstNonEmpty returns the first argument that is not empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// --- launcher add wizard (sequential clack-style prompts) ---
//
// One question on screen at a time, mirroring oma create: each answer
// freezes into the box as history before the next prompt appears.

// launcherCustomRow is the command-list index of the custom… row.
const launcherCustomRow = 3

// launcherActions are the preset command rows of the wizard's command list;
// index len(launcherActions) is the custom… row (launcherCustomRow mirrors
// this length). Keep both in sync when adding an action.
var launcherActions = []string{"summon", "toggle", "hide"}

// launcherActionLabels[i] is the muted description for command row i; its
// length must stay len(launcherActions)+1.
var launcherActionLabels = [4]string{
	"open the panel",
	"show / hide",
	"close if open",
	"type any command",
}

// Wizard steps, asked in order.
const (
	stepName = iota
	stepCommand
	stepIcon
)

type launcherWizardInput struct {
	id            string   // plugin id, shown inside the preset commands
	kinds         []string // decides summon validity + default action
	existingNames []string // declared launchers; a match shows an update note
	prefillIcon   string   // --icon flag beats the oma.json global icon
	prefillCustom string   // --exec flag seeds the custom… row
	defaultAction string   // manifest-derived starting selection
}

type launcherWizardModel struct {
	input   launcherWizardInput
	theme   theme
	step    int    // stepName → stepCommand → stepIcon
	name    string // committed answers
	action  string // committed preset action ("" when a custom command wins)
	exec    string // committed custom command
	icon    string // committed at the final step
	command int    // selected command row during stepCommand
	editing bool   // typing into the custom… input within stepCommand
	err     string // inline validation error for the active prompt
	ti      textinput.Model
	aborted bool
	done    bool
}

// newLauncherWizardModel builds the first prompt: an empty name input, with
// command selection from the kinds (or an explicit flag / seeded --exec).
func newLauncherWizardModel(in launcherWizardInput) launcherWizardModel {
	m := launcherWizardModel{
		input: in,
		theme: currentTheme(),
	}
	for i, a := range launcherActions {
		if a == in.defaultAction {
			m.command = i
			break
		}
	}
	if in.prefillCustom != "" {
		m.command = launcherCustomRow
	}
	m.rebuildInput()
	return m
}

// rebuildInput configures the single active text input for the current
// context; only one exists at a time because the flow is sequential.
func (m *launcherWizardModel) rebuildInput() tea.Cmd {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Width = 60
	ti.Focus()
	switch {
	case m.step == stepName:
		ti.Placeholder = "Launcher name"
		ti.CharLimit = 64
	case m.step == stepCommand && m.editing:
		ti.Placeholder = "command to run"
		ti.CharLimit = 256
		ti.SetValue(strings.TrimSpace(m.input.prefillCustom))
	case m.step == stepIcon:
		ti.Placeholder = "icon name (blank = inherit)"
		ti.CharLimit = 128
		ti.SetValue(strings.TrimSpace(m.input.prefillIcon))
	}
	m.ti = ti
	return textinput.Blink
}

// promptLauncherWizard runs the prompts and returns the entry fragment they
// collected: Name plus either Action (preset) or Exec (custom), and Icon
// ("" inherits the global icon).
func promptLauncherWizard(in launcherWizardInput) (def launcherEntryDef, aborted bool, err error) {
	p := tea.NewProgram(newLauncherWizardModel(in))
	final, err := p.Run()
	if err != nil {
		return launcherEntryDef{}, true, err
	}
	lm := final.(launcherWizardModel)
	if lm.aborted || !lm.done {
		return launcherEntryDef{}, true, nil
	}
	return lm.result(), false, nil
}

// summonable reports whether the plugin has a surface summon can open.
func summonable(kinds []string) bool {
	return contains(kinds, "panel") || contains(kinds, "overlay")
}

// result folds the committed answers into the entry to save.
func (m launcherWizardModel) result() launcherEntryDef {
	def := launcherEntryDef{Name: m.name, Icon: m.icon}
	if m.exec != "" {
		def.Exec = m.exec // exec-only: action omitted, exec overrides it anyway
	} else {
		def.Action = m.action
	}
	return def
}

func (m launcherWizardModel) Init() tea.Cmd { return textinput.Blink }

func (m launcherWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "esc":
			if m.step == stepCommand && m.editing {
				m.editing = false // back to the list, keep going
				m.err = ""
				return m, nil
			}
			m.aborted = true
			return m, tea.Quit
		case "enter":
			return m.advance()
		}
		if m.step == stepCommand && !m.editing {
			// the list owns the keys; nothing to type here yet
			n := len(launcherActions) + 1
			switch msg.String() {
			case "up", "left":
				m.command = (m.command + n - 1) % n
			case "down", "right":
				m.command = (m.command + 1) % n
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return m, cmd
}

// advance handles enter for the active prompt and moves to the next one,
// rebuilding the text input along the way.
func (m launcherWizardModel) advance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepName:
		v := strings.TrimSpace(m.ti.Value())
		if v == "" {
			m.err = "name is required"
			return m, nil
		}
		m.name = v
		m.err = ""
		m.step = stepCommand
		return m, nil

	case stepCommand:
		if m.editing {
			v := strings.TrimSpace(m.ti.Value())
			if v == "" {
				m.editing = false // nothing typed: back to the list
				return m, nil
			}
			m.exec = v
			m.action = ""
		} else {
			if m.command == launcherCustomRow {
				m.editing = true
				return m, m.rebuildInput()
			}
			m.action = launcherActions[m.command]
			if m.action == "summon" && !summonable(m.input.kinds) {
				m.action = "toggle" // warned live; matches the writer's fallback
			}
			m.exec = ""
		}
		m.err = ""
		m.step = stepIcon
		return m, m.rebuildInput()

	case stepIcon:
		m.icon = strings.TrimSpace(m.ti.Value()) // blank inherits the global icon
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// history lists the committed answers as frozen "label: value" lines shown
// above the active prompt.
func (m launcherWizardModel) history() []string {
	var h []string
	if m.name != "" {
		h = append(h, formatPrev("name", m.name))
	}
	if m.step > stepCommand {
		cmd := m.exec
		if cmd == "" && m.action != "" {
			cmd = fmt.Sprintf("omarchy-shell shell %s %s", m.action, m.input.id)
		}
		if cmd != "" {
			h = append(h, formatPrev("command", cmd))
		}
	}
	return h
}

func (m launcherWizardModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	check := lipgloss.NewStyle().Foreground(hexColor(m.theme.cyan)).Bold(true)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma launcher add"))
	b.WriteString("\n")
	writeBarLine(&b, bar)

	for _, h := range m.history() {
		parts := strings.SplitN(h, ": ", 2)
		if len(parts) == 2 {
			writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
		} else {
			writeHistoryLine(&b, bar, check, muted, fg, "", h)
		}
	}
	if m.step >= stepCommand && contains(m.input.existingNames, m.name) {
		b.WriteString(bar.Render("│     ") + muted.Render("updates existing entry"))
		b.WriteString("\n")
	}
	if len(m.history()) > 0 {
		writeBarLine(&b, bar)
	}

	switch m.step {
	case stepName:
		b.WriteString(accent.Render("◆  "))
		b.WriteString(fg.Render("name"))
		b.WriteString("\n")
		b.WriteString(bar.Render("│  ") + m.ti.View() + "\n")
		if m.err != "" {
			errStyle := lipgloss.NewStyle().Foreground(hexColor(m.theme.red)).Bold(true)
			b.WriteString(bar.Render("│  ") + errStyle.Render("✖ "+m.err) + "\n")
		}
		writeBarLine(&b, bar)
		b.WriteString(bar.Render("└  ") + muted.Render("enter to continue · esc to cancel"))

	case stepCommand:
		b.WriteString(accent.Render("◆  "))
		b.WriteString(fg.Render("command"))
		b.WriteString(muted.Render(" - pick a preset or type your own"))
		b.WriteString("\n")
		writeBarLine(&b, bar)
		for i := 0; i <= len(launcherActions); i++ {
			mark := muted.Render("○")
			var value, label string
			if i < len(launcherActions) {
				value = fmt.Sprintf("omarchy-shell shell %s %s", launcherActions[i], m.input.id)
			} else {
				value = "custom…"
			}
			label = launcherActionLabels[i]
			selected := !m.editing && i == m.command
			raw := "  " + value
			if selected {
				mark = accent.Render("●")
				raw = "› " + value
			}
			row := fg.Render(fmt.Sprintf("%-52s", raw))
			if selected {
				row = accent.Render(fmt.Sprintf("%-52s", raw))
			}
			b.WriteString(bar.Render("│  ") + mark + " " + row)
			if !(m.editing && i == launcherCustomRow) {
				b.WriteString(muted.Render(label))
			}
			b.WriteString("\n")
			// the custom row opens into its own input line while editing
			if m.editing && i == launcherCustomRow {
				b.WriteString(bar.Render("│        ") + m.ti.View() + "\n")
			}
		}
		if !m.editing && m.command < len(launcherActions) &&
			launcherActions[m.command] == "summon" && !summonable(m.input.kinds) {
			b.WriteString(bar.Render("│  ") + muted.Render("no panel/overlay - saved as toggle") + "\n")
		}
		writeBarLine(&b, bar)
		footer := muted.Render("enter to continue · ↑↓ choose · esc to cancel")
		if m.editing {
			footer = muted.Render("enter to save command · esc back · ctrl+c cancel")
		}
		b.WriteString(bar.Render("└  ") + footer)

	case stepIcon:
		b.WriteString(accent.Render("◆  "))
		b.WriteString(fg.Render("icon"))
		b.WriteString(muted.Render(" - optional"))
		b.WriteString("\n")
		b.WriteString(bar.Render("│  ") + m.ti.View() + "\n")
		writeBarLine(&b, bar)
		b.WriteString(bar.Render("└  ") + muted.Render("enter to finish · esc to cancel"))
	}
	return b.String()
}

// --- launcher remove: drop entries from oma.json + their .desktop files ---

// removeLauncherDefs deletes the named entries (or all when all==true) from
// dir/oma.json launchers[], dropping the key when none remain. Unknown names
// fail the whole call before anything is written.
func removeLauncherDefs(dir string, names []string, all bool) ([]string, error) {
	path := filepath.Join(filepath.Clean(dir), "oma.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no oma.json in %s", dir)
	}
	if err != nil {
		return nil, err
	}
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	entries, _ := raw["launchers"].([]any)

	var declared []string
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			if name, _ := m["name"].(string); name != "" {
				declared = append(declared, name)
			}
		}
	}
	if !all {
		var unknown []string
		for _, n := range names {
			if !contains(declared, n) {
				unknown = append(unknown, n)
			}
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("no launcher named %q (declared: %s)", strings.Join(unknown, ", "), strings.Join(declared, ", "))
		}
	}

	var removed []string
	var kept []any
	for _, e := range entries {
		m, ok := e.(map[string]any)
		name, _ := m["name"].(string)
		if ok && (all || contains(names, name)) {
			removed = append(removed, name)
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		delete(raw, "launchers")
	} else {
		raw["launchers"] = kept
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return removed, err
	}
	return removed, os.WriteFile(path, append(out, '\n'), 0o644)
}

// syncLauncherFiles realigns .desktop files with oma.json: sweep every
// oma-managed file for this id, then re-create the survivors. Cheap and
// removes index-shift guesswork.
func syncLauncherFiles(dir string) ([]string, error) {
	if _, err := removeLauncherEntries(dir); err != nil {
		return nil, err
	}
	return writeLauncherEntries(dir)
}

// runLauncherRemove implements 'oma launcher remove': name args, --all, or a
// multi-select wizard when run without either.
func runLauncherRemove(dir string, args []string) error {
	var names []string
	all := false
	for _, a := range args {
		switch {
		case a == "--all":
			all = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			names = append(names, a)
		}
	}

	cfg, err := loadOMAConfig(dir)
	if err != nil {
		return err
	}
	var declared []string
	for _, l := range cfg.Launchers {
		declared = append(declared, l.Name)
	}
	if len(declared) == 0 {
		// nothing declared: still sweep stray managed files, then done
		swept, err := removeLauncherEntries(dir)
		for _, p := range swept {
			okLine("removed " + displayPath(p))
		}
		if err == nil && len(swept) == 0 {
			noteLine("no launcher entries declared in oma.json")
		}
		return err
	}

	if !all && len(names) == 0 {
		picked, aborted := promptLauncherRemove(declared)
		if aborted {
			return nil
		}
		if len(picked) == 0 {
			noteLine("nothing selected")
			return nil
		}
		names = picked
	}

	removed, err := removeLauncherDefs(dir, names, all)
	if err != nil {
		return err
	}

	written, err := syncLauncherFiles(dir)
	if err != nil {
		return err
	}
	for _, n := range removed {
		okLine("removed " + n)
	}
	for _, p := range written {
		okLine("launcher " + displayPath(p))
	}
	return nil
}

// --- launcher remove wizard (multi-select, mirrors surface add) ---

type launcherRemoveModel struct {
	options []string
	cursor  int
	chosen  map[int]bool
	help    help.Model
	theme   theme
	aborted bool
}

func promptLauncherRemove(options []string) ([]string, bool) {
	m := launcherRemoveModel{options: options, chosen: map[int]bool{}, help: help.New(), theme: currentTheme()}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, true
	}
	lm := final.(launcherRemoveModel)
	if lm.aborted {
		return nil, true
	}
	var picked []string
	for i, s := range lm.options {
		if lm.chosen[i] {
			picked = append(picked, s)
		}
	}
	return picked, false
}

func (m launcherRemoveModel) Init() tea.Cmd { return nil }

func (m launcherRemoveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, keys.Quit):
			m.aborted = true
			return m, tea.Quit
		case key.Matches(msg, keys.Toggle):
			m.chosen[m.cursor] = !m.chosen[m.cursor]
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
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

func (m launcherRemoveModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma launcher remove"))
	b.WriteString("\n")
	b.WriteString(bar.Render("│  "))
	b.WriteString(fg.Render("select entries to remove"))
	for i, s := range m.options {
		mark := muted.Render("○")
		if m.chosen[i] {
			mark = accent.Render("●")
		}
		row := "  " + s
		if m.cursor == i {
			row = "› " + s
		}
		b.WriteString("\n")
		b.WriteString(bar.Render("│  ") + mark + " " + fg.Render(row))
	}
	b.WriteString("\n")
	b.WriteString(bar.Render("└  "))
	b.WriteString(muted.Render("space toggle · ↑↓ move · enter remove · esc cancel"))
	return b.String()
}
