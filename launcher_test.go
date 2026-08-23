package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOMAConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "oma.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLauncherEntriesGolden(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "omusic")
	if _, err := scaffoldWithOptions(target, []string{"panel", "overlay"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = removeLauncherEntries(target) })

	writeOMAConfig(t, target, `{
	  "icon": "utilities-system-monitor",
	  "launchers": [
	    {"name": "Play Next Music", "action": "toggle"},
	    {"name": "Music Settings",  "icon": "audio-card", "comment": "Open settings"}
	  ]
	}`)

	written, err := writeLauncherEntries(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("written = %v", written)
	}

	first := readTrim(t, written[0])
	for _, want := range []string{
		"[Desktop Entry]",
		"Name=Play Next Music",
		"Exec=omarchy-shell shell toggle tester.omusic",
		"TryExec=omarchy-shell",
		"Icon=utilities-system-monitor",
		"GenericName=Bar widget",
		"Comment=A custom plugin for Omarchy - built with Oma",
		managedMarker,
	} {
		if !strings.Contains(first, want) {
			t.Errorf("entry 1 missing %q\n%s", want, first)
		}
	}
	if filepath.Base(written[0]) != "tester.omusic.desktop" {
		t.Errorf("first filename = %s", filepath.Base(written[0]))
	}

	second := readTrim(t, written[1])
	if !strings.Contains(second, "Exec=omarchy-shell shell summon tester.omusic") {
		t.Errorf("summon stays default while a summonable kind exists:\n%s", second)
	}
	if !strings.Contains(second, "Icon=audio-card") {
		t.Errorf("per-entry icon override failed:\n%s", second)
	}
	if !strings.Contains(filepath.Base(written[1]), "tester.omusic-music-settings") {
		t.Errorf("slugged filename = %s", filepath.Base(written[1]))
	}

	// remove only oma-managed files
	removed, err := removeLauncherEntries(target)
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed = %v err = %v", removed, err)
	}
	for _, p := range removed {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", p)
		}
	}
}

func TestLauncherForeignEntryUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"overlay"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	writeOMAConfig(t, target, `{"launchers":[{"name":"App"}]}`)
	if _, err := writeLauncherEntries(target); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	apps, _ := applicationsDir()
	foreign := filepath.Join(apps, m.ID+".desktop")
	marker := readTrim(t, foreign)
	if err := os.WriteFile(foreign, []byte(strings.ReplaceAll(marker, managedMarker, "# not managed")), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeLauncherEntries(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed foreign file: %v", removed)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("foreign entry was deleted")
	}
}

func TestStatusFixture(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "st")
	if err := scaffold(target, []string{"panel"}); err != nil {
		t.Fatal(err)
	}
	if err := runStatus(target); err != nil {
		t.Fatal(err)
	}
	// missing manifest must error
	if err := runStatus(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestLoadOMAConfigValidation(t *testing.T) {
	dir := t.TempDir()
	writeOMAConfig(t, dir, `{"launchers":[{"action":"fly"}]}`)
	if _, err := loadOMAConfig(dir); err == nil {
		t.Fatal("expected unknown-action error")
	}
	writeOMAConfig(t, dir, `{"launchers":[{}]}`)
	if _, err := loadOMAConfig(dir); err == nil {
		t.Fatal("expected missing-name error")
	}
	writeOMAConfig(t, dir, `{not json`)
	if _, err := loadOMAConfig(dir); err == nil {
		t.Fatal("expected parse error")
	}
	writeOMAConfig(t, dir, `{"icon":"x"}`)
	c, err := loadOMAConfig(dir)
	if err != nil || c.Icon != "x" || len(c.Launchers) != 0 {
		t.Fatalf("valid config misread: %+v %v", c, err)
	}
}

func readTrim(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLauncherHelpers(t *testing.T) {
	slugCases := map[string]string{
		"Play Next Music": "play-next-music",
		"Already-Lower":   "already-lower",
		"  A--B  ":        "a-b",
		"UPPER_case!":     "upper-case",
		"Quoted 'Name'":   "quoted-name",
	}
	for in, want := range slugCases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}

	kindCases := []struct {
		kinds []string
		want  string
	}{
		{[]string{"panel", "overlay"}, "Panel"},
		{[]string{"overlay"}, "Overlay"},
		{[]string{"menu"}, "Launcher"},
		{[]string{"bar-widget"}, "Bar widget"},
		{[]string{"bar"}, "Status bar"},
		{[]string{"service"}, "Service"},
		{nil, "Plugin"},
	}
	for _, c := range kindCases {
		if got := kindLabel(c.kinds); got != c.want {
			t.Errorf("kindLabel(%v) = %q, want %q", c.kinds, got, c.want)
		}
	}

	if got := defaultLauncherAction([]string{"panel"}); got != "summon" {
		t.Errorf("summonable kinds should default to summon, got %q", got)
	}
	if got := defaultLauncherAction([]string{"bar-widget"}); got != "toggle" {
		t.Errorf("bar-widget-only should default to toggle, got %q", got)
	}
	if !summonable([]string{"panel"}) || !summonable([]string{"overlay"}) || summonable([]string{"service"}) {
		t.Error("summonable misclassifies kinds")
	}
	if got := firstNonEmpty("", "x", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q", got)
	}
}

func TestLauncherDefaultToggleForBarWidgetOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "onlybar")
	if _, err := scaffoldWithOptions(target, []string{"bar-widget"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = removeLauncherEntries(target) })
	writeOMAConfig(t, target, `{"launchers":[{"name":"Only Bar"}]}`)
	written, err := writeLauncherEntries(target)
	if err != nil || len(written) != 1 {
		t.Fatalf("written=%v err=%v", written, err)
	}
	if got := readTrim(t, written[0]); !strings.Contains(got, "Exec=omarchy-shell shell toggle ") {
		t.Fatalf("expected toggle default:\n%s", got)
	}
}

func TestLauncherIconMaterialization(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "omusic")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = removeLauncherEntries(target) })

	// project-relative file icon (svg -> scalable/apps) and a remote URL
	iconsDir := filepath.Join(target, "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`
	if err := os.WriteFile(filepath.Join(iconsDir, "logo.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	writeOMAConfig(t, target, `{
	  "launchers": [
	    {"name": "Local App", "action": "summon", "icon": "icons/logo.svg"},
	    {"name": "Remote App", "action": "summon", "icon": "`+srv.URL+`/icon.png"},
	    {"name": "Theme App", "action": "summon", "icon": "audio-card"}
	  ]
	}`)

	written, err := writeLauncherEntries(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 3 {
		t.Fatalf("written = %v", written)
	}

	// .desktop Icon= lines reference the materialized theme names
	contents := ""
	for _, p := range written {
		contents += readTrim(t, p)
	}
	for _, want := range []string{"Icon=local-app", "Icon=remote-app", "Icon=audio-card"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("missing %s in:\n%s", want, contents)
		}
	}

	// files landed in the hicolor theme (svg spec-dir, png size-dir)
	svgPath := filepath.Join(home, ".local", "share", "icons", "hicolor", "scalable", "apps", "local-app.svg")
	if got := readTrim(t, svgPath); got != svg {
		t.Fatalf("scalable icon = %q", got)
	}
	pngPath := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "remote-app.png")
	if got := readTrim(t, pngPath); got != "PNGDATA" {
		t.Fatalf("256x256 icon = %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "audio-card.png")); !os.IsNotExist(err) {
		t.Fatal("theme-name ref must not be materialized")
	}
}
