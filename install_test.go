package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// install must rebuild a stale or unbuilt project but not touch a fresh one
// (the dev loop builds right before install, so this must not double-build).
func TestInstallNeedsBuild(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "p")
	if _, err := scaffoldWithOptions(project, []string{"panel"}, scaffoldOptions{Author: "tester"}); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(filepath.Join(project, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !installNeedsBuild(project, m) {
		t.Fatal("unbuilt project must need a build")
	}
	if err := build(project); err != nil {
		t.Fatal(err)
	}
	if installNeedsBuild(project, m) {
		t.Fatal("fresh build must not need a rebuild")
	}
	src := filepath.Join(project, "src", "index.js")
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	future := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatal(err)
	}
	if !installNeedsBuild(project, m) {
		t.Fatal("src newer than the build must need a rebuild")
	}
}
