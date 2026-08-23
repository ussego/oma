package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type theme struct {
	accent     string
	foreground string
	background string
	muted      string
	cyan       string
	magenta    string
	blue       string
	red        string
}

var colorLine = regexp.MustCompile(`^([a-z_]+)\s*=\s*"?#?([0-9a-fA-F]{6})"?`)

// currentTheme reads the active Omarchy theme's colors.toml so the TUI follows
// the system theme. It pulls the important palette colors (accent, foreground,
// muted, plus cyan/magenta/blue for the logo) from ~/.local/state/omarchy/current/theme/colors.toml.
// Falls back to neutral grays if no theme is resolved.
func currentTheme() theme {
	t := theme{accent: "a0a0a0", foreground: "a5a5a5", background: "191919", muted: "666666", cyan: "a0a0a0", magenta: "a0a0a0", blue: "a0a0a0", red: "f38ba8"}
	home, err := os.UserHomeDir()
	if err != nil {
		return t
	}
	link := filepath.Join(home, ".local", "state", "omarchy", "current", "theme")
	dir, err := filepath.EvalSymlinks(link)
	if err != nil {
		return t
	}
	f, err := os.Open(filepath.Join(dir, "colors.toml"))
	if err != nil {
		return t
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := colorLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		switch m[1] {
		case "accent":
			t.accent = m[2]
		case "foreground":
			t.foreground = m[2]
		case "background":
			t.background = m[2]
		case "muted":
			t.muted = m[2]
		case "cyan", "bright_cyan":
			if t.cyan == "a0a0a0" {
				t.cyan = m[2]
			}
		case "magenta", "bright_magenta":
			if t.magenta == "a0a0a0" {
				t.magenta = m[2]
			}
		case "blue", "bright_blue":
			if t.blue == "a0a0a0" {
				t.blue = m[2]
			}
		case "red", "bright_red":
			if t.red == "f38ba8" { // default sentinel: first red found wins
				t.red = m[2]
			}
		}
	}
	// fill palette fallbacks from important colors if still missing
	if t.cyan == "a0a0a0" {
		t.cyan = t.accent
	}
	if t.magenta == "a0a0a0" {
		t.magenta = t.accent
	}
	if t.blue == "a0a0a0" {
		t.blue = t.accent
	}
	if t.red == "a0a0a0" {
		t.red = "f38ba8"
	}
	return t
}

func hexColor(s string) lipgloss.Color {
	if strings.HasPrefix(s, "#") {
		return lipgloss.Color(s)
	}
	return lipgloss.Color("#" + s)
}

// omaLogo returns the block logo colored with the theme's palette: accent,
// cyan, magenta (each letter distinct) + muted tagline on the right, 2 lines
// to avoid layout breaks.
func omaLogo(t theme) string {
	accent := lipgloss.NewStyle().Foreground(hexColor(t.accent)).Bold(true)
	cyanStyle := lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true)
	magentaStyle := lipgloss.NewStyle().Foreground(hexColor(t.magenta)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(hexColor(t.muted))

	//  █▀█ █▀▄▀█ █▀█  one project
	//  █▄█ █ ▀ █ █▀█  multiple surfaces
	logo1 := accent.Render(" █▀█") + " " + cyanStyle.Render("█▀▄▀█") + " " + magentaStyle.Render("█▀█")
	logo2 := accent.Render(" █▄█") + " " + cyanStyle.Render("█ ▀ █") + " " + magentaStyle.Render("█▀█")

	line1 := logo1 + "  " + muted.Render("one project")
	line2 := logo2 + "  " + muted.Render("multiple surfaces")
	return line1 + "\n" + line2
}

// Logo entry animation: each row slides in as a whole block from the left
// edge (left-clipped while traveling) and parks at home, top row first,
// bottom row right behind it. Once both rows landed, the tagline reveals
// left-to-right in muted. Frames advance via tickMsg in create.go.
const (
	slideDist     = 10 // columns of travel per row
	rowFrames     = 6  // travel time per row
	rowStagger    = 3  // bottom row starts this many frames later
	tagStartFrame = 9  // tagline reveal begins (both rows landed)
	logoDoneFrame = 16 // tagline fully revealed; static logo from here
)

var logoRows = [2]string{" █▀█ █▀▄▀█ █▀█", " █▄█ █ ▀ █ █▀█"}
var logoTaglines = [2]string{"one project", "multiple surfaces"}

// finalBlockColor returns the color for a block column: the three letter
// groups map to accent / cyan / magenta, matching omaLogo.
// Row layout: cols 1-3 = "o", 5-9 = "m", 11-13 = "a".
func finalBlockColor(t theme, ci int) string {
	switch {
	case ci <= 4:
		return t.accent
	case ci <= 10:
		return t.cyan
	default:
		return t.magenta
	}
}

// omaLogoFrame renders the logo at the given animation frame. Frames >=
// logoDoneFrame return the settled static logo.
func omaLogoFrame(t theme, frame int) string {
	if frame >= logoDoneFrame {
		return omaLogo(t)
	}
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(hexColor(t.muted))

	var b strings.Builder
	for ri, row := range logoRows {
		p := frame - ri*rowStagger
		if p <= 0 {
			// row not launched yet
			b.WriteString("\n")
			continue
		}
		if p > rowFrames {
			p = rowFrames // parked
		}
		// slice by rune: block glyphs are multi-byte
		rr := []rune(row)
		shift := slideDist * (rowFrames - p) / rowFrames
		for x, r := range rr[shift:] {
			ci := shift + x
			if r == ' ' {
				b.WriteString(" ")
				continue
			}
			col := hexColor(finalBlockColor(t, ci))
			b.WriteString(lipgloss.NewStyle().Foreground(col).Inherit(bold).Render(string(r)))
		}
		// tagline reveals left-to-right once both rows landed
		if frame >= tagStartFrame {
			b.WriteString("  ")
			reveal := (frame - tagStartFrame) * 3
			for ti, tr := range logoTaglines[ri] {
				if ti >= reveal {
					break
				}
				b.WriteString(muted.Render(string(tr)))
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
