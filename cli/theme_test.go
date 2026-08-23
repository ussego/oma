package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Without an Omarchy theme the TUI falls back to neutral defaults.
func TestCurrentThemeFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	th := currentTheme()
	if th.accent != "a0a0a0" || th.foreground != "a5a5a5" || th.background != "191919" ||
		th.muted != "666666" || th.red != "f38ba8" {
		t.Fatalf("fallback theme = %+v", th)
	}
}

// A real theme colors.toml is parsed; the first red wins and missing palette
// colors fall back to the accent.
func TestCurrentThemeParsesColors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	themeDir := filepath.Join(home, ".local", "state", "omarchy", "current", "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	colors := "accent = \"#ff0000\"\n" +
		"foreground=00ff00\n" +
		"red = #0000ff\n" +
		"bright_red = #123456\n" + // second red ignored
		"notacolor = #ffffff\n"
	if err := os.WriteFile(filepath.Join(themeDir, "colors.toml"), []byte(colors), 0o644); err != nil {
		t.Fatal(err)
	}

	th := currentTheme()
	if th.accent != "ff0000" || th.foreground != "00ff00" || th.red != "0000ff" {
		t.Fatalf("theme = %+v", th)
	}
	// no cyan/magenta/blue declared: they fall back to the accent
	if th.cyan != "ff0000" || th.magenta != "ff0000" || th.blue != "ff0000" {
		t.Fatalf("palette fallbacks = %+v", th)
	}
}

func TestColorLineRegex(t *testing.T) {
	cases := []struct {
		line    string
		key     string
		val     string
		matches bool
	}{
		{`accent = #ff0000`, "accent", "ff0000", true},
		{`accent = "#ff0000"`, "accent", "ff0000", true},
		{`foreground = 00ff00`, "foreground", "00ff00", true},
		{`red = "ABCDEF"`, "red", "ABCDEF", true},
		{`accent = red`, "", "", false},
		{`accent = #ff00`, "", "", false},
		{`accent = "#ff00"`, "", "", false},
		{`accent=`, "", "", false},
	}
	for _, c := range cases {
		m := colorLine.FindStringSubmatch(c.line)
		if c.matches {
			if m == nil || m[1] != c.key || m[2] != c.val {
				t.Errorf("%q: got %v, want %s=%s", c.line, m, c.key, c.val)
			}
		} else if m != nil {
			t.Errorf("%q matched unexpectedly: %v", c.line, m)
		}
	}
}

// The logo animation settles at logoDoneFrame and both rows stay intact.
func TestOmaLogoFrames(t *testing.T) {
	th := theme{accent: "ff0000", cyan: "00ff00", magenta: "0000ff", muted: "666666"}
	static := omaLogo(th)
	for _, frame := range []int{0, 3, logoDoneFrame, logoDoneFrame + 5} {
		out := omaLogoFrame(th, frame)
		if frame >= logoDoneFrame {
			if out != static {
				t.Fatalf("frame %d not settled:\n%q\nwant:\n%q", frame, out, static)
			}
			if !strings.Contains(out, "one project") || !strings.Contains(out, "multiple surfaces") {
				t.Fatalf("frame %d missing taglines:\n%q", frame, out)
			}
		}
	}
}
