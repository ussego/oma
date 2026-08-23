package main

import (
	"fmt"
	"os"
	"os/exec"
)

func validate(dir string) error {
	cmd := exec.Command("omarchy", "plugin", "validate", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("omarchy plugin validate: %w", err)
	}
	okLine("valid")
	return nil
}

// restart installs the project and restarts the shell so it picks up changes.
func restart(projectDir string) error {
	if err := install(projectDir); err != nil {
		return err
	}
	cmd := exec.Command("omarchy", "restart", "shell")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("omarchy restart shell: %w", err)
	}
	noteLine("restarted shell")
	return nil
}
