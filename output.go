package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Shared result-output vocabulary for the mutating commands (build, package,
// install, ...). Every accomplished fact gets a cyan ● line, the run ends with
// a plain "<verb> in <duration>" summary, and a muted "next:" hint appears
// when another step is needed. Warnings stay on stderr as plain text.

func markOK() lipgloss.Style {
	t := currentTheme()
	return lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true)
}

func mutedStyle() lipgloss.Style {
	t := currentTheme()
	return lipgloss.NewStyle().Foreground(hexColor(t.muted))
}

// okLine prints one accomplished fact: "● <thing>".
func okLine(msg string) {
	fmt.Println(markOK().Render("● ") + msg)
}

// noteLine prints an informational non-action line (nothing was changed).
func noteLine(msg string) {
	fmt.Println(mutedStyle().Render(msg))
}

// nextHint points at the logical follow-up command.
func nextHint(cmd string) {
	fmt.Println(mutedStyle().Render("next: ") + cmd)
}

// doneLine closes a mutating command with its wall-clock cost.
func doneLine(verb string, d time.Duration) {
	fmt.Printf("%s in %s\n", verb, fmtDur(d))
}

// fmtDur renders durations the way build tools do: ms below a second.
func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// humanSize formats byte counts for artifact lines (esbuild-style kB).
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// displayPath shortens a path for humans: relative to the working directory
// when it lives underneath it, otherwise with $HOME abbreviated to ~.
func displayPath(p string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, p); err == nil &&
			rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
			return "~/" + filepath.ToSlash(rest)
		}
	}
	return p
}
