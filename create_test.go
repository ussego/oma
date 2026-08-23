package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCreateArgs(t *testing.T) {
	cases := []struct {
		label string
		args  []string
		want  createFlags
	}{
		{
			"flags with space-separated values",
			[]string{"omusic", "-s", "panel,overlay", "-a", "ussego", "-d", "desc", "-v", "1.0.0"},
			createFlags{name: "omusic", kinds: []string{"panel", "overlay"}, author: "ussego", desc: "desc", ver: "1.0.0"},
		},
		{
			"long flags with = values",
			[]string{"omusic", "--surfaces=panel", "--author=u", "--description=d", "--version=0.2.0"},
			createFlags{name: "omusic", kinds: []string{"panel"}, author: "u", desc: "d", ver: "0.2.0"},
		},
		{
			"legacy positional kinds",
			[]string{"foo", "panel,overlay"},
			createFlags{name: "foo", kinds: []string{"panel", "overlay"}},
		},
		{
			"panel-mode short flag",
			[]string{"foo", "-p", "window"},
			createFlags{name: "foo", panelMode: "window"},
		},
		{
			"panel-mode long flag",
			[]string{"foo", "--panel-mode=both"},
			createFlags{name: "foo", panelMode: "both"},
		},
		{
			"deduped kinds",
			[]string{"foo", "-s", "panel,panel,overlay"},
			createFlags{name: "foo", kinds: []string{"panel", "overlay"}},
		},
	}
	for _, c := range cases {
		got, err := parseCreateArgs(c.args)
		if err != nil {
			t.Fatalf("%s: %v", c.label, err)
		}
		if got.name != c.want.name || got.author != c.want.author || got.desc != c.want.desc ||
			got.ver != c.want.ver || got.panelMode != c.want.panelMode ||
			strings.Join(got.kinds, ",") != strings.Join(c.want.kinds, ",") {
			t.Fatalf("%s: got %+v, want %+v", c.label, got, c.want)
		}
	}

	bad := []struct {
		label string
		args  []string
	}{
		{"unknown flag", []string{"foo", "--bogus", "x"}},
		{"unknown kind", []string{"foo", "-s", "widget"}},
		{"invalid panel mode", []string{"foo", "-p", "docked"}},
		{"flag without value", []string{"foo", "-s"}},
	}
	for _, c := range bad {
		if _, err := parseCreateArgs(c.args); err == nil {
			t.Fatalf("%s: expected error", c.label)
		}
	}
}

func TestValidateProjectName(t *testing.T) {
	valid := []string{"my-plugin", "ab", "abc123", "x-y-z", "UPPER"} // case is normalized
	for _, n := range valid {
		if err := validateProjectName(n); err != nil {
			t.Errorf("name %q should pass: %v", n, err)
		}
	}
	invalid := []struct{ name, want string }{
		{"", "required"},
		{"a", "2 characters"},
		{"my plugin", "lowercase"},
		{"my_plugin", "lowercase"},
		{"omarchy", "reserved"},
		{"omarchy.foo", "lowercase"}, // dots aren't valid in names at all
		{"omarchy-tools", "reserved"},
		{strings.Repeat("x", 41), "40"},
	}
	for _, c := range invalid {
		if err := validateProjectName(c.name); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("name %q: got %v, want error containing %q", c.name, err, c.want)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	valid := []string{"", "0.1.0", "1.2.3", "2.0.0-beta.1", "1.0.0+build.5"}
	for _, v := range valid {
		if err := validateVersion(v); err != nil {
			t.Errorf("version %q should pass: %v", v, err)
		}
	}
	invalid := []struct{ ver, want string }{
		{"1.0", "semver"},
		{"v1.0.0", "semver"},
		{"1.0.0.0", "semver"},
		// one suffix only — pre-release AND build together is not supported
		{"0.0.1-rc.1+b.2", "semver"},
		{strings.Repeat("1.", 10) + "0", "20"},
	}
	for _, c := range invalid {
		if err := validateVersion(c.ver); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("version %q: got %v, want error containing %q", c.ver, err, c.want)
		}
	}
}

func TestValidateAuthor(t *testing.T) {
	valid := []string{"", "ussego", "my-user", "a1", "UPPER"} // case is normalized
	for _, a := range valid {
		if err := validateAuthor(a); err != nil {
			t.Errorf("author %q should pass: %v", a, err)
		}
	}
	invalid := []struct{ author, want string }{
		{"sp ace", "lowercase"},
		{"sp_ace", "lowercase"},
		{"omarchy", "reserved"},
		{strings.Repeat("x", 31), "30"},
	}
	for _, c := range invalid {
		if err := validateAuthor(c.author); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("author %q: got %v, want error containing %q", c.author, err, c.want)
		}
	}
}

// The fully non-interactive path: everything provided, no prompts, files land.
func TestRunCreateNonInteractive(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if err := runCreate([]string{"omusic", "-s", "panel,overlay", "-a", "tester", "-d", "desc", "-v", "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(filepath.Join("omusic", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "tester.omusic" || m.Version != "1.0.0" || m.Description != "desc" {
		t.Fatalf("manifest = %+v", m)
	}
	// panel in attached mode ships as bar-widget; overlay stays
	if strings.Join(m.Kinds, ",") != "bar-widget,overlay" {
		t.Fatalf("kinds = %v", m.Kinds)
	}
	for _, f := range []string{
		"src/index.js",
		"ui/BarWidget.qml",
		"ui/Panel.qml",
		"ui/Overlay.qml",
		"package.json",
		"oma.json",
		".gitignore",
	} {
		if _, err := os.Stat(filepath.Join("omusic", f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}

	// conflicting existing directory fails without prompting
	if err := runCreate([]string{"omusic", "-s", "panel", "-a", "tester"}); err == nil {
		t.Fatal("expected conflict error for existing directory")
	}
	// invalid name fails up front
	if err := runCreate([]string{"Bad Name!", "-s", "panel"}); err == nil {
		t.Fatal("expected validation error")
	}
}

// The scaffold gitignore must ignore node_modules and local cruft, but never
// the built ui/ payload — omarchy plugin add clones HEAD and validates
// ui/index.mjs + the bridge without running oma build.
func TestScaffoldGitignore(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if err := runCreate([]string{"proj", "-s", "service", "-a", "tester"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join("proj", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"pkg/", "node_modules/", "*.log", ".DS_Store"} {
		if !strings.Contains(content, want) {
			t.Errorf(".gitignore missing %q", want)
		}
	}
	// entries, not comment mentions: ui/ is committed build output
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "ui") {
			t.Errorf("gitignore entry %q ignores committed build output", line)
		}
	}
}
