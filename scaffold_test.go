package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffold(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "omusic")
	if err := scaffold(target, []string{"bar-widget", "overlay"}); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{
		"manifest.json",
		filepath.Join("src", "index.js"),
		filepath.Join("ui", "BarWidget.qml"),
		filepath.Join("ui", "Overlay.qml"),
	} {
		if _, err := os.Stat(filepath.Join(target, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d", m.SchemaVersion)
	}
	if m.ID != userNamespace()+".omusic" {
		t.Fatalf("id = %q", m.ID)
	}
	if len(m.Kinds) != 2 || m.Kinds[0] != "bar-widget" || m.Kinds[1] != "overlay" {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	if m.EntryPoints["barWidget"] != "ui/BarWidget.qml" {
		t.Fatalf("barWidget entry point = %q", m.EntryPoints["barWidget"])
	}
	if m.EntryPoints["overlay"] != "ui/Overlay.qml" {
		t.Fatalf("overlay entry point = %q", m.EntryPoints["overlay"])
	}
	if m.Framework != "oma" {
		t.Fatalf("framework = %q, want oma", m.Framework)
	}

	index, err := os.ReadFile(filepath.Join(target, "src", "index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `from "@oma/runtime"`) {
		t.Fatal("index.js must import the oma runtime")
	}

	widget, err := os.ReadFile(filepath.Join(target, "ui", "BarWidget.qml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"qs.Ui", "BarWidget", "moduleName", "Omusic {", "logic.toggle()", "OpticalGlyph"} {
		if !strings.Contains(string(widget), want) {
			t.Fatalf("BarWidget.qml missing %q", want)
		}
	}
}

func TestQmlSafeBridge(t *testing.T) {
	cases := map[string]string{
		"my-plugin": "MyPlugin",
		"123abc":    "B123abc",
		"a b":       "AB",
		"":          "Bridge",
		"9":         "B9",
	}
	for in, want := range cases {
		if got := qmlSafeBridge(in); got != want {
			t.Errorf("qmlSafeBridge(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeAndUserNamespace(t *testing.T) {
	if got := sanitize("Foo Bar!"); got != "foo-bar" {
		t.Errorf("sanitize = %q", got)
	}
	if got := sanitize("---"); got != "" {
		t.Errorf("sanitize(---) = %q", got)
	}
	if got := sanitize("My.Plugin_v2"); got != "my.plugin-v2" {
		t.Errorf("sanitize = %q", got)
	}
	t.Setenv("USER", "My.User")
	if got := userNamespace(); got != "my.user" { // dots are preserved
		t.Errorf("userNamespace = %q", got)
	}
}

// Template scaffolds produce a buildable project with the placeholders
// substituted (a template that forgets __BRIDGE__ breaks at load time).
func TestScaffoldTemplate(t *testing.T) {
	for _, tpl := range []string{"todo", "counter", "settings-panel"} {
		dir := filepath.Join(t.TempDir(), "tpl-"+tpl)
		if _, err := scaffoldWithOptions(dir, []string{"panel"}, scaffoldOptions{Author: "tester", PanelMode: "window", Template: tpl}); err != nil {
			t.Fatalf("%s: scaffold: %v", tpl, err)
		}
		m, err := readManifest(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		bridge := bridgeBaseName(capitalize(m.Name), m.Kinds)
		panel, err := os.ReadFile(filepath.Join(dir, "ui", "Panel.qml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(panel), bridge+" {") {
			t.Errorf("%s: Panel.qml does not instantiate the %s bridge", tpl, bridge)
		}
		if strings.Contains(string(panel), "__BRIDGE__") || strings.Contains(string(panel), "__ID__") {
			t.Errorf("%s: template placeholders survived", tpl)
		}
		if _, err := runBuild(dir); err != nil {
			t.Errorf("%s: build: %v", tpl, err)
		}
	}
}

// The todo template must use the guarded-persistence pattern (no fsReady
// hand-rolling) - it is the canonical example of the fixed config().
func TestTodoTemplateUsesConfigPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tpl-todo")
	if _, err := scaffoldWithOptions(dir, []string{"panel"}, scaffoldOptions{Author: "tester", PanelMode: "window", Template: "todo"}); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, "src", "index.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"config(", "onReady", "snapshot"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("todo template missing %q", want)
		}
	}
	if strings.Contains(string(src), "fsReady") {
		t.Error("todo template still hand-rolls persistence (fsReady)")
	}
}
