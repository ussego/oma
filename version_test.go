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
