package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Uninstalling must remove the persisted config file (~/.config/omarchy/<id>.json)
// alongside the plugin directory and oma-managed launcher entries. Runs under a
// fake $HOME so the real session is never touched.
func TestUninstallRemovesSettingsAndPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	project := filepath.Join(home, "proj")
	if _, err := scaffoldWithOptions(project, []string{"bar-widget"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(filepath.Join(project, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}

	// simulate an installed plugin + persisted settings + a launcher entry
	dest := filepath.Join(home, ".config", "omarchy", "plugins", m.ID)
	if err := os.MkdirAll(filepath.Join(dest, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "ui", "index.mjs"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".config", "omarchy", m.ID+".json")
	if err := os.WriteFile(settingsPath, []byte(`{"savedVolume": 42}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := uninstall(project); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("plugin dir still exists after uninstall: %s", dest)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings file still exists after uninstall: %s", settingsPath)
	}
}

// install() copies the payload, writes launcher entries and degrades enable
// failures to a warning (no omarchy-shell in a hermetic test env).
func TestInstallCopiesPayloadAndWritesLaunchers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	project := filepath.Join(home, "proj")
	if _, err := scaffoldWithOptions(project, []string{"bar-widget"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	// simulate a built plugin
	if err := os.MkdirAll(filepath.Join(project, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "ui", "index.mjs"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "ui", "BarWidget.qml"), []byte("BarWidget {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runLauncherAdd(project, []string{"App"}); err != nil {
		t.Fatal(err)
	}

	if err := install(project); err != nil {
		t.Fatalf("install: %v", err)
	}

	dest := filepath.Join(home, ".config", "omarchy", "plugins", "tester.proj")
	for _, f := range []string{"manifest.json", "ui/index.mjs", "ui/BarWidget.qml", "README.md"} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Errorf("installed copy missing %s", f)
		}
	}
	// the src dir is not part of the payload
	if _, err := os.Stat(filepath.Join(dest, "src")); !os.IsNotExist(err) {
		t.Error("src shipped inside the installed plugin")
	}
	desktop := filepath.Join(home, ".local", "share", "applications", "tester.proj.desktop")
	if _, err := os.Stat(desktop); err != nil {
		t.Errorf("launcher entry not materialized: %v", err)
	}

	// second install replaces the previous copy
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("# proj v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := install(project); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil || string(data) != "# proj v2\n" {
		t.Fatalf("reinstall did not replace payload: %q %v", data, err)
	}
}

func TestCopyPluginPayload(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	for _, d := range []string{"ui/sub", "src"} {
		if err := os.MkdirAll(filepath.Join(src, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "manifest.json"), []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"ui/Panel.qml", "ui/sub/helper.qml", "ui/index.mjs", "src/index.js", "README.md", "README.zh.md", "LICENSE", "preview.png"} {
		if err := os.WriteFile(filepath.Join(src, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dst := filepath.Join(t.TempDir(), "pkg")
	if err := copyPluginPayload(src, dst); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"manifest.json",
		"ui/Panel.qml",
		"ui/sub/helper.qml",
		"ui/index.mjs",
		"README.md",
		"README.zh.md",
		"LICENSE",
		"preview.png",
	} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Errorf("payload missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "src")); !os.IsNotExist(err) {
		t.Error("src copied into payload")
	}

	// missing manifest errors
	if err := copyPluginPayload(filepath.Join(t.TempDir(), "empty"), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected error for missing manifest")
	}
}
