package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// copyPluginPayload copies the minimal installable plugin from src to dst:
// manifest.json + ui/ (surfaces, bundled JS, bridge, LauncherWriter). README,
// LICENSE and preview assets are copied if present for marketplace compliance.
// src is the project dir, dst is the destination plugin dir (or pkg/).
func copyPluginPayload(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	manifestSrc := filepath.Join(src, "manifest.json")
	if _, err := os.Stat(manifestSrc); err != nil {
		return fmt.Errorf("missing manifest.json: %w", err)
	}
	if err := copyFile(manifestSrc, filepath.Join(dst, "manifest.json")); err != nil {
		return fmt.Errorf("copy manifest.json: %w", err)
	}
	uiSrc := filepath.Join(src, "ui")
	if _, err := os.Stat(uiSrc); err != nil {
		return fmt.Errorf("missing ui: %w", err)
	}
	if err := copyTree(uiSrc, filepath.Join(dst, "ui")); err != nil {
		return fmt.Errorf("copy ui: %w", err)
	}
	// optional docs/assets — copy if present, ignore otherwise
	for _, pat := range []string{"README*", "LICENSE*", "preview.*"} {
		matches, _ := filepath.Glob(filepath.Join(src, pat))
		for _, m := range matches {
			// skip directories
			if info, err := os.Stat(m); err != nil || info.IsDir() {
				continue
			}
			rel := filepath.Base(m)
			if err := copyFile(m, filepath.Join(dst, rel)); err != nil {
				return fmt.Errorf("copy %s: %w", rel, err)
			}
		}
	}
	return nil
}
