//go:build live

package main

// Live verification for the full plugin pipeline against a real Quickshell.
//
// Two tiers, both excluded from normal `go test ./...` by this build tag:
//
//	Tier 1 (safe):   go test -tags live -run Offscreen
//	  Boots a throwaway Quickshell (QT_QPA_PLATFORM=offscreen) on the generated
//	  bridge and asserts the persistence round-trip through stdout + disk.
//	  Catches QJSEngine-only breakage (TDZ ordering, QtObject child limits,
//	  tree-shaking holes) without touching the desktop session.
//
//	Tier 2 (disruptive): OMA_LIVE_TEST=1 go test -tags live -run LiveShell
//	  Installs the fixture into ~/.config/omarchy/plugins/, restarts the real
//	  shell, drives it over IPC (summon/call/hide), asserts values and clean
//	  logs, then uninstalls and restores the shell. Restarts your desktop
//	  shell twice - that is the point, and why it is gated behind the env var.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	fixtureName     = "livetest"
	fixtureSeedRead = 77 // pre-seeded into <id>.json; proves the load path
	fixtureSeedSave = 88 // written via set(); proves the write path
)

// fixturePaths builds the test project in a temp dir and bundles it,
// returning the project dir and the installed-plugin id.
func fixturePaths(t *testing.T) (dir, id string) {
	t.Helper()
	dir = t.TempDir()
	if _, err := scaffoldWithOptions(filepath.Join(dir, fixtureName), []string{"panel"}, scaffoldOptions{PanelMode: "window"}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	project := filepath.Join(dir, fixtureName)

	// Known-good plugin source: state mirrors config so the probe can observe
	// persisted values through bridged QML properties.
	writeFile(filepath.Join(project, "src", "index.js"), `import { config, state } from "@oma/runtime";

export const music = state({ playing: false, volume: 100 });
export const settings = config({ savedVolume: 50 });

export function toggle() {
	music.playing = !music.playing;
}

export function applySavedVolume() {
	music.volume = settings.get("savedVolume");
}

export function setSavedVolume(v) {
	settings.set("savedVolume", Number(v));
}
`)

	// Minimal panel surface: embeds the bridge as `logic`, exposes the shell
	// contract (open/close with recursion guard) and a probe() the test calls
	// over IPC. arg, when numeric, exercises the persistence write path.
	writeFile(filepath.Join(project, "ui", "Panel.qml"), `import QtQuick
import Quickshell

Item {
	id: root
	property var shell: null
	property bool opened: false
	property string pluginId: "__ID__"

	function open(payloadJson) {
		root.opened = true
	}

	function close() {
		if (!root.opened) return
		root.opened = false
		if (typeof shell !== "undefined" && shell && typeof shell.hide === "function")
			shell.hide(root.pluginId)
	}

	function probe(arg) {
		if (arg !== undefined && arg !== null && arg !== "") logic.setSavedVolume(Number(arg))
		logic.applySavedVolume()
		return JSON.stringify({ playing: logic.playing, volume: logic.volume })
	}

	Livetest {
		id: logic
	}
}
`)
	m, err := readManifest(filepath.Join(project, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	writeFile(filepath.Join(project, "ui", "Panel.qml"),
		strings.Replace(readFileString(t, filepath.Join(project, "ui", "Panel.qml")), "__ID__", m.ID, 1))

	if err := bundleProject(project); err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if _, err := generateBridge(project); err != nil {
		t.Fatalf("bridge: %v", err)
	}
	return project, m.ID
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func settingsFileFor(id string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "omarchy", id+".json")
}

func waitFor(t *testing.T, timeout time.Duration, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func runCapture(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run() // timeouts and non-zero exits are handled by callers scanning output
	return buf.String()
}

func restartShellAndWait(t *testing.T) {
	t.Helper()
	if out, err := exec.Command("omarchy", "restart", "shell").CombinedOutput(); err != nil {
		t.Fatalf("omarchy restart shell: %v\n%s", err, out)
	}
	waitFor(t, 30*time.Second, "omarchy-shell IPC to come up", func() bool {
		return exec.Command("omarchy-shell", "shell", "ping").Run() == nil
	})
	time.Sleep(2 * time.Second) // plugin scan subprocess settles
}

func shellIPC(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("omarchy-shell", append([]string{"shell"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("IPC-ERROR(%v) %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

var qmlBreakageRe = regexp.MustCompile(`unavailable|Cannot assign|threw:|ReferenceError|TypeError|used before its declaration`)

func TestOffscreenBridgeSmoke(t *testing.T) {
	if _, err := exec.LookPath("quickshell"); err != nil {
		t.Skip("quickshell not installed")
	}
	project, id := fixturePaths(t)

	settingsPath := settingsFileFor(id)
	if err := os.WriteFile(settingsPath, []byte(fmt.Sprintf(`{"savedVolume": %d}`, fixtureSeedRead)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(settingsPath) })

	harness := t.TempDir()
	for _, f := range []string{"Livetest.qml", "index.mjs"} {
		data, err := os.ReadFile(filepath.Join(project, "ui", f))
		if err != nil {
			t.Fatalf("copy %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(harness, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(harness, "shell.qml"), `import QtQuick
import Quickshell

Scope {
	Livetest {
		id: logic
		// FileView loads asynchronously - only assert once the persistence
		// bootstrap has actually bound (see __omaLoad).
		onOmaReadyChanged: if (omaReady) Qt.callLater(runProbe)
		function runProbe() {
			logic.applySavedVolume()
			console.log("BRIDGE-SEEDED " + logic.volume)
			logic.setSavedVolume(88)
		}
	}
	Timer {
		interval: 3000
		running: true
		onTriggered: Qt.quit()
	}
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "quickshell", "-n", "-p", harness)
	cmd.Env = append(os.Environ(), "QT_QPA_PLATFORM=offscreen")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch quickshell: %v", err)
	}

	// Write-through: config.set() -> persist registry -> debounced Timer ->
	// FileView.setText -> disk. Poll instead of assuming the 200ms debounce.
	wrote := false
	for i := 0; i < 40 && ctx.Err() == nil; i++ {
		if data, err := os.ReadFile(settingsPath); err == nil && bytes.Contains(data, []byte(fmt.Sprint(fixtureSeedSave))) {
			wrote = true
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	cancel()
	_ = cmd.Wait()

	if !strings.Contains(out.String(), fmt.Sprintf("BRIDGE-SEEDED %d", fixtureSeedRead)) {
		t.Fatalf("load path failed (seeded %d not observed)\noutput:\n%s", fixtureSeedRead, out.String())
	}
	if !wrote {
		t.Fatalf("write path failed (%d never reached %s)\noutput:\n%s", fixtureSeedSave, settingsPath, out.String())
	}
	if hit := qmlBreakageRe.FindString(out.String()); hit != "" {
		t.Fatalf("QML breakage detected (%q):\n%s", hit, out.String())
	}
}

// TestOffscreenBridgeChurn is the RUNTIME-001 regression: the shell destroys
// a hidden panel's bridge QtObject while the JS module survives, then mounts
// a fresh bridge. Writes after the swap must still reach disk (the runtime
// re-targets its write channel on every bind; the old per-instance latch
// used to route them into the destroyed bridge and lose everything).
func TestOffscreenBridgeChurn(t *testing.T) {
	if _, err := exec.LookPath("quickshell"); err != nil {
		t.Skip("quickshell not installed")
	}
	project, id := fixturePaths(t)

	settingsPath := settingsFileFor(id)
	if err := os.WriteFile(settingsPath, []byte(fmt.Sprintf(`{"savedVolume": %d}`, fixtureSeedRead)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(settingsPath) })

	harness := t.TempDir()
	for _, f := range []string{"Livetest.qml", "index.mjs"} {
		data, err := os.ReadFile(filepath.Join(project, "ui", f))
		if err != nil {
			t.Fatalf("copy %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(harness, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Bridge A mounts, seeds, writes 88, then is destroyed and replaced by
	// bridge B; B must accept the 89 write and flush it to disk.
	writeFile(filepath.Join(harness, "shell.qml"), `import QtQuick
import Quickshell

Scope {
	id: root
	property bool showBridge: true
	property int stage: 0

	Loader {
		id: holder
		sourceComponent: root.showBridge ? bridgeComp : null
	}

	Component {
		id: bridgeComp
		Livetest {}
	}

	// Reconnects to each mounted bridge (A, then B) and drives the churn
	// sequence; everything is reached through holder.item, so no id scoping
	// across the Loader boundary is needed.
	Connections {
		target: holder.item
		function onOmaReadyChanged() {
			if (!holder.item || !holder.item.omaReady) return
			if (root.stage === 0) {
				root.stage = 1
				holder.item.applySavedVolume()
				console.log("CHURN-A-SEEDED " + holder.item.volume)
				holder.item.setSavedVolume(88)
				Qt.callLater(function() {
					root.showBridge = false // destroy A: flush + unbind
					Qt.callLater(function() {
						root.showBridge = true // mount B: re-target sink
					})
				})
			} else if (root.stage === 1) {
				root.stage = 2
				console.log("CHURN-B-SEEDED")
				holder.item.setSavedVolume(89) // must reach disk via B
			}
		}
	}

	Timer {
		interval: 3000
		running: true
		onTriggered: Qt.quit()
	}
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "quickshell", "-n", "-p", harness)
	cmd.Env = append(os.Environ(), "QT_QPA_PLATFORM=offscreen")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch quickshell: %v", err)
	}

	// B's write is debounced - poll the disk for the final value.
	wrote := false
	for i := 0; i < 40 && ctx.Err() == nil; i++ {
		if data, err := os.ReadFile(settingsPath); err == nil && bytes.Contains(data, []byte(fmt.Sprint(fixtureSeedSave+1))) {
			wrote = true
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	cancel()
	_ = cmd.Wait()

	for _, want := range []string{"CHURN-A-SEEDED", "CHURN-B-SEEDED"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q (churn sequence did not complete)\noutput:\n%s", want, out.String())
		}
	}
	if !wrote {
		t.Fatalf("write after bridge replacement failed (%d never reached %s)\noutput:\n%s", fixtureSeedSave+1, settingsPath, out.String())
	}
	if hit := qmlBreakageRe.FindString(out.String()); hit != "" {
		t.Fatalf("QML breakage detected (%q):\n%s", hit, out.String())
	}
}

func TestLiveShellRoundTrip(t *testing.T) {
	if os.Getenv("OMA_LIVE_TEST") != "1" {
		t.Skip("set OMA_LIVE_TEST=1 to run (restarts the real omarchy shell)")
	}
	for _, bin := range []string{"omarchy", "omarchy-shell", "qs"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	project, id := fixturePaths(t)

	if err := os.WriteFile(settingsFileFor(id), []byte(fmt.Sprintf(`{"savedVolume": %d}`, fixtureSeedRead)), 0o644); err != nil {
		t.Fatal(err)
	}
	restarted := false
	t.Cleanup(func() {
		_ = uninstall(project)
		restartShellAndWait(t) // leave the session with stock plugins
		if restarted {
			_ = os.Remove(settingsFileFor(id))
		}
	})

	if err := install(project); err != nil {
		t.Fatalf("install: %v", err)
	}
	restarted = true
	restartShellAndWait(t)

	// discovered + enabled
	listing := shellIPC(t, "listPlugins")
	var plugins []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(listing), &plugins); err != nil {
		t.Fatalf("listPlugins: %v\n%s", err, listing)
	}
	found := false
	for _, p := range plugins {
		if p.ID == id {
			found = p.Enabled
			break
		}
	}
	if !found {
		t.Fatalf("%s not enabled; listing:\n%s", id, listing)
	}

	if out := shellIPC(t, "summon", id, "{}"); out != "ok" {
		t.Fatalf("summon: %q", out)
	}
	waitFor(t, 5*time.Second, "panel to mount", func() bool {
		return strings.Contains(shellIPC(t, "call", id, "probe", ""), `"volume":`)
	})

	// load path through the real engine
	readBack := shellIPC(t, "call", id, "probe", "")
	if !strings.Contains(readBack, fmt.Sprintf(`"volume":%d`, fixtureSeedRead)) {
		t.Fatalf("persistence read failed: want volume %d, got %s", fixtureSeedRead, readBack)
	}
	// write path through the real engine (debounced)
	writeBack := shellIPC(t, "call", id, "probe", fmt.Sprint(fixtureSeedSave))
	if !strings.Contains(writeBack, fmt.Sprintf(`"volume":%d`, fixtureSeedSave)) {
		t.Fatalf("state write failed: got %s", writeBack)
	}
	waitFor(t, 3*time.Second, fmt.Sprintf("%s to contain %d", settingsFileFor(id), fixtureSeedSave), func() bool {
		return strings.Contains(readFileString(t, settingsFileFor(id)), fmt.Sprint(fixtureSeedSave))
	})

	// shell.hide() returns void - empty output (or a quiet no-op) is success.
	if out := shellIPC(t, "hide", id); strings.HasPrefix(out, "IPC-ERROR") {
		t.Fatalf("hide: %q", out)
	}

	// the session log must be free of plugin errors (recursion guards, TDZ...)
	time.Sleep(500 * time.Millisecond)
	// `qs log --all` does not exist (it errored into an empty capture, making
	// this check vacuous) - select the instance by pid instead.
	shellPID, err := findShellPID()
	if err != nil {
		t.Fatalf("find shell pid: %v", err)
	}
	logOut := runCapture(t, 15*time.Second, "qs", "log", "--pid", fmt.Sprint(shellPID), "-t", "500")
	if logOut == "" {
		t.Fatal("qs log returned no output for the running shell")
	}
	for _, line := range strings.Split(logOut, "\n") {
		if strings.Contains(line, id) && qmlBreakageRe.MatchString(line) {
			t.Fatalf("session log shows breakage:\n%s", line)
		}
	}
}
