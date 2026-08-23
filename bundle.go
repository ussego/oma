package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

// entryPoint returns the plugin's source entry: src/index.js, falling back to
// src/index.ts (TypeScript is stripped by esbuild; the bridge scanner reads
// both). An empty result means neither exists.
func entryPoint(dir string) string {
	for _, name := range []string{"index.js", "index.ts"} {
		p := filepath.Join(dir, "src", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// bundleProject bundles the plugin entry (src/index.js or src/index.ts) into
// ui/index.mjs, an ESM file Quickshell's QJSEngine loads directly. Target
// es2016 transpiles async arrows, object spread, class fields, optional
// chaining and nullish down to syntax QJSEngine parses (es2017 output keeps
// async expressions/arrows, which it rejects). TypeScript annotations are
// stripped by esbuild along the way. esbuild does NOT polyfill runtime APIs
// (e.g. Object.fromEntries) - keep those out of plugin code.
//
// BigInt is defined away because QJSEngine has none; libraries touching it at
// module scope (e.g. zod's BIGINT_FORMAT_RANGES) would otherwise throw on
// load. The globalthis shim gives QJSEngine a stand-in for the missing global.
//
// Imports resolve like standard Node: relative files and node_modules (run
// your package manager in the project to add libraries). The specifier
// "@oma/runtime" aliases the bundled runtime.
func bundleProject(dir string) error {
	// esbuild's stdin ResolveDir needs an absolute path.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	entry := entryPoint(abs)
	if entry == "" {
		return fmt.Errorf("missing src/index.js or src/index.ts (plugins start there)")
	}
	runtime, err := assetPath("oma.js")
	if err != nil {
		return err
	}
	shim, err := assetPath("globalthis-shim.mjs")
	if err != nil {
		return err
	}
	outDir := filepath.Join(abs, "ui")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	result := api.Build(api.BuildOptions{
		// Entry is a generated wrapper, not the plugin file itself: it
		// re-exports the plugin's exports plus the bridge bootstrap symbols
		// (persistence bind/unbind, deep-clone snap, debounce interval) the
		// generated bridge calls. A plain footer can't do this -
		// tree-shaking runs before footers exist and would strip them,
		// leaving a dangling reference that throws under QJSEngine.
		Stdin: &api.StdinOptions{
			Contents: fmt.Sprintf("export * from %q;\n"+
				"export { __omaBind as __omaBindRef, __omaUnbind as __omaUnbindRef, snap as __omaSnap, __omaDebounceMs as __omaDebounceMsRef } from %q;\n",
				filepath.ToSlash(entry), filepath.ToSlash(runtime)),
			Loader:     api.LoaderJS,
			ResolveDir: abs,
			Sourcefile: "plugin-entry.js",
		},
		Bundle:   true,
		Write:    true,
		Format:   api.FormatESModule,
		Target:   api.ES2016,
		Outfile:  filepath.Join(outDir, "index.mjs"),
		Define:   map[string]string{"BigInt": "Number"},
		Alias:    map[string]string{"@oma/runtime": runtime},
		Inject:   []string{shim},
		LogLevel: api.LogLevelSilent,
	})
	for _, e := range result.Errors {
		fmt.Fprintln(os.Stderr, formatMessage(e))
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("bundle failed (%d error(s))", len(result.Errors))
	}
	for _, w := range result.Warnings {
		fmt.Fprintln(os.Stderr, formatMessage(w))
	}
	return nil
}

func formatMessage(m api.Message) string {
	loc := ""
	if m.Location != nil {
		loc = fmt.Sprintf("%s:%d:%d: ", m.Location.File, m.Location.Line, m.Location.Column)
	}
	return loc + m.Text
}
