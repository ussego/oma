package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var surfaces = []string{"bar", "bar-widget", "menu", "panel", "overlay", "service"}

var surfaceDesc = map[string]string{
	"bar":        "the status bar itself",
	"bar-widget": "a widget in the bar",
	"menu":       "a launcher/menu",
	"panel":      "a panel: attached popup, floating window, or both (--panel-mode)",
	"overlay":    "a popup layer",
	"service":    "a background service",
}

type keymap struct {
	Up, Down, Toggle, Confirm, Quit key.Binding
}

var keys = keymap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Toggle:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Confirm, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

// runCreate scaffolds a project. All inputs can be passed directly
// (oma create <name> -s panel,overlay -a <author> -d "desc" -v 1.0.0);
// anything missing is prompted for in the wizard, pre-seeded with flags.
func runCreate(args []string) error {
	f, err := parseCreateArgs(args)
	if err != nil {
		return err
	}

	nameEmpty := f.name == ""
	kindsEmpty := len(f.kinds) == 0

	if !nameEmpty && !kindsEmpty {
		// fully non-interactive - never prompt, fail on conflicts
		if err := validateProjectName(f.name); err != nil {
			return err
		}
		if err := validateVersion(f.ver); err != nil {
			return err
		}
		if err := validateAuthor(f.author); err != nil {
			return err
		}
		if dirExistsWithFiles(f.name) {
			return fmt.Errorf("directory %q already exists and is not empty", filepath.Base(filepath.Clean(f.name)))
		}
		mode := f.panelMode
		if mode == "" {
			mode = "attached"
		}
		files, err := scaffoldWithOptions(f.name, f.kinds, scaffoldOptions{
			Description: f.desc,
			Version:     f.ver,
			Author:      f.author,
			PanelMode:   mode,
		})
		if err != nil {
			return err
		}
		printCreated(files)
		printSkillsHint()
		return nil
	}

	// interactive: ask before the wizard when the name came from args, so the
	// user doesn't fill everything only to hit the conflict at the end
	preConfirmed := ""
	if !nameEmpty && dirExistsWithFiles(f.name) {
		ok, aborted := confirmOverwrite(f.name)
		if aborted || !ok {
			printCancelled("existing folder left untouched")
			return nil
		}
		preConfirmed = f.name
	}

	res, aborted := runWizard(f, preConfirmed)
	if aborted {
		return nil
	}
	// collisions were gated in the wizard; scaffold failures surface here
	if res.err != "" {
		return fmt.Errorf("%s", res.err)
	}
	return nil
}

// chosenKinds collects the selected surfaces in canonical order.
func chosenKinds(m wizardModel) []string {
	var kinds []string
	for i, s := range surfaces {
		if m.chosen[i] {
			kinds = append(kinds, s)
		}
	}
	return kinds
}

func hasPanelChosen(m wizardModel) bool {
	for i, s := range surfaces {
		if s == "panel" && m.chosen[i] {
			return true
		}
	}
	return false
}

// dirExistsWithFiles reports whether the target directory already exists and
// contains entries (an empty existing dir is fine to scaffold into).
func dirExistsWithFiles(name string) bool {
	entries, err := os.ReadDir(filepath.Clean(name))
	return err == nil && len(entries) > 0
}

// --- overwrite confirmation ---

type confirmModel struct {
	dir     string
	cursor  int // 0 = No, 1 = Yes
	options []string
	theme   theme
	aborted bool
	value   bool
}

func confirmOverwrite(dir string) (bool, bool) {
	// alt screen: the confirm box disappears completely on exit, so the
	// wizard (or the cancel line) starts from a clean terminal
	p := tea.NewProgram(confirmModel{
		dir:     dir,
		options: []string{"No", "Yes"},
		theme:   currentTheme(),
	}, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return false, true
	}
	m := final.(confirmModel)
	if m.aborted {
		return false, true
	}
	return m.cursor == 1, false
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			return m, tea.Quit
		case tea.KeyLeft, tea.KeyUp:
			m.cursor = 0
		case tea.KeyRight, tea.KeyDown:
			m.cursor = 1
		case tea.KeyEnter:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	diamond := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	warn := lipgloss.NewStyle().Foreground(hexColor(m.theme.red)).Bold(true)

	var b strings.Builder
	// no logo here: this is a transient pre-step, and when confirmed the
	// wizard below plays its own animated header - one header per run
	b.WriteString("\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma create"))
	b.WriteString("\n")
	writeBarLine(&b, bar)
	b.WriteString(diamond.Render("◆  "))
	b.WriteString(fg.Render(`directory "`) + warn.Render(filepath.Base(filepath.Clean(m.dir))) + fg.Render(`/" already exists`))
	b.WriteString("\n")
	writeBarLine(&b, bar)
	// each option states its consequence, so No/Yes are never ambiguous
	descs := []string{"keep the existing folder untouched", "scaffold anyway and replace overlapping files"}
	for i, opt := range m.options {
		mark := muted.Render("○")
		label := muted.Render(opt)
		if m.cursor == i {
			mark = accent.Render("●")
			label = accent.Bold(true).Render(opt)
		}
		b.WriteString(bar.Render("│  ") + mark + " " + label + muted.Render("   "+descs[i]))
		b.WriteString("\n")
	}
	b.WriteString(bar.Render("│") + "\n")
	b.WriteString(bar.Render("└  ") + muted.Render("← → to choose · enter confirms · esc cancels"))
	// trailing newline so shutdown's [2K erase can't wipe this line
	b.WriteString("\n")
	return b.String()
}

// printCreated logs each scaffolded file with a checkmark plus a count,
// like create-vite & friends.
func printCreated(files []string) {
	t := currentTheme()
	check := lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(hexColor(t.muted))
	for _, f := range files {
		fmt.Println(check.Render("● ") + f)
	}
	fmt.Println(muted.Render(fmt.Sprintf("%d files created", len(files))))
}

// printSkillsHint advertises the agent-skill install command after a
// non-interactive create, mirroring the wizard outro line.
func printSkillsHint() {
	t := currentTheme()
	muted := lipgloss.NewStyle().Foreground(hexColor(t.muted))
	fmt.Println(muted.Render("npx skills add ussego/oma  - agent skills for this project"))
}

// printCancelled prints the red cancel line used whenever an interactive
// flow is declined or aborted (safe to call after the tea program exited).
func printCancelled(msg string) {
	t := currentTheme()
	errStyle := lipgloss.NewStyle().Foreground(hexColor(t.red)).Bold(true)
	fmt.Println(errStyle.Render("✖ ") + msg)
}

type createFlags struct {
	name, desc, ver, author string
	kinds                   []string
	panelMode               string // attached | window | both
}

// parseCreateArgs parses positionals plus bun-style flags. Both "--flag val"
// and "--flag=val" forms work; a lone positional after the name is still
// accepted as comma-separated kinds (oma create foo panel,overlay).
func parseCreateArgs(args []string) (createFlags, error) {
	var f createFlags
	var pos []string
	takeVal := func(i int, flag string) (string, int, error) {
		// handles "-s panel" and returns value + consumed count
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("flag %s needs a value", flag)
		}
		return args[i+1], 1, nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := ""
		flag := ""
		switch {
		case strings.HasPrefix(a, "--"):
			flag = strings.SplitN(a[2:], "=", 2)[0]
			if eq := strings.Index(a, "="); eq >= 0 {
				val = a[eq+1:]
			} else {
				v, n, err := takeVal(i, flag)
				if err != nil {
					return f, err
				}
				val = v
				i += n
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			flag = a[1:]
			if eq := strings.Index(a, "="); eq >= 0 {
				flag = a[1:eq]
				val = a[eq+1:]
			} else {
				v, n, err := takeVal(i, flag)
				if err != nil {
					return f, err
				}
				val = v
				i += n
			}
		default:
			pos = append(pos, a)
			continue
		}
		switch flag {
		case "s", "surfaces":
			f.kinds = parseKinds(val)
		case "a", "author":
			f.author = strings.TrimSpace(val)
		case "d", "description", "desc":
			f.desc = strings.TrimSpace(val)
		case "v", "version":
			f.ver = strings.TrimSpace(val)
		case "panel-mode", "p":
			f.panelMode = strings.ToLower(strings.TrimSpace(val))
			switch f.panelMode {
			case "attached", "window", "both":
			default:
				return f, fmt.Errorf("unknown --panel-mode %q (attached, window or both)", val)
			}
		default:
			return f, fmt.Errorf("unknown flag %q (see oma create --help)", flag)
		}
	}
	if len(pos) > 0 {
		f.name = pos[0]
	}
	if len(pos) > 1 {
		// legacy positional kinds: oma create foo panel,overlay
		extra := parseKinds(strings.Join(pos[1:], ","))
		f.kinds = append(f.kinds, extra...)
	}
	for _, k := range f.kinds {
		if !isSurface(k) {
			return f, fmt.Errorf("unknown kind %q (choose from %s)", k, strings.Join(surfaces, ", "))
		}
	}
	return f, nil
}

func parseKinds(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(k)
		if k == "" || contains(out, k) {
			continue
		}
		out = append(out, k)
	}
	return out
}

type wizardResult struct {
	name, desc, ver, author string
	kinds                   []string
	created                 []string // files written by the outro-time scaffold
	err                     string   // scaffold failure, if any
}

func runWizard(f createFlags, preConfirmed string) (wizardResult, bool) {
	theme := currentTheme()
	m := wizardModel{
		theme:        theme,
		help:         help.New(),
		chosen:       map[int]bool{},
		preConfirmed: preConfirmed,
	}
	// seed from flags; the wizard only prompts for what's missing
	m.name = f.name
	m.desc = f.desc
	m.ver = f.ver
	m.author = f.author
	if m.name != "" {
		if err := validateProjectName(m.name); err != nil {
			// invalid name via args - force correction in wizard
			m.err = err.Error()
		}
	}
	for _, k := range f.kinds {
		for i, s := range surfaces {
			if s == k {
				m.chosen[i] = true
			}
		}
	}
	m.panelMode = f.panelMode
	if m.panelMode == "" {
		m.panelMode = "attached"
	}
	// map mode to cursor
	switch m.panelMode {
	case "window":
		m.panelCursor = 1
	case "both":
		m.panelCursor = 2
	default:
		m.panelCursor = 0
	}
	m.step = m.firstMissing(0)
	m.maxStep = m.step
	if m.step < 4 {
		m.input = newWizardInput(m.step, m.name)
		if m.name != "" && m.step == 0 && m.err != "" {
			m.input.SetValue(m.name)
		}
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return wizardResult{}, true
	}
	wm := final.(wizardModel)
	if wm.aborted {
		return wizardResult{}, true
	}
	ver := wm.ver
	if ver == "" {
		ver = "0.1.0"
	}
	var kinds []string
	for i, s := range surfaces {
		if wm.chosen[i] {
			kinds = append(kinds, s)
		}
	}
	return wizardResult{name: wm.name, desc: wm.desc, ver: ver, author: wm.author, kinds: kinds, created: wm.created, err: wm.scaffoldErr}, false
}

// firstMissing returns the first wizard step whose input has no value yet,
// so flag-provided fields are skipped and only gaps are prompted.
// Version (step 2) is never prompted: it is settable only via -v/--version
// and defaults to 0.1.0.
func (m *wizardModel) firstMissing(from int) int {
	s := from
	for s < 4 {
		switch s {
		case 0:
			if m.name == "" {
				return s
			}
		case 1:
			if m.desc == "" {
				return s
			}
		case 2:
			// version is flag-only; skip
		case 3:
			if m.author == "" {
				return s
			}
		}
		s++
	}
	return 4
}

func newWizardInput(step int, name string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Width = 40
	ti.Focus()
	switch step {
	case 0:
		ti.Placeholder = "my-plugin"
		ti.CharLimit = 40
	case 1:
		ti.Placeholder = "A custom plugin for Omarchy - built with Oma"
		ti.CharLimit = 100
	case 3:
		ti.Placeholder = userNamespace()
		ti.CharLimit = 40
	default:
		ti.Placeholder = ""
		ti.CharLimit = 40
	}
	return ti
}

// --- single clack-like wizard (one tea program, no stacked boxes) ---

type tickMsg time.Time

func logoTick() tea.Cmd {
	return tea.Tick(30*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type wizardModel struct {
	theme        theme
	step         int // 0:name 1:desc 2:ver 3:author 4:surfaces 5:outro 6:cancelled 7:overwrite-confirm 8:panel-mode
	maxStep      int // furthest text step reached; drives history visibility
	frame        int // logo entry animation frame
	name         string
	desc         string
	ver          string
	author       string
	panelMode    string // attached | window | both
	panelCursor  int
	input        textinput.Model
	cursor       int
	chosen       map[int]bool
	help         help.Model
	aborted      bool
	done         bool
	err          string
	created      []string // files written at confirm time, shown in the outro
	scaffoldErr  string
	preConfirmed string // name whose overwrite was already accepted (args path)
	owCursor     int    // overwrite confirm selection: 0=No, 1=Yes
}

// scaffoldForOutro runs the project write at surfaces-confirm time so the
// outro can show the real file list, then transitions to the outro.
func (m wizardModel) scaffoldForOutro() (tea.Model, tea.Cmd) {
	ver := m.ver
	if ver == "" {
		ver = "0.1.0"
	}
	modes := []string{"attached", "window", "both"}
	mode := m.panelMode
	if mode == "" && len(modes) > m.panelCursor {
		mode = modes[m.panelCursor]
	}
	if mode == "" {
		mode = "attached"
	}
	if !hasPanelChosen(m) {
		mode = "attached"
	}
	files, err := scaffoldWithOptions(m.name, chosenKinds(m), scaffoldOptions{
		Description: m.desc,
		Version:     ver,
		Author:      m.author,
		PanelMode:   mode,
	})
	m.created = files
	if err != nil {
		m.scaffoldErr = err.Error()
	}
	m.step = 5
	m.done = true
	// settle the logo for the outro frame
	m.frame = logoDoneFrame
	return m, tea.Quit
}

func (m wizardModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, logoTick())
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.frame < logoDoneFrame {
			m.frame++
			return m, logoTick()
		}
		return m, nil
	case tea.KeyMsg:
		if m.step == 5 {
			// outro - any key exits
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			m.aborted = true
			m.step = 6
			m.frame = logoDoneFrame
			return m, tea.Quit
		}
		// "q" quits from selection/confirm steps, but must be typed in text inputs (e.g. "quattro")
		if key.Matches(msg, keys.Quit) {
			if m.step < 4 {
				// let textinput handle "q" as character
			} else {
				m.aborted = true
				m.step = 6
				m.frame = logoDoneFrame
				return m, tea.Quit
			}
		}
		if m.step < 4 {
			// clear previous validation error on any edit
			if m.err != "" && msg.Type != tea.KeyEnter {
				m.err = ""
			}
			if msg.Type == tea.KeyEnter {
				v := strings.TrimSpace(m.input.Value())
				allowEmpty := m.step != 0
				if v == "" && !allowEmpty {
					m.err = "project name is required"
					return m, nil
				}
				var vErr error
				switch m.step {
				case 0:
					vErr = validateProjectName(v)
				case 1:
					// description: no strict validation, just length already limited
					vErr = nil
				case 2:
					vErr = validateVersion(v)
				case 3:
					vErr = validateAuthor(v)
				}
				if vErr != nil {
					m.err = vErr.Error()
					return m, nil
				}
				switch m.step {
				case 0:
					m.name = v
				case 1:
					m.desc = v
				case 2:
					m.ver = v
				case 3:
					m.author = v
				}
				m.step = m.firstMissing(m.step + 1)
				if m.step > m.maxStep {
					m.maxStep = m.step
				}
				m.err = ""
				if m.step < 4 {
					m.input = newWizardInput(m.step, m.name)
					return m, textinput.Blink
				}
				return m, nil
			}
		} else if m.step == 4 {
			switch {
			case key.Matches(msg, keys.Up):
				if m.cursor > 0 {
					m.cursor--
				}
			case key.Matches(msg, keys.Down):
				if m.cursor < len(surfaces)-1 {
					m.cursor++
				}
			case key.Matches(msg, keys.Toggle):
				m.chosen[m.cursor] = !m.chosen[m.cursor]
			case key.Matches(msg, keys.Confirm):
				hasSelection := false
				for _, v := range m.chosen {
					if v {
						hasSelection = true
						break
					}
				}
				if !hasSelection {
					return m, nil
				}
				if hasPanelChosen(m) {
					m.step = 8
					return m, nil
				}
				// target folder predates the run and wasn't confirmed yet?
				// gate here - before any file is touched
				if dirExistsWithFiles(m.name) && m.name != m.preConfirmed {
					m.step = 7
					m.owCursor = 0
					return m, nil
				}
				return m.scaffoldForOutro()
			}
			return m, nil
		} else if m.step == 8 {
			// panel mode selection (only when panel chosen)
			switch {
			case key.Matches(msg, keys.Up):
				if m.panelCursor > 0 {
					m.panelCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.panelCursor < 2 {
					m.panelCursor++
				}
			case key.Matches(msg, keys.Confirm):
				modes := []string{"attached", "window", "both"}
				m.panelMode = modes[m.panelCursor]
				if dirExistsWithFiles(m.name) && m.name != m.preConfirmed {
					m.step = 7
					m.owCursor = 0
					return m, nil
				}
				return m.scaffoldForOutro()
			}
			return m, nil
		} else if m.step == 7 {
			// overwrite confirmation (No/Yes selector)
			switch msg.Type {
			case tea.KeyLeft, tea.KeyUp:
				m.owCursor = 0
			case tea.KeyRight, tea.KeyDown:
				m.owCursor = 1
			case tea.KeyEnter:
				if m.owCursor == 0 {
					// No: cancel out with the recovery outro
					m.aborted = true
					m.step = 6
					m.frame = logoDoneFrame
					return m, tea.Quit
				}
				return m.scaffoldForOutro()
			}
			return m, nil
		}
	}
	if m.step < 4 {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m wizardModel) View() string {
	accent := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	fg := lipgloss.NewStyle().Foreground(hexColor(m.theme.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	bar := lipgloss.NewStyle().Foreground(hexColor(m.theme.muted))
	diamond := lipgloss.NewStyle().Foreground(hexColor(m.theme.accent)).Bold(true)
	check := lipgloss.NewStyle().Foreground(hexColor(m.theme.cyan)).Bold(true)

	var b strings.Builder
	// logo once at top - clack intro (animated entry)
	b.WriteString("\n")
	b.WriteString(omaLogoFrame(m.theme, m.frame))
	b.WriteString("\n\n")
	b.WriteString(bar.Render("┌  "))
	b.WriteString(accent.Render("oma create"))
	b.WriteString("\n")
	writeBarLine(&b, bar)

	// cancel outro - user exited (Ctrl+C / Esc / q)
	if m.step == 6 {
		history := wizardHistory(m)
		for _, h := range history {
			parts := strings.SplitN(h, ": ", 2)
			if len(parts) == 2 {
				writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
			} else {
				writeHistoryLine(&b, bar, check, muted, fg, "", h)
			}
		}
		writeBarLine(&b, bar)
		errStyle := lipgloss.NewStyle().Foreground(hexColor(m.theme.red)).Bold(true)
		b.WriteString(bar.Render("└  "))
		b.WriteString(errStyle.Render("✖ "))
		b.WriteString(fg.Render("Cancelled - no project was created"))
		// trailing newline: bubbletea's shutdown erases the cursor's line
		// ([2K); without it the cancelled message itself gets wiped.
		b.WriteString("\n")
		// subtle recovery command so answers are not lost
		if rec := recoveryCommand(m); rec != "" {
			b.WriteString("\n")
			b.WriteString(muted.Render("Pick up where you left off:"))
			b.WriteString("\n")
			b.WriteString(muted.Render("  "))
			b.WriteString(accent.Render(rec))
			b.WriteString("\n")
		}
		return b.String()
	}

	// overwrite confirmation - same box, asked before any file is touched
	if m.step == 7 {
		history := wizardHistory(m)
		for _, h := range history {
			parts := strings.SplitN(h, ": ", 2)
			if len(parts) == 2 {
				writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
			} else {
				writeHistoryLine(&b, bar, check, muted, fg, "", h)
			}
		}
		writeBarLine(&b, bar)
		warn := lipgloss.NewStyle().Foreground(hexColor(m.theme.red)).Bold(true)
		b.WriteString(diamond.Render("◆  "))
		b.WriteString(fg.Render(`directory "`) + warn.Render(filepath.Base(filepath.Clean(m.name))) + fg.Render(`/" already exists`))
		b.WriteString("\n")
		writeBarLine(&b, bar)
		descs := []string{"keep the existing folder untouched", "scaffold anyway and replace overlapping files"}
		for i, opt := range []string{"No", "Yes"} {
			mark := muted.Render("○")
			label := muted.Render(opt)
			if m.owCursor == i {
				mark = accent.Render("●")
				label = accent.Bold(true).Render(opt)
			}
			b.WriteString(bar.Render("│  ") + mark + " " + label + muted.Render("   "+descs[i]))
			b.WriteString("\n")
		}
		b.WriteString(bar.Render("│") + "\n")
		b.WriteString(bar.Render("└  ") + muted.Render("← → to choose · enter confirms · esc cancels"))
		return b.String()
	}

	// outro - all done, history + next steps (clack outro)
	if m.step == 5 {
		history := wizardHistory(m)
		// include surfaces as last history line
		var kinds []string
		for i, s := range surfaces {
			if m.chosen[i] {
				kinds = append(kinds, s)
			}
		}
		if len(kinds) > 0 {
			history = append(history, formatPrev("surfaces", strings.Join(kinds, ", ")))
		}
		for _, h := range history {
			parts := strings.SplitN(h, ": ", 2)
			if len(parts) == 2 {
				writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
			} else {
				writeHistoryLine(&b, bar, check, muted, fg, "", h)
			}
		}
		writeBarLine(&b, bar)
		if m.scaffoldErr != "" {
			errStyle := lipgloss.NewStyle().Foreground(hexColor(m.theme.red)).Bold(true)
			b.WriteString(errStyle.Render("✖ scaffold failed: " + m.scaffoldErr))
			b.WriteString("\n")
			return b.String()
		}
		// created files stay inside the box, above the closing line
		for _, f := range m.created {
			b.WriteString(bar.Render("│  ") + check.Render("● ") + fg.Render(f) + "\n")
		}
		if len(m.created) > 0 {
			writeBarLine(&b, bar)
		}
		b.WriteString(bar.Render("└  "))
		b.WriteString(check.Render("● "))
		b.WriteString(accent.Render("You're all set!"))
		b.WriteString("\n\n")
		b.WriteString(muted.Render("Next steps:"))
		b.WriteString("\n")
		b.WriteString(muted.Render("  cd "))
		b.WriteString(fg.Render(m.name))
		b.WriteString("\n")
		b.WriteString(muted.Render("  oma build    "))
		b.WriteString(muted.Render("- bundle JS/TS + generate the QML bridge"))
		b.WriteString("\n")
		b.WriteString(muted.Render("  oma install  "))
		b.WriteString(muted.Render("- install into Omarchy"))
		b.WriteString("\n")
		b.WriteString(muted.Render("  npx jsr add @oma/runtime  "))
		b.WriteString(muted.Render("- optional editor types for @oma/runtime"))
		b.WriteString("\n")
		b.WriteString(muted.Render("  npx skills add ussego/oma  "))
		b.WriteString(muted.Render("- agent skills for this project"))
		// trailing newline so shutdown's [2K erase can't wipe this line
		b.WriteString("\n")
		return b.String()
	}

	// panel mode selection - standalone step after surfaces
	if m.step == 8 {
		// show frozen surfaces history above the mode picker
		history := wizardHistory(m)
		var kinds []string
		for i, s := range surfaces {
			if m.chosen[i] {
				kinds = append(kinds, s)
			}
		}
		if len(kinds) > 0 {
			history = append(history, formatPrev("surfaces", strings.Join(kinds, ", ")))
		}
		for _, h := range history {
			parts := strings.SplitN(h, ": ", 2)
			if len(parts) == 2 {
				writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
			} else {
				writeHistoryLine(&b, bar, check, muted, fg, "", h)
			}
		}
		writeBarLine(&b, bar)
		b.WriteString(diamond.Render("◆  "))
		b.WriteString(fg.Render("panel mode"))
		b.WriteString(muted.Render(" - how the panel is presented"))
		b.WriteString("\n")
		writeBarLine(&b, bar)
		descs := map[string]string{
			"attached": "bar-anchored popup + widget (panel+bar-widget)",
			"window":   "draggable FloatingWindow, no widget (only-window)",
			"both":     "both: anchored + floating + widget (panel+bar-widget+window)",
		}
		for i, opt := range []string{"attached", "window", "both"} {
			mark := muted.Render("○")
			if m.panelCursor == i {
				mark = accent.Render("●")
			}
			raw := "  " + opt
			if m.panelCursor == i {
				raw = "› " + opt
			}
			label := fg.Render(fmt.Sprintf("%-14s", raw))
			if m.panelCursor == i {
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

	// history - frozen styled, not inputs (clack submitted)
	history := wizardHistory(m)
	for _, h := range history {
		parts := strings.SplitN(h, ": ", 2)
		if len(parts) == 2 {
			writeHistoryLine(&b, bar, check, muted, fg, parts[0], parts[1])
		} else {
			writeHistoryLine(&b, bar, check, muted, fg, "", h)
		}
	}
	if len(history) > 0 {
		writeBarLine(&b, bar)
	}

	if m.step < 4 {
		var label, hint string
		switch m.step {
		case 0:
			label = "project name"
			hint = "enter to continue · esc to quit"
		case 1:
			label = "description"
			hint = "enter to continue (empty = default) · esc to quit"
		case 2:
			label = "version"
			hint = "enter to continue (empty = default) · esc to quit"
		case 3:
			label = "author"
			hint = "enter to continue (empty = default) · esc to quit"
		}
		b.WriteString(diamond.Render("◆  "))
		b.WriteString(fg.Render(label))
		b.WriteString("\n")
		b.WriteString(bar.Render("│  "))
		b.WriteString(m.input.View())
		b.WriteString("\n")
		if m.err != "" {
			errStyle := lipgloss.NewStyle().Foreground(hexColor(m.theme.red))
			b.WriteString(bar.Render("│  "))
			b.WriteString(errStyle.Render("✖ " + m.err))
			b.WriteString("\n")
		}
		writeBarLine(&b, bar)
		b.WriteString(bar.Render("└  "))
		b.WriteString(muted.Render(hint))
	} else {
		b.WriteString(diamond.Render("◆  "))
		b.WriteString(fg.Render("select surfaces"))
		b.WriteString(muted.Render(" - " + m.name))
		b.WriteString("\n")
		writeBarLine(&b, bar)
		for i, s := range surfaces {
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
	}
	return b.String()
}

// writeBarLine writes a bare vertical bar row of the clack box.
func writeBarLine(b *strings.Builder, bar lipgloss.Style) {
	b.WriteString(bar.Render("│"))
	b.WriteString("\n")
}

// writeHistoryLine writes one frozen submitted answer inside the clack box.
// An empty label renders the raw value without the ": " separator.
func writeHistoryLine(b *strings.Builder, bar, check, muted, fg lipgloss.Style, label, value string) {
	b.WriteString(bar.Render("│  "))
	b.WriteString(check.Render("◇  "))
	if label != "" {
		b.WriteString(muted.Render(label + ": "))
	}
	b.WriteString(fg.Render(value))
	b.WriteString("\n")
}

// recoveryCommand rebuilds an equivalent non-interactive 'oma create' from
// the answers given before a cancel, so nothing the user typed is lost.
// Returns "" when there is nothing worth recovering (no name yet).
func recoveryCommand(m wizardModel) string {
	if m.name == "" {
		return ""
	}
	quote := func(v string) string {
		if strings.ContainsAny(v, " \t") {
			return `"` + v + `"`
		}
		return v
	}
	cmd := "oma create " + quote(m.name)
	if m.desc != "" {
		cmd += " -d " + quote(m.desc)
	}
	if m.author != "" {
		cmd += " -a " + quote(m.author)
	}
	if m.ver != "" {
		cmd += " -v " + quote(m.ver)
	}
	var kinds []string
	for i, s := range surfaces {
		if m.chosen[i] {
			kinds = append(kinds, s)
		}
	}
	if len(kinds) > 0 {
		cmd += " -s " + strings.Join(kinds, ",")
	}
	if hasPanelChosen(m) {
		modes := []string{"attached", "window", "both"}
		mode := m.panelMode
		if mode == "" && m.panelCursor < len(modes) {
			mode = modes[m.panelCursor]
		}
		if mode != "" && mode != "attached" {
			cmd += " --panel-mode " + mode
		}
	}
	return cmd
}

// wizardHistory lists the answered fields as frozen "label: value" lines.
// Visibility uses maxStep (how far the user actually got) rather than the
// current step, so cancelling from an early step doesn't display defaults
// for fields that were never reached; flag-seeded values always show.
func wizardHistory(m wizardModel) []string {
	var h []string
	if m.name != "" {
		h = append(h, formatPrev("project name", m.name))
	}
	if m.desc != "" || m.maxStep > 1 {
		effDesc := m.desc
		if effDesc == "" {
			effDesc = "A custom plugin for Omarchy - built with Oma"
		}
		h = append(h, formatPrev("description", effDesc))
	}
	if m.ver != "" || m.maxStep > 2 {
		effVer := m.ver
		if effVer == "" {
			effVer = "0.1.0"
		}
		h = append(h, formatPrev("version", effVer))
	}
	if m.author != "" || m.maxStep > 3 {
		if m.author != "" {
			h = append(h, formatPrev("author", m.author))
		}
	}
	return h
}

func isSurface(s string) bool {
	for _, k := range surfaces {
		if k == s {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func formatPrev(label, value string) string {
	return label + ": " + value
}

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*$`)
	versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+([\-+][0-9A-Za-z\-.]+)?$`)
	authorRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*$`)
)

func validateProjectName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	base := filepath.Base(filepath.Clean(name))
	if len(base) < 2 {
		return fmt.Errorf("at least 2 characters")
	}
	if len(base) > 40 {
		return fmt.Errorf("at most 40 characters")
	}
	lower := strings.ToLower(base)
	if !nameRe.MatchString(lower) {
		return fmt.Errorf("use lowercase letters, numbers and hyphens (e.g. my-plugin)")
	}
	if lower == "omarchy" || strings.HasPrefix(lower, "omarchy.") || strings.HasPrefix(lower, "omarchy-") {
		return fmt.Errorf("omarchy namespace is reserved")
	}
	if sanitize(base) == "" {
		return fmt.Errorf("project name is invalid")
	}
	return nil
}

func validateVersion(ver string) error {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return nil
	}
	if len(ver) > 20 {
		return fmt.Errorf("version too long (max 20)")
	}
	if !versionRe.MatchString(ver) {
		return fmt.Errorf("use semver like 0.1.0")
	}
	return nil
}

func validateAuthor(author string) error {
	author = strings.TrimSpace(author)
	if author == "" {
		return nil
	}
	if len(author) > 30 {
		return fmt.Errorf("author too long (max 30)")
	}
	lower := strings.ToLower(author)
	if !authorRe.MatchString(lower) {
		return fmt.Errorf("use lowercase letters, numbers and hyphens")
	}
	if lower == "omarchy" || strings.HasPrefix(lower, "omarchy.") || strings.HasPrefix(lower, "omarchy-") {
		return fmt.Errorf("omarchy namespace is reserved")
	}
	if sanitize(author) == "" {
		return fmt.Errorf("author is invalid")
	}
	return nil
}
