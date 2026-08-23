package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		bi   *debug.BuildInfo
		want string
	}{
		{"module version strips v prefix", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.2"}}, "0.1.2"},
		{"module version without v prefix", &debug.BuildInfo{Main: debug.Module{Version: "0.1.2"}}, "0.1.2"},
		{"dirty tagged build keeps truth", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.2+dirty"}}, "0.1.2+dirty"},
		{"local build", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"empty version", &debug.BuildInfo{}, "dev"},
		{"nil build info", nil, "dev"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionFromBuildInfo(tc.bi); got != tc.want {
				t.Fatalf("versionFromBuildInfo(%+v) = %q, want %q", tc.bi, got, tc.want)
			}
		})
	}
}

func TestSchemaURLFor(t *testing.T) {
	const base = "https://raw.githubusercontent.com/ussego/oma/"
	cases := []struct {
		name    string
		version string
		want    string
	}{
		{"release tag", "0.1.5", base + "v0.1.5/assets/schemas/oma.json"},
		{"dev", "dev", base + "main/assets/schemas/oma.json"},
		{"dirty tagged build", "0.1.5+dirty", base + "main/assets/schemas/oma.json"},
		{"go pseudo-version", "0.1.5-0.20260823152300-abc123def456", base + "main/assets/schemas/oma.json"},
		{"empty", "", base + "main/assets/schemas/oma.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schemaURLFor(tc.version); got != tc.want {
				t.Fatalf("schemaURLFor(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestSchemaVersionFromURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"pinned tag", "https://raw.githubusercontent.com/ussego/oma/v0.1.5/assets/schemas/oma.json", "0.1.5"},
		{"main fallback", "https://raw.githubusercontent.com/ussego/oma/main/assets/schemas/oma.json", ""},
		{"unrelated url", "https://example.com/schema.json", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schemaVersionFromURL(tc.url); got != tc.want {
				t.Fatalf("schemaVersionFromURL(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
