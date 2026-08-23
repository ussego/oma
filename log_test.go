package main

import (
	"strings"
	"testing"
	"time"
)

func TestStripANSI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\x1b[34m DEBUG\x1b[97m qml\x1b[0m: msg", " DEBUG qml: msg"},
		{"\x1b[90m[READER]\x1b[0m x", "[READER] x"},
		{"\x1b[31m ERROR\x1b[0m scene\x1b[0m: boom", " ERROR scene: boom"},
		{"\x1b[2K", ""},
		{"plain", "plain"},
	}
	for _, c := range cases {
		if got := stripANSI(c.in); got != c.want {
			t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseLogLine(t *testing.T) {
	cases := []struct {
		in                   string
		level, category, msg string
	}{
		{" DEBUG qml: hello", "DEBUG", "qml", "hello"},
		{"\t WARN scene: @Main.qml[3:-1]: TypeError: x", "WARN", "scene", "@Main.qml[3:-1]: TypeError: x"},
		{"plain text", "", "", "plain text"},
		{" INFO qml: omarchy idle 2026-01-01T00:00:00Z ready", "INFO", "qml", "omarchy idle 2026-01-01T00:00:00Z ready"},
		{" INFO: Launching config: \"/usr/share/omarchy/shell/shell.qml\"", "INFO", "", "Launching config: \"/usr/share/omarchy/shell/shell.qml\""},
		{" WARN: no category", "WARN", "", "no category"},
	}
	for _, c := range cases {
		l := parseLogLine(c.in)
		if l.level != c.level || l.category != c.category || l.message != c.msg {
			t.Errorf("parseLogLine(%q) = %+v, want %s/%s/%q", c.in, l, c.level, c.category, c.msg)
		}
	}
}

func TestKeepLevel(t *testing.T) {
	cases := []struct {
		level, min string
		want       bool
	}{
		{"DEBUG", "debug", true},
		{"DEBUG", "warn", false},
		{"INFO", "info", true},
		{"WARN", "info", true},
		{"ERROR", "warn", true},
		{"", "error", true}, // unknown lines always pass
	}
	for _, c := range cases {
		l := logLine{level: c.level}
		if got := keepLevel(l, c.min); got != c.want {
			t.Errorf("keepLevel(%s, %s) = %v, want %v", c.level, c.min, got, c.want)
		}
	}
}

func TestLogLineJSON(t *testing.T) {
	l := logLine{level: "WARN", category: "scene", message: "boom"}
	want := `{"category":"scene","level":"warn","msg":"boom"}`
	if got := logLineJSON(l); got != want {
		t.Errorf("logLineJSON = %s, want %s", got, want)
	}
}

func TestParsePIDList(t *testing.T) {
	out := "1234 quickshell -n -p /other/shell\n5678 quickshell -n -p /usr/share/omarchy/shell\n9012 bash test\n"
	if pid, ok := parsePIDList(out); !ok || pid != 5678 {
		t.Fatalf("parsePIDList = %d, %v; want 5678, true", pid, ok)
	}
	if _, ok := parsePIDList("1234 some other process\n"); ok {
		t.Fatal("parsePIDList matched a non-omarchy process")
	}
}

// The rendered pretty line carries the level badge and the ● marker for
// matching lines; non-matching lines dim to plain text in --all mode.
func TestRenderLogLine(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	l := logLine{level: "ERROR", category: "qml", message: "boom", raw: " ERROR qml: boom"}

	marked := renderLogLine(l, th, true, true)
	if !strings.Contains(marked, "●") || !strings.Contains(marked, "ERROR") || !strings.Contains(marked, "boom") {
		t.Errorf("matching line missing marker/badge/message: %q", marked)
	}

	dimmed := renderLogLine(l, th, false, true)
	if strings.Contains(dimmed, "●") {
		t.Errorf("non-matching line should not get a marker: %q", dimmed)
	}
	if !strings.Contains(dimmed, "ERROR qml: boom") {
		t.Errorf("dimmed line lost content: %q", dimmed)
	}
}

func TestRenderLogLineCount(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	e := newLogEmitter(th, "", false, "debug", false, true)
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: single")
		e.flush()
	})
	if strings.Contains(stripCodes(out), "(×") {
		t.Fatalf("single line must not produce a count:\n%s", out)
	}
	if !strings.Contains(stripCodes(out), "single") {
		t.Fatalf("single line lost:\n%s", out)
	}
}

// Consecutive identical lines within the dedupe window print once; the (×N)
// count line follows when the burst breaks (non-terminal path).
func TestLogEmitterGrouping(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	now := time.Unix(0, 0)
	e := newLogEmitter(th, "", false, "debug", false, true)
	e.now = func() time.Time { return now }
	advance := func() { now = now.Add(100 * time.Millisecond) }
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: reload a")
		advance()
		e.emit(" DEBUG qml: reload a")
		advance()
		e.emit(" DEBUG qml: reload a")
		advance()
		e.emit(" INFO qml: done")
		advance()
		e.emit(" WARN scene: boom")
		advance()
		e.emit(" WARN scene: boom")
		e.flush()
	})
	out = stripCodes(out)
	if strings.Count(out, "reload a") != 1 {
		t.Fatalf("reload a should appear once:\n%s", out)
	}
	if !strings.Contains(out, "(×3)") {
		t.Fatalf("missing (×3) count line:\n%s", out)
	}
	if strings.Count(out, "boom") != 1 {
		t.Fatalf("boom should appear once:\n%s", out)
	}
	if !strings.Contains(out, "(×2)") {
		t.Fatalf("missing (×2) count line:\n%s", out)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("single line lost:\n%s", out)
	}
}

// On a terminal the (×N) count updates in place as duplicates arrive; flush
// terminates the count line so the next line starts fresh.
func TestLogEmitterTTYLiveCount(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	now := time.Unix(0, 0)
	e := newLogEmitter(th, "", false, "debug", false, true)
	e.tty = true
	e.now = func() time.Time { return now }
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: err")
		now = now.Add(100 * time.Millisecond)
		e.emit(" DEBUG qml: err")
		now = now.Add(100 * time.Millisecond)
		e.emit(" DEBUG qml: err")
		e.flush()
	})
	out = stripCodes(out)
	if strings.Count(out, "err") != 1 {
		t.Fatalf("tty path must print the line once:\n%s", out)
	}
	if !strings.Contains(out, "(×3)") {
		t.Fatalf("missing live count update:\n%s", out)
	}
	if !strings.Contains(out, "(×2)") {
		t.Fatalf("missing intermediate count update:\n%s", out)
	}
	if !strings.HasSuffix(out, "(×3)\n") {
		t.Fatalf("flush must terminate the live count line:\n%q", out)
	}
}

// After the live count line is terminated, the next distinct line starts on
// its own line (the user-visible "weird output" regression).
func TestLogEmitterTTYNextLineStartsFresh(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	now := time.Unix(0, 0)
	e := newLogEmitter(th, "", false, "debug", false, true)
	e.tty = true
	e.now = func() time.Time { return now }
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: reload")
		now = now.Add(100 * time.Millisecond)
		e.emit(" DEBUG qml: reload")
		now = now.Add(100 * time.Millisecond)
		e.emit(" INFO qml: next")
		e.flush()
	})
	out = stripCodes(out)
	if !strings.Contains(out, "(×2)\n") {
		t.Fatalf("count line must end before the next line:\n%q", out)
	}
	if !strings.Contains(out, "(×2)\nINFO  qml  next") {
		t.Fatalf("next line must start on its own line:\n%q", out)
	}
}

// Recurrence must never be hidden: the same line outside the dedupe window
// prints again as its own group.
func TestLogEmitterTimeBoundedDedupe(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	now := time.Unix(0, 0)
	e := newLogEmitter(th, "", false, "debug", false, true)
	e.now = func() time.Time { return now }
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: err")
		now = now.Add(200 * time.Millisecond) // within window: grouped
		e.emit(" DEBUG qml: err")
		now = now.Add(5 * time.Second) // outside window: must print again
		e.emit(" DEBUG qml: err")
		e.flush()
	})
	out = stripCodes(out)
	if strings.Count(out, "err") != 2 {
		t.Fatalf("recurring line must print twice, got:\n%s", out)
	}
	if strings.Count(out, "(×2)") != 1 || strings.Contains(out, "(×3)") {
		t.Fatalf("expected one (×2) group, got:\n%s", out)
	}
}

// One-shot mode (dedupe off) is lossless: identical consecutive lines all show.
func TestLogEmitterOneShotLossless(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	e := newLogEmitter(th, "", false, "debug", false, false)
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: dup")
		e.emit(" DEBUG qml: dup")
		e.emit(" DEBUG qml: dup")
		e.flush()
	})
	if strings.Count(stripCodes(out), "dup") != 3 {
		t.Fatalf("one-shot must show every line:\n%s", out)
	}
}

func TestLogEmitterLevelFilter(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	e := newLogEmitter(th, "", false, "warn", false, true)
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: hidden")
		e.emit(" WARN scene: shown")
		e.flush()
	})
	out = stripCodes(out)
	if strings.Contains(out, "hidden") {
		t.Fatalf("below-threshold line leaked:\n%s", out)
	}
	if !strings.Contains(out, "shown") {
		t.Fatalf("warn line missing:\n%s", out)
	}
}

func TestLogEmitterJSONLossless(t *testing.T) {
	th := theme{accent: "a0a0a0", foreground: "ffffff", muted: "666666", cyan: "00ff00", magenta: "a0a0a0", red: "ff0000", background: "000000", blue: "a0a0a0"}
	e := newLogEmitter(th, "", false, "debug", true, true)
	out := captureStdout(t, func() {
		e.emit(" DEBUG qml: dup")
		e.emit(" DEBUG qml: dup")
		e.flush()
	})
	if strings.Count(out, `"msg":"dup"`) != 2 {
		t.Fatalf("json output must stay lossless:\n%s", out)
	}
}
