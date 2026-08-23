package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// package builds the project and assembles a minimal installable plugin under
// pkg/: manifest.json + ui/ (surfaces, generated bridge, JS bundle) + docs
// (README/LICENSE/preview if present) — no JS source. It then
// validates the result with omarchy plugin validate.
func packageDir(dir string) error {
	start := time.Now()
	if _, err := runBuild(dir); err != nil {
		return err
	}

	pkg := filepath.Join(dir, "pkg")
	if err := os.RemoveAll(pkg); err != nil {
		return err
	}
	if err := copyPluginPayload(dir, pkg); err != nil {
		return err
	}

	cmd := exec.Command("omarchy", "plugin", "validate", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("omarchy plugin validate: %w", err)
	}
	okLine(displayPath(pkg) + "/")
	doneLine("packaged", time.Since(start))
	return nil
}
