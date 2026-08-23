package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// runStatus prints a one-shot, read-only view of the project state: manifest
// identity, kinds/entry points, build freshness, installed-copy freshness,
// launcher-entry state and required tools. No mutations; exits 0 even when
// things are stale (informational), errors only when the manifest is missing.
func runStatus(dir string) error {
	m, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	cfg, cfgErr := loadOMAConfig(dir)

	t := currentTheme()
	label := lipgloss.NewStyle().Foreground(hexColor(t.muted)).Width(12)
	okMark := lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true).Render("●")
	badMark := lipgloss.NewStyle().Foreground(hexColor(t.red)).Bold(true).Render("✖")
	infoMark := lipgloss.NewStyle().Foreground(hexColor(t.accent)).Bold(true).Render("○")
	fg := lipgloss.NewStyle().Foreground(hexColor(t.foreground))
	muted := lipgloss.NewStyle().Foreground(hexColor(t.muted))

	line := func(mark, key, value string) {
		fmt.Println(label.Render(key) + mark + " " + value)
	}

	fmt.Println()
	line(infoMark, "project", fg.Render(m.ID+" - "+m.Name+" v"+m.Version))
	if m.Description != "" && m.Description != m.Name {
		fmt.Println(label.Render("") + muted.Render(m.Description))
	}

	// kinds + entry points
	var kindParts []string
	for _, k := range m.Kinds {
		kindParts = append(kindParts, fg.Render(k))
	}
	line(okMark, "kinds", strings.Join(kindParts, ", "))
	for _, ep := range []string{"bar", "barWidget", "menu", "panel", "overlay", "service"} {
		rel, ok := m.EntryPoints[ep]
		if !ok {
			continue
		}
		mark := okMark
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			mark = badMark
		}
		line(mark, "", muted.Render(ep+" → "+rel))
	}
	// panel mode (only when a panel surface is involved)
	hasPanelKind := false
	for _, k := range m.Kinds {
		if k == "panel" || k == "bar-widget" {
			// bar-widget may host an attached panel; show mode if oma.json has it set
			hasPanelKind = true
			break
		}
	}
	if hasPanelKind && cfgErr == nil && cfg.Panel != nil && cfg.Panel.Mode != "" && cfg.Panel.Mode != "attached" {
		line(infoMark, "panel", fg.Render(cfg.Panel.Mode))
	}
	if hasPanelKind && cfgErr == nil {
		// also surface extra file for both mode (not an entryPoint)
		if cfg.panelMode() == "both" {
			extra := "ui/PanelWindow.qml"
			mark := okMark
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(extra))); err != nil {
				mark = badMark
			}
			line(mark, "", muted.Render("panel-window → "+extra))
		}
	}

	// build freshness: ui artifacts vs newest source file. The bridge may
	// carry a collision-renamed or legacy name — probe what build writes.
	localBundle := filepath.Join(dir, "ui", "index.mjs")
	candidates := []string{
		filepath.Join(dir, "ui", bridgeBaseName(capitalize(m.Name), m.Kinds)+".qml"),
		filepath.Join(dir, "ui", bridgeBaseName(capitalize(m.Name), m.Kinds)+"Bridge.qml"),
		filepath.Join(dir, "ui", capitalize(m.Name)+".qml"), // legacy (pre-sanitizer) name
	}
	bridge := candidates[0]
	for _, c := range candidates[1:] {
		if _, err := os.Stat(c); err == nil {
			bridge = c
			break
		}
	}
	switch stale, missing := buildStale(dir, localBundle, bridge); {
	case missing:
		line(badMark, "build", "not built yet (run oma build)")
	case stale:
		line(badMark, "build", "stale - src changed since last build")
	default:
		line(okMark, "build", "fresh")
	}

	// installed copy
	dest, destErr := pluginDest(dir)
	if destErr != nil {
		line(badMark, "installed", destErr.Error())
	} else if _, err := os.Stat(dest); os.IsNotExist(err) {
		line(badMark, "installed", "not installed (oma install)")
	} else if _, err := os.Stat(filepath.Join(dest, "ui", "index.mjs")); os.IsNotExist(err) {
		line(badMark, "installed", muted.Render(dest)+" - incomplete install (oma install)")
	} else if installedStale(localBundle, filepath.Join(dest, "ui", "index.mjs")) {
		line(badMark, "installed", muted.Render(dest)+" - older than local build")
	} else {
		line(okMark, "installed", dest)
	}

	// launcher entries
	if cfgErr != nil {
		line(badMark, "launcher", cfgErr.Error())
	} else if len(cfg.Launchers) == 0 {
		line(infoMark, "launcher", muted.Render("none declared (add a launchers[] array to oma.json)"))
	} else {
		apps, _ := applicationsDir()
		for i, def := range cfg.Launchers {
			filename, _ := renderLauncherEntry(m, def, cfg.Icon, i, dir, false)
			path := filepath.Join(apps, filename)
			if _, err := os.Stat(path); err != nil {
				line(badMark, "", path+" - not created")
			} else {
				line(okMark, "", path)
			}
		}
	}

	// editor lib (@oma/runtime from jsr) — optional, but drift is misleading
	libVersion := installedLibVersion(dir)
	switch {
	case libVersion == "":
		line(infoMark, "lib", muted.Render("not installed (optional: npx jsr add @oma/runtime)"))
	case libVersion != version():
		line(badMark, "lib", "@oma/runtime "+libVersion+muted.Render(" - cli is "+version()+" (npx jsr add @oma/runtime)"))
	default:
		line(okMark, "lib", "@oma/runtime "+libVersion)
	}

	// tools
	line(toolMark("omarchy-shell"), "tools", "omarchy-shell"+toolNote("omarchy-shell"))
	line(toolMark("omarchy"), "", "omarchy"+toolNote("omarchy"))
	fmt.Println()
	return nil
}

// installedLibVersion reads the version of the locally installed runtime lib
// (node_modules/@oma/runtime), "" when absent.
func installedLibVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "node_modules", "@oma", "runtime", "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Version
}

func toolMark(bin string) string {
	t := currentTheme()
	mark := lipgloss.NewStyle().Foreground(hexColor(t.red)).Bold(true).Render("✖")
	if _, err := exec.LookPath(bin); err == nil {
		mark = lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true).Render("●")
	}
	return mark
}

func toolNote(bin string) string {
	if _, err := exec.LookPath(bin); err == nil {
		return ""
	}
	return " missing"
}

// buildStale reports whether the bundle or bridge is missing, or older than
// the newest file under src/ (plus the two config files that shape output).
func buildStale(dir, bundle, bridge string) (stale bool, missing bool) {
	bundleInfo, err1 := os.Stat(bundle)
	bridgeInfo, err2 := os.Stat(bridge)
	if err1 != nil || err2 != nil {
		return false, true
	}
	newest := newerModTime(bundleInfo.ModTime(), bridgeInfo.ModTime())
	srcNewest, err := newestSrcModTime(filepath.Join(dir, "src"))
	if err != nil {
		return false, false // can't tell → don't claim stale
	}
	for _, extra := range []string{
		filepath.Join(dir, "manifest.json"),
		filepath.Join(dir, "oma.json"),
	} {
		if info, err := os.Stat(extra); err == nil {
			newest = newerModTime(newest, info.ModTime())
		}
	}
	return srcNewest.After(newest), false
}

func newestSrcModTime(srcDir string) (time.Time, error) {
	var newest time.Time
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			newest = newerModTime(newest, info.ModTime())
		}
		return nil
	})
	return newest, err
}

// installedStale reports whether the installed artifact is older than the
// local one — or missing entirely while the local copy exists (a half-deleted
// install must not read as fresh).
func installedStale(local, installed string) bool {
	l, err1 := os.Stat(local)
	i, err2 := os.Stat(installed)
	if err1 != nil {
		return false // nothing built locally to compare against
	}
	if err2 != nil {
		return true
	}
	return l.ModTime().After(i.ModTime())
}

func newerModTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
