package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// pluginDest resolves ~/.config/omarchy/plugins/<id>/ from the project's
// manifest id.
func pluginDest(projectDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "manifest.json"))
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}
	if m.ID == "" {
		return "", errors.New("manifest has no id")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "omarchy", "plugins", m.ID), nil
}

// install copies the minimal plugin payload (manifest.json + ui/ + docs) into
// ~/.config/omarchy/plugins/<id>/ so the running shell picks it up, enables
// it (discovered plugins start disabled — summon ignores them), then
// materializes launcher entries declared in oma.json (warn-and-skip: a broken
// optional config must not fail the install).
func install(projectDir string) error {
	start := time.Now()
	dest, err := pluginDest(projectDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := copyPluginPayload(projectDir, dest); err != nil {
		return err
	}
	okLine(displayPath(dest))
	m, err := readManifest(filepath.Join(projectDir, "manifest.json"))
	if err != nil {
		return err
	}
	if err := enableInstalled(m.ID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not enable %s: %v\n", m.ID, err)
	}
	written, err := writeLauncherEntries(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping launcher entries: %v\n", err)
	} else {
		for _, path := range written {
			okLine("launcher " + displayPath(path))
		}
	}
	doneLine("installed", time.Since(start))
	return nil
}

// enableInstalled mirrors what omarchy-plugin-add does after cloning:
// rescan the plugin dirs, wait for the shell to discover the id, then enable.
// Best-effort: failures warn (files stay installed) instead of failing.
func enableInstalled(id string) error {
	_ = exec.Command("omarchy-shell", "shell", "rescanPlugins").Run()
	known := false
	for range 40 {
		out, err := exec.Command("omarchy-shell", "shell", "listPlugins").Output()
		if err == nil && strings.Contains(string(out), `"`+id+`"`) {
			known = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !known {
		return fmt.Errorf("shell did not discover the plugin (is omarchy-shell running?)")
	}
	cmd := exec.Command("omarchy", "plugin", "enable", id)
	var errOut strings.Builder
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(errOut.String()))
	}
	okLine("enabled " + id)
	return nil
}

// uninstall removes the installed plugin dir (~/.config/omarchy/plugins/<id>/)
// and any launcher entries oma created for it.
func uninstall(projectDir string) error {
	dest, err := pluginDest(projectDir)
	if err != nil {
		return err
	}
	if removed, _ := removeLauncherEntries(projectDir); len(removed) > 0 {
		for _, path := range removed {
			okLine("removed " + displayPath(path))
		}
	}
	if m, err := readManifest(filepath.Join(projectDir, "manifest.json")); err == nil && m.ID != "" {
		if home, err := os.UserHomeDir(); err == nil {
			settingsPath := filepath.Join(home, ".config", "omarchy", m.ID+".json")
			if err := os.Remove(settingsPath); err == nil {
				okLine("removed " + displayPath(settingsPath))
			}
		}
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	okLine("removed " + displayPath(dest))
	noteLine("uninstalled")
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
