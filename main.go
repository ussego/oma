package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "--version" || os.Args[1] == "version" || os.Args[1] == "-V") {
		fmt.Println("oma " + version())
		return
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	// per-command help: oma <command> --help / -h
	cmd := os.Args[1]
	for _, a := range os.Args[2:] {
		if a == "-h" || a == "--help" {
			if _, ok := cmdHelp[cmd]; ok {
				printCmdHelp(cmd)
				os.Exit(0)
			}
			break
		}
	}

	arg := func() string {
		if len(os.Args) > 2 {
			return os.Args[2]
		}
		return "."
	}

	switch os.Args[1] {
	case "create":
		if err := runCreate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "surface":
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		if sub == "" || sub == "-h" || sub == "--help" || sub == "help" {
			printCmdHelp("surface")
			os.Exit(0)
		}
		if sub != "add" {
			fmt.Fprintf(os.Stderr, "error: unknown surface command %q (use 'oma surface add')\n", sub)
			os.Exit(2)
		}
		// runs inside the project dir; args are the kinds to add
		if err := runSurfaceAdd(".", os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "log":
		if err := runLog(".", os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "tail":
		// alias for `oma log -f`
		if err := runLog(".", append(os.Args[2:], "--follow")); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "launcher":
		sub := ""
		if len(os.Args) > 2 {
			sub = os.Args[2]
		}
		if sub == "" || sub == "-h" || sub == "--help" || sub == "help" {
			printCmdHelp("launcher")
			os.Exit(0)
		}
		dir := "."
		switch sub {
		case "add":
			// upserts the entry into oma.json (wizard when no name is given),
			// then materializes every declared entry
			if err := runLauncherAdd(dir, os.Args[3:]); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		case "remove":
			// drops selected entries from oma.json and realigns .desktop files
			if err := runLauncherRemove(dir, os.Args[3:]); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "error: unknown launcher command %q (use add or remove)\n", sub)
			os.Exit(2)
		}
	case "validate":
		if err := validate(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "build":
		if err := build(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "package":
		if err := packageDir(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "install":
		if err := install(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "uninstall":
		if err := uninstall(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "restart", "r":
		if err := restart(arg()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "skills":
		err := runSkills(os.Args[2:])
		if err != nil {
			if _, ok := err.(unknownSkill); ok {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}
	case "help", "--help", "-h":
		usage()
		os.Exit(0)
	default:
		usage()
		os.Exit(2)
	}
}

// cliVersion is set by release builds via
// -ldflags "-X main.cliVersion=X.Y.Z" (no v prefix). When not injected
// (go install, go build, go test), version() falls back to the module
// version embedded by the toolchain, or "dev" for local builds.
var cliVersion = ""

// version returns the CLI version: the ldflags-injected value when present,
// otherwise the module version recorded by the toolchain (go install pkg@vX
// embeds it), otherwise "dev" for local builds.
func version() string {
	if cliVersion != "" {
		return cliVersion
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		return versionFromBuildInfo(bi)
	}
	return "dev"
}

// versionFromBuildInfo maps a build info's main module version to a display
// version. The v prefix is stripped so go install output matches the injected
// release format ("0.1.2", not "v0.1.2").
func versionFromBuildInfo(bi *debug.BuildInfo) string {
	if bi == nil {
		return "dev"
	}
	v := bi.Main.Version
	if v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	return "dev"
}

// cmdHelp holds the per-command help shown by 'oma <command> --help',
// structured like bun's subcommand help pages.
type helpFlag struct {
	short, long, val, desc string
}

var cmdHelp = map[string]struct {
	args  string     // usage arg spec after [flags], e.g. "[dir]"
	alias string     // short alias shown on the Alias: line
	flags []helpFlag // extra flags beyond -h/--help
	desc  string     // indented description paragraphs
}{
	"create": {
		args: "[name] [kinds...]",
		flags: []helpFlag{
			{"-s", "--surfaces", "=<val>", "Comma-separated kinds to scaffold (e.g. panel,overlay)"},
			{"-a", "--author", "=<val>", "Author namespace for the plugin id (<author>.<name>)"},
			{"-d", "--description", "=<val>", "Manifest description"},
			{"-v", "--version", "=<val>", "Semver version (default 0.1.0, flag-only)"},
			{"", "--panel-mode", "=<val>", "Panel presentation: attached, window or both (default attached)"},
		},
		desc: "Scaffold a new oma project (interactive wizard).\n\nAny flag you pass is skipped in the wizard, so this runs\nfully non-interactive when everything is provided:\n\noma create omusic -s panel,overlay -a ussego -d \"Music plugin\"\noma create omusic -s panel --panel-mode window\noma create omusic -s panel --panel-mode attached   # panel+bar-widget (default)\noma create omusic -s panel --panel-mode window      # only-window (no bar widget)\noma create omusic -s panel,bar-widget --panel-mode window  # window+bar-widget\noma create omusic -s panel --panel-mode both        # panel+bar-widget+window\n\nThe finish screen also prints 'npx skills add ussego/oma' to install the\nagent skills that teach plugin development.",
	},
	"surface": {
		args: "add [kinds...]",
		desc: "Add surfaces to an existing project (run inside the project dir).\n\nUpdates manifest.json kinds + entryPoints and generates the\nmissing ui/<Kind>.qml skeletons. Already-present kinds are skipped.\n\noma surface add panel,overlay   # explicit kinds\noma surface add panel --panel-mode window      # only-window\noma surface add panel --panel-mode attached     # panel+bar-widget\noma surface add panel --panel-mode both         # panel+bar-widget+window\noma surface add bar-widget                      # add status widget to existing window\noma surface add                 # interactive multi-select (prompts for panel mode)",
	},
	"status": {
		args: "[dir]",
		desc: "One-shot, read-only project state.\n\nShows id/version/kinds/entry points, whether the build is fresh\nor stale, the installed copy state, launcher entries and tools.",
	},
	"log": {
		args: "[flags]",
		flags: []helpFlag{
			{"-n", "--lines", "=<n>", "Lines to show (default 100, 0 = all)"},
			{"-f", "--follow", "", "Keep streaming new lines"},
			{"", "--all", "", "Show the whole shell log, not just this plugin"},
			{"", "--level", "=<lvl>", "Minimum level: debug, info, warn or error"},
			{"", "--json", "", "One JSON object per line (for agents)"},
			{"", "--pid", "=<pid>", "omarchy-shell process id (auto-detected)"},
		},
		desc: "Show log lines from the running omarchy shell, filtered to this\nplugin (run inside the project dir). Pretty and theme-aware by\ndefault; --json switches to one object per line for agents.\n\noma log                  # last 100 lines mentioning this plugin\noma log --all -n 500     # everything, last 500 lines\noma log --level warn     # only warnings and errors\noma log -f               # follow (same as: oma tail)\noma log --json           # machine-readable stream\n\nIn follow mode, identical lines arriving within a second collapse into one\nline with a live (xN) count; repeats after that print again (a recurring\nerror is never hidden). One-shot output is always lossless. Follow mode also\nsurvives shell restarts (oma restart / session restarts) by reconnecting.",
	},
	"tail": {
		args: "[flags]",
		desc: "Follow the shell log (alias for `oma log -f`).",
	},
	"launcher": {
		args: "add [name] | remove [names | --all]",
		desc: "Add or remove launcher entries for this plugin.\n\n'launcher add' upserts an entry into oma.json launchers[] and writes\n~/.local/share/applications/<id>.desktop. Without a name it asks one\nthing at a time - name, then command (real presets or your own), then\nan optional icon (blank = inherit). Esc cancels.\n\noma launcher add \"Play Next\" --action toggle --icon media-play\noma launcher add \"Open\" --exec \"my-cmd --flag\"\n\n'oma install' creates entries automatically. 'remove' drops entries\nfrom oma.json (name args, --all, or multi-select wizard) and\ndeletes only oma-managed .desktop files.\n\noma.json shape (editors autocomplete via the $schema stub):\n{\n  \"icon\": \"utilities-system-monitor\",\n  \"launchers\": [\n    { \"name\": \"Play\", \"action\": \"summon|toggle|hide\",\n      \"exec\": \"\", \"icon\": \"\" }\n  ]\n}",
	},
	"validate": {args: "[dir]", desc: "Validate the built plugin with Omarchy.\n\nShells out to: omarchy plugin validate <dir>"},
	"build": {
		args: "[dir]",
		desc: "Bundle JS/TS + generate the QML bridge.\n\nsrc/index.js (or src/index.ts) is bundled to ui/index.mjs\n(esbuild, ES2016 for QJSEngine; TS annotations are stripped) and\nthe bridge QtObject is generated next to it. Run once before\ninstalling.",
	},
	"package":   {args: "[dir]", desc: "Assemble an installable plugin directory.\n\nCopies manifest.json + ui/ into pkg/ and validates it."},
	"install":   {args: "[dir]", desc: "Copy the plugin into ~/.config/omarchy/plugins/<id>/.\n\nThe running shell hot-reloads plugins from that directory."},
	"uninstall": {args: "[dir]", desc: "Delete the plugin from ~/.config/omarchy/plugins/<id>/."},
	"restart": {
		args:  "[dir]",
		alias: "r",
		desc:  "Install + restart the shell in one step.\n\nEquivalent to: oma install && omarchy restart shell",
	},
	"skills": {args: "list | get <name>", desc: "Print or list bundled documentation skills."},
}

func printCmdHelp(cmd string) {
	h := cmdHelp[cmd]
	t := currentTheme()
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(hexColor(t.muted))
	plain := lipgloss.NewStyle().Foreground(hexColor(t.foreground))
	cyan := lipgloss.NewStyle().Foreground(hexColor(t.cyan))
	blue := lipgloss.NewStyle().Foreground(hexColor(t.blue))
	accent := lipgloss.NewStyle().Foreground(hexColor(t.accent)).Bold(true)

	var b strings.Builder
	// Usage line: bun renders program+command bold-colored, [flags] cyan,
	// positional args blue.
	b.WriteString(bold.Render("Usage: "))
	b.WriteString(accent.Render("oma " + cmd))
	b.WriteString(cyan.Render(" [flags]"))
	if h.args != "" {
		b.WriteString(" ")
		b.WriteString(blue.Render(h.args))
	}
	b.WriteString("\n")
	if h.alias != "" {
		b.WriteString(bold.Render("Alias: "))
		b.WriteString(accent.Render("oma " + h.alias))
		b.WriteString("\n")
	}
	// description paragraphs, indented two spaces like bun
	b.WriteString("\n")
	for _, line := range strings.Split(h.desc, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("  ")
		b.WriteString(plain.Render(line))
		b.WriteString("\n")
	}
	// flags section
	b.WriteString("\n")
	b.WriteString(bold.Render("Flags:"))
	b.WriteString("\n")
	flagRows := append([]helpFlag{}, h.flags...)
	flagRows = append(flagRows, helpFlag{short: "-h", long: "--help", desc: "Print this help menu"})
	for _, f := range flagRows {
		label := "    "
		if f.short != "" {
			label = "  " + f.short + ", "
		}
		b.WriteString(label)
		b.WriteString(cyan.Render(f.long))
		if f.val != "" {
			b.WriteString(dim.Render(f.val))
		}
		pad := 30 - len(label) - len(f.long) - len(f.val)
		if pad < 2 {
			pad = 2
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(plain.Render(f.desc))
		b.WriteString("\n")
	}

	fmt.Print(b.String())
}

func usage() {
	t := currentTheme()
	// bun renders with bold + a few accents; we map those roles onto the
	// Omarchy theme palette so colors follow the active theme.
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(hexColor(t.muted))
	plain := lipgloss.NewStyle().Foreground(hexColor(t.foreground))
	groupColor := map[string]lipgloss.Style{
		"accent":  lipgloss.NewStyle().Foreground(hexColor(t.accent)).Bold(true),
		"blue":    lipgloss.NewStyle().Foreground(hexColor(t.blue)).Bold(true),
		"magenta": lipgloss.NewStyle().Foreground(hexColor(t.magenta)).Bold(true),
		"cyan":    lipgloss.NewStyle().Foreground(hexColor(t.cyan)),
		"cyanB":   lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true),
	}

	type entry struct {
		cmd, example, alias, desc string
		exBold                    bool // example rendered bold-cyan (--help)
	}
	var b strings.Builder
	b.WriteString(groupColor["accent"].Render("oma"))
	b.WriteString(plain.Render(" is an SDK for Omarchy plugins - one project, shared logic, multiple surfaces. "))
	b.WriteString(dim.Render("(" + version() + ")"))
	b.WriteString("\n\n")
	b.WriteString(bold.Render("Usage: "))
	b.WriteString(bold.Render("oma <command> "))
	b.WriteString(groupColor["cyan"].Render("[...flags]"))
	b.WriteString(bold.Render(" [...args]"))
	b.WriteString("\n\n")
	b.WriteString(bold.Render("Commands:"))
	b.WriteString("\n")

	// one accent color per functional group, like bun
	groups := []struct {
		color string
		rows  []entry
	}{
		{
			color: "accent",
			rows: []entry{
				{cmd: "create", example: "my-plugin panel", desc: "Scaffold a new project with surfaces"},
				{cmd: "surface add", example: "panel,overlay", desc: "Add surfaces to an existing project"},
				{cmd: "build", example: "[dir]", desc: "bundle JS + generate the QML bridge"},
				{cmd: "package", example: "[dir]", desc: "assemble + validate pkg/"},
				{cmd: "validate", example: "[dir]", desc: "omarchy plugin validate"},
			},
		},
		{
			color: "blue",
			rows: []entry{
				{cmd: "status", example: "[dir]", desc: "Project state at a glance"},
				{cmd: "log", example: "[flags]", desc: "Plugin logs from the running shell"},
				{cmd: "tail", example: "[flags]", desc: "Follow logs (alias for log -f)"},
				{cmd: "install", example: "[dir]", desc: "Copy the plugin into ~/.config/omarchy/plugins/"},
				{cmd: "launcher add", example: "[name]", desc: "Create launcher entries from oma.json"},
				{cmd: "uninstall", example: "[dir]", desc: "Delete it from ~/.config/omarchy/plugins/"},
				{cmd: "restart", example: "[dir]", alias: "oma r", desc: "Install + restart the shell"},
			},
		},
		{
			color: "cyanB",
			rows: []entry{
				{cmd: "skills", example: "list", desc: "List available skills"},
				{cmd: "skills", example: "get <name>", desc: "Print a skill's content"},
			},
		},
		{
			color: "dim",
			rows: []entry{
				{cmd: "<command>", example: "--help", desc: "Print help text for command.", exBold: true},
			},
		},
	}
	cmdWidth, exWidth := 0, 0
	for _, g := range groups {
		for _, c := range g.rows {
			if len(c.cmd) > cmdWidth {
				cmdWidth = len(c.cmd)
			}
			if len(c.example) > exWidth {
				exWidth = len(c.example)
			}
		}
	}
	groupColor["dim"] = dim
	writeRows := func(color string, rows []entry) {
		st := groupColor[color]
		for _, c := range rows {
			b.WriteString("  ")
			b.WriteString(st.Render(c.cmd + strings.Repeat(" ", cmdWidth-len(c.cmd))))
			b.WriteString("  ")
			if c.example != "" {
				if c.exBold {
					b.WriteString(groupColor["cyanB"].Render(c.example + strings.Repeat(" ", exWidth-len(c.example))))
				} else {
					b.WriteString(dim.Render(c.example + strings.Repeat(" ", exWidth-len(c.example))))
				}
			} else {
				b.WriteString(strings.Repeat(" ", exWidth))
			}
			b.WriteString("  ")
			b.WriteString(plain.Render(c.desc))
			if c.alias != "" {
				b.WriteString(dim.Render(" (" + c.alias + ")"))
			}
			b.WriteString("\n")
		}
	}
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		writeRows(g.color, g.rows)
	}

	b.WriteString("\n")
	b.WriteString(bold.Render("Flags:"))
	b.WriteString("\n")
	flags := []struct{ short, long, desc string }{
		{"-h", "--help", "Display this menu and exit"},
	}
	for _, f := range flags {
		b.WriteString("  ")
		if f.short != "" {
			b.WriteString(groupColor["cyan"].Render(f.short))
			b.WriteString(", ")
		} else {
			b.WriteString("    ")
		}
		b.WriteString(groupColor["cyan"].Render(f.long))
		b.WriteString(strings.Repeat(" ", 26-len(f.long)))
		b.WriteString(plain.Render(f.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dim.Render("(more docs in 'oma skills list' and 'oma skills get <name>')"))
	b.WriteString("\n\n")
	b.WriteString(plain.Render("Learn more about oma:      "))
	b.WriteString(groupColor["cyan"].Render("'oma skills get oma'"))

	fmt.Print(b.String())
}
