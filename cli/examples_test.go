package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stripBannerLines drops esbuild's `// <path>` module banners so builds in
// different directories compare equal.
func stripBannerLines(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "// ") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// The shipped examples must build and every hand-written surface must
// reference the bridge type the generator actually produces (a stale
// "CounterBridge" reference used to break the counter example at load time).
// Committed ui/ artifacts must also be fresh: AGENTS.md requires built
// index.mjs + <Name>.qml to be committed, so editing an example without
// rebuilding fails here.
func TestExamplesBuild(t *testing.T) {
	repo := filepath.Clean(filepath.Join(".."))
	for _, ex := range []string{"counter", "todo", "stopwatch"} {
		dir := filepath.Join(t.TempDir(), ex)
		if err := copyTree(filepath.Join(repo, "examples", ex), dir); err != nil {
			t.Fatalf("%s: copy: %v", ex, err)
		}
		m, err := readManifest(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatalf("%s: %v", ex, err)
		}
		want := bridgeBaseName(capitalize(m.Name), m.Kinds)

		res, err := runBuild(dir)
		if err != nil {
			t.Fatalf("%s: build: %v", ex, err)
		}
		if res.bridgeRel != "ui/"+want+".qml" {
			t.Fatalf("%s: bridge = %q, want ui/%s.qml", ex, res.bridgeRel, want)
		}

		for _, f := range []string{"index.mjs", want + ".qml"} {
			repoData, err := os.ReadFile(filepath.Join(repo, "examples", ex, "ui", f))
			if err != nil {
				t.Fatalf("%s: read committed ui/%s: %v", ex, f, err)
			}
			builtData, err := os.ReadFile(filepath.Join(dir, "ui", f))
			if err != nil {
				t.Fatal(err)
			}
			// esbuild banners embed the runtime/source path relative to the
			// output dir, so builds in different locations differ only there.
			if stripBannerLines(string(repoData)) != stripBannerLines(string(builtData)) {
				t.Errorf("%s: committed ui/%s is stale - run oma build", ex, f)
			}
		}

		entries, err := os.ReadDir(filepath.Join(dir, "ui"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || name == want+".qml" || name == "index.mjs" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, "ui", name))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), want+" {") {
				t.Errorf("%s: %s does not instantiate the %s bridge", ex, name, want)
			}
		}
	}
}
