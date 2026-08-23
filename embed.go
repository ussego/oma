package main

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Everything the CLI needs ships inside the binary: runtime, bundler shim,
// skills docs. No SDK checkout, no OMA_ROOT, no Deno. The oma.json schema
// stays committed in the repo and is referenced by versioned raw URL.

//go:embed assets/oma.js
//go:embed assets/globalthis-shim.mjs
var assetFS embed.FS

//go:embed assets/skill-data
var skillFS embed.FS

// omaDir returns the per-user directory for extracted assets, creating it.
func omaDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "oma")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// assetPath extracts an embedded asset to the user cache dir (only when its
// content changed) and returns the real path, so file-path-based consumers
// (esbuild Inject/Alias) can use it directly.
func assetPath(name string) (string, error) {
	content, err := assetFS.ReadFile("assets/" + name)
	if err != nil {
		return "", fmt.Errorf("embedded asset %q: %w", name, err)
	}
	dir, err := omaDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(name))
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, content) {
		return path, nil
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// extractSkillData mirrors the embedded skill-data tree into the cache dir
// and returns its path, keeping skills.go's plain-file readers untouched.
func extractSkillData() (string, error) {
	root, err := omaDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(root, "skill-data")
	err = fs.WalkDir(skillFS, "assets/skill-data", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("assets/skill-data", p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := skillFS.ReadFile(p)
		if err != nil {
			return err
		}
		if existing, err := os.ReadFile(target); err != nil || !bytes.Equal(existing, content) {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, content, 0o644); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}
