package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurfaceAdd(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "myapp")
	if err := scaffold(target, []string{"bar-widget"}); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(target); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(dir) })

	// add two new kinds, one comma-separated one spaced
	if err := runSurfaceAdd(".", []string{"panel,overlay", "menu"}); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	// "panel" ships as a bar-widget button + anchored popup: the manifest kind
	// becomes bar-widget (deduped — project already had one), Panel.qml is
	// loaded by the widget's Loader, so it registers no separate entry point.
	want := map[string]bool{"bar-widget": true, "overlay": true, "menu": true}
	if len(m.Kinds) != len(want) {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	for _, k := range m.Kinds {
		if !want[k] {
			t.Fatalf("unexpected kind %q in %v", k, m.Kinds)
		}
	}
	for kind, ep := range map[string]string{
		"overlay": "ui/Overlay.qml",
		"menu":    "ui/Menu.qml",
	} {
		if m.EntryPoints[kind] != ep {
			t.Fatalf("entryPoints[%s] = %q, want %q", kind, m.EntryPoints[kind], ep)
		}
		if _, err := os.Stat(filepath.Join(target, ep)); err != nil {
			t.Fatalf("missing skeleton %s: %v", ep, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "ui", "Panel.qml")); err != nil {
		t.Fatalf("missing panel popup skeleton: %v", err)
	}

	// existing bar-widget file untouched and still registered once
	data, _ := os.ReadFile(filepath.Join(target, "manifest.json"))
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// idempotent: re-adding present kinds is a no-op that reports skips
	if err := runSurfaceAdd(".", []string{"panel"}); err != nil {
		t.Fatal(err)
	}
	m2, _ := readManifest(filepath.Join(target, "manifest.json"))
	if len(m2.Kinds) != len(want) {
		t.Fatalf("duplicate kinds after idempotent run: %v", m2.Kinds)
	}

	// unknown kind rejected
	if err := runSurfaceAdd(".", []string{"nope"}); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func readOMAConfigRaw(t *testing.T, project string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, "oma.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse oma.json: %v\n%s", err, data)
	}
	return raw
}

func panelModeOf(t *testing.T, raw map[string]any) string {
	t.Helper()
	if raw["panel"] == nil {
		return ""
	}
	m, _ := raw["panel"].(map[string]any)
	s, _ := m["mode"].(string)
	return s
}

func TestPersistPanelMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()

	// missing file + non-attached mode: created with mode + $schema
	if err := persistPanelMode(dir, "window"); err != nil {
		t.Fatal(err)
	}
	raw := readOMAConfigRaw(t, dir)
	if panelModeOf(t, raw) != "window" || raw["$schema"] == nil {
		t.Fatalf("created oma.json = %v", raw)
	}

	// missing file + attached: nothing is written
	if err := os.Remove(filepath.Join(dir, "oma.json")); err != nil {
		t.Fatal(err)
	}
	if err := persistPanelMode(dir, "attached"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "oma.json")); !os.IsNotExist(err) {
		t.Fatal("attached mode created an oma.json")
	}

	// attached drops a mode-only panel key, keeps other keys
	writeOMAConfig(t, dir, `{"icon":"x","panel":{"mode":"attached"}}`)
	if err := persistPanelMode(dir, "attached"); err != nil {
		t.Fatal(err)
	}
	raw = readOMAConfigRaw(t, dir)
	if raw["panel"] != nil || raw["icon"] != "x" {
		t.Fatalf("mode-only panel key not dropped: %v", raw)
	}

	// attached rewrites a non-default mode, preserving extra keys
	writeOMAConfig(t, dir, `{"panel":{"mode":"window","extra":1}}`)
	if err := persistPanelMode(dir, "attached"); err != nil {
		t.Fatal(err)
	}
	raw = readOMAConfigRaw(t, dir)
	if panelModeOf(t, raw) != "attached" {
		t.Fatalf("mode = %v", raw["panel"])
	}
	if m := raw["panel"].(map[string]any); m["extra"] != float64(1) {
		t.Fatalf("extra key lost: %v", raw["panel"])
	}

	// window/both sets the mode and preserves extras
	writeOMAConfig(t, dir, `{"panel":{"mode":"attached","extra":true}}`)
	if err := persistPanelMode(dir, "both"); err != nil {
		t.Fatal(err)
	}
	raw = readOMAConfigRaw(t, dir)
	if panelModeOf(t, raw) != "both" {
		t.Fatalf("mode = %v", raw["panel"])
	}
	if m := raw["panel"].(map[string]any); m["extra"] != true {
		t.Fatalf("extra key lost: %v", raw["panel"])
	}

	// $schema is added when absent
	writeOMAConfig(t, dir, `{"icon":"x"}`)
	if err := persistPanelMode(dir, "window"); err != nil {
		t.Fatal(err)
	}
	if raw := readOMAConfigRaw(t, dir); raw["$schema"] == nil {
		t.Fatal("$schema not added")
	}

	// broken json errors
	writeOMAConfig(t, dir, `{bad`)
	if err := persistPanelMode(dir, "window"); err == nil {
		t.Fatal("expected parse error")
	}
}

// readManifestKinds is a small helper for the switching tests below.
func readManifestKinds(t *testing.T, project string) []string {
	t.Helper()
	m, err := readManifest(filepath.Join(project, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m.Kinds
}

// attached -> window: kinds collapse to panel-only, host widget and PanelWindow
// leftovers are removed, oma.json records the new mode.
func TestSurfaceAddPanelSwitchAttachedToWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}

	if err := runSurfaceAdd(target, []string{"--panel-mode", "window", "panel"}); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Kinds) != 1 || m.Kinds[0] != "panel" {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	if m.EntryPoints["panel"] != "ui/Panel.qml" || m.EntryPoints["barWidget"] != "" {
		t.Fatalf("entryPoints = %v", m.EntryPoints)
	}
	for _, gone := range []string{"ui/BarWidget.qml", "ui/PanelWindow.qml"} {
		if _, err := os.Stat(filepath.Join(target, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s should be gone in window mode", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "ui", "Panel.qml")); err != nil {
		t.Fatal("floating Panel.qml missing")
	}
	if panelModeOf(t, readOMAConfigRaw(t, target)) != "window" {
		t.Fatal("oma.json panel.mode != window")
	}
}

// window -> attached: kinds collapse to bar-widget host + attached popup.
func TestSurfaceAddPanelSwitchWindowToAttached(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester", PanelMode: "window"}); err != nil {
		t.Fatal(err)
	}

	if err := runSurfaceAdd(target, []string{"--panel-mode", "attached", "panel"}); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Kinds) != 1 || m.Kinds[0] != "bar-widget" {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	if m.EntryPoints["barWidget"] != "ui/BarWidget.qml" || m.EntryPoints["panel"] != "" {
		t.Fatalf("entryPoints = %v", m.EntryPoints)
	}
	for _, present := range []string{"ui/BarWidget.qml", "ui/Panel.qml"} {
		if _, err := os.Stat(filepath.Join(target, present)); err != nil {
			t.Fatalf("%s missing", present)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "ui", "PanelWindow.qml")); !os.IsNotExist(err) {
		t.Fatal("PanelWindow.qml should be removed in attached mode")
	}
	// attached is the default, but the persisted mode follows the switch
	if panelModeOf(t, readOMAConfigRaw(t, target)) != "attached" {
		t.Fatal("oma.json panel.mode != attached")
	}
}

// attached -> both: kinds keep the host and add the standalone window entry.
func TestSurfaceAddPanelSwitchAttachedToBoth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}

	if err := runSurfaceAdd(target, []string{"--panel-mode", "both", "panel"}); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Kinds) != 2 || m.Kinds[0] != "bar-widget" || m.Kinds[1] != "panel" {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	if m.EntryPoints["barWidget"] != "ui/BarWidget.qml" || m.EntryPoints["panel"] != "ui/PanelWindow.qml" {
		t.Fatalf("entryPoints = %v", m.EntryPoints)
	}
	for _, present := range []string{"ui/BarWidget.qml", "ui/Panel.qml", "ui/PanelWindow.qml"} {
		if _, err := os.Stat(filepath.Join(target, present)); err != nil {
			t.Fatalf("%s missing", present)
		}
	}
	if panelModeOf(t, readOMAConfigRaw(t, target)) != "both" {
		t.Fatal("oma.json panel.mode != both")
	}
}

// Without a --panel-mode flag the existing oma.json mode is reused; a
// same-mode re-add is a no-op.
func TestSurfaceAddPanelModeFallbackFromOMAConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester", PanelMode: "window"}); err != nil {
		t.Fatal(err)
	}

	before := readManifestKinds(t, target)
	if err := runSurfaceAdd(target, []string{"panel"}); err != nil {
		t.Fatal(err)
	}
	after := readManifestKinds(t, target)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Fatalf("same-mode re-add changed kinds: %v -> %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(target, "ui", "BarWidget.qml")); !os.IsNotExist(err) {
		t.Fatal("window mode gained a bar widget host")
	}
}

// --panel-mode with an invalid value is rejected up front.
func TestSurfaceAddInvalidPanelMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if _, err := scaffoldWithOptions(target, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	if err := runSurfaceAdd(target, []string{"--panel-mode", "docked", "panel"}); err == nil {
		t.Fatal("expected unknown --panel-mode error")
	}
	if err := runSurfaceAdd(target, []string{"--panel-mode"}); err == nil {
		t.Fatal("expected missing-value error")
	}
}

func TestSurfaceAddBarIcon(t *testing.T) {
	// barIcon (glyph vocabulary) must land in a new bar-widget skeleton,
	// while the launcher icon (freedesktop vocabulary) must not leak into it.
	dir := t.TempDir()

	target := filepath.Join(dir, "glyphproj")
	if _, err := scaffoldWithOptions(target, []string{"overlay"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	writeOMAConfig(t, target, `{"barIcon":"\uf4d8"}`)
	if err := runSurfaceAdd(target, []string{"bar-widget"}); err != nil {
		t.Fatal(err)
	}
	qml := readTrim(t, filepath.Join(target, "ui", "BarWidget.qml"))
	// the glyph is %q-escaped into the QML, so look for the literal sequence
	if !strings.Contains(qml, `\uf4d8`) {
		t.Fatalf("barIcon not used in the skeleton:\n%s", qml)
	}
	if strings.Contains(qml, `\uf013`) {
		t.Fatal("cog fallback used despite barIcon")
	}

	target2 := filepath.Join(dir, "iconproj")
	if _, err := scaffoldWithOptions(target2, []string{"overlay"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	writeOMAConfig(t, target2, `{"icon":"utilities-system-monitor"}`)
	if err := runSurfaceAdd(target2, []string{"bar-widget"}); err != nil {
		t.Fatal(err)
	}
	qml2 := readTrim(t, filepath.Join(target2, "ui", "BarWidget.qml"))
	if !strings.Contains(qml2, `\uf013`) {
		t.Fatalf("expected the cog fallback:\n%s", qml2)
	}
	if strings.Contains(qml2, "utilities-system-monitor") {
		t.Fatal("launcher icon leaked into the bar glyph")
	}
}
