package main

import (
	"testing"
	"time"
)

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 kB"},
		{1536, "1.5 kB"},
		{1 << 20, "1.0 MB"},
		{2 << 20, "2.0 MB"},
	}
	for _, c := range cases {
		if got := humanSize(c.n); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestFmtDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{50 * time.Millisecond, "50ms"},
		{999 * time.Millisecond, "999ms"},
		{time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
	}
	for _, c := range cases {
		if got := fmtDur(c.d); got != c.want {
			t.Errorf("fmtDur(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	if got := displayPath("/home/tester/.config/x"); got != "~/.config/x" {
		t.Errorf("displayPath home = %q", got)
	}
	// absolute paths outside $HOME pass through unchanged
	if got := displayPath("/usr/share/oma"); got != "/usr/share/oma" {
		t.Errorf("displayPath absolute = %q", got)
	}
}
