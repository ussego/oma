package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// oma log: pretty viewer over the running omarchy shell's quickshell log.
// The shell's stdout/stderr feed Quickshell's per-instance log buffer, read
// back through `qs log --pid <pid>`. Plugin console.log output (QML + JS)
// lands there under the qml category.

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// stripANSI removes SGR/CSI escape sequences from qs log output.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

type logLine struct {
	level    string // DEBUG | TRACE | INFO | WARN | ERROR | "" (unknown)
	category string
	message  string
	raw      string // the stripped line, for dim rendering
}

var logLineRe = regexp.MustCompile(`^[ \t]*(DEBUG|TRACE|INFO|WARN|ERROR)[ \t]+([^:]+):[ \t]?(.*)$`)

// boot lines have no category: `INFO: Launching config ...`
var logBareLevelRe = regexp.MustCompile(`^[ \t]*(DEBUG|TRACE|INFO|WARN|ERROR):[ \t]?(.*)$`)

func parseLogLine(s string) logLine {
	if m := logLineRe.FindStringSubmatch(s); m != nil {
		return logLine{level: m[1], category: m[2], message: m[3], raw: s}
	}
	if m := logBareLevelRe.FindStringSubmatch(s); m != nil {
		return logLine{level: m[1], message: m[2], raw: s}
	}
	return logLine{message: s, raw: s}
}

var levelRank = map[string]int{"trace": 0, "debug": 1, "info": 2, "warn": 3, "error": 4}

// keepLevel reports whether a line passes the --level threshold. Unknown
// lines always pass through.
func keepLevel(l logLine, min string) bool {
	if l.level == "" {
		return true
	}
	return levelRank[strings.ToLower(l.level)] >= levelRank[min]
}

func logLineJSON(l logLine) string {
	data, _ := json.Marshal(map[string]string{
		"level":    strings.ToLower(l.level),
		"category": l.category,
		"msg":      l.message,
	})
	return string(data)
}

// parsePIDList picks the first quickshell pid whose command line mentions
// omarchy (the shell runs as `quickshell -n -p <omarchy>/shell`).
func parsePIDList(out string) (int, bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.Contains(line, "omarchy") {
			if pid, err := strconv.Atoi(fields[0]); err == nil {
				return pid, true
			}
		}
	}
	return 0, false
}

func findShellPID() (int, error) {
	out, err := exec.Command("pgrep", "-af", "quickshell").Output()
	if err != nil {
		return 0, fmt.Errorf("could not find the omarchy shell: %w", err)
	}
	if pid, ok := parsePIDList(string(out)); ok {
		return pid, nil
	}
	return 0, fmt.Errorf("omarchy shell is not running (start it with 'omarchy restart shell')")
}

func logBadge(l logLine, t theme) string {
	st := lipgloss.NewStyle().Foreground(hexColor(t.muted))
	switch l.level {
	case "INFO":
		st = lipgloss.NewStyle().Foreground(hexColor(t.cyan))
	case "WARN":
		st = lipgloss.NewStyle().Foreground(hexColor(t.magenta))
	case "ERROR":
		st = lipgloss.NewStyle().Foreground(hexColor(t.red)).Bold(true)
	}
	return st.Render(fmt.Sprintf("%-5s", l.level))
}

// renderLogLine renders one line theme-aware. marker enables the cyan ● on
// matching lines and dimming for non-matching ones (--all mode).
func renderLogLine(l logLine, t theme, matched, marker bool) string {
	if marker && !matched {
		dim := lipgloss.NewStyle().Foreground(hexColor(t.muted))
		return dim.Render(l.raw)
	}
	mark := ""
	if marker {
		mark = lipgloss.NewStyle().Foreground(hexColor(t.cyan)).Bold(true).Render("●") + " "
	}
	category := lipgloss.NewStyle().Foreground(hexColor(t.muted)).Render(l.category)
	message := lipgloss.NewStyle().Foreground(hexColor(t.foreground)).Render(l.message)
	if l.category == "" {
		return mark + message
	}
	return mark + logBadge(l, t) + " " + category + "  " + message
}

// logDedupeWindow bounds burst collapsing: identical consecutive lines only
// group when they arrive within this window. Repeats separated by more than
// the window print again — deduplication must never hide recurrence.
const logDedupeWindow = time.Second

type logGroup struct {
	line    logLine
	matched bool
	count   int
	last    time.Time
}

// logEmitter filters and pretty-groups lines. Every line prints immediately
// (never buffered — follow mode must always show output). With dedupe on
// (follow mode), identical consecutive lines arriving within logDedupeWindow
// collapse to one line with a (×N) count: on a terminal the count line
// updates in place as duplicates arrive (live), on pipes it prints when the
// burst breaks. One-shot mode (dedupe off) is lossless — no arrival
// timestamps, so nothing is collapsed. JSON passes through losslessly.
type logEmitter struct {
	t         theme
	id        string // plugin id, "" when --all
	all       bool
	min       string
	jsonOut   bool
	dedupe    bool
	tty       bool
	window    time.Duration
	now       func() time.Time
	group     *logGroup
	countLive bool // count line already rendered live on the terminal
}

func newLogEmitter(t theme, id string, all bool, min string, jsonOut bool, dedupe bool) *logEmitter {
	return &logEmitter{
		t: t, id: id, all: all, min: min, jsonOut: jsonOut, dedupe: dedupe,
		window: logDedupeWindow, now: time.Now,
	}
}

func (e *logEmitter) emit(line string) {
	clean := stripANSI(line)
	l := parseLogLine(clean)
	if !keepLevel(l, e.min) {
		return
	}
	matched := e.id == "" || strings.Contains(clean, e.id)
	if e.id != "" && !matched && !e.all {
		return
	}
	if e.jsonOut {
		fmt.Println(logLineJSON(l))
		return
	}
	if !e.dedupe {
		fmt.Println(renderLogLine(l, e.t, matched, e.id != ""))
		return
	}
	now := e.now()
	if e.group != nil && (e.group.line.raw != l.raw || e.group.matched != matched || now.Sub(e.group.last) > e.window) {
		e.flush()
	}
	if e.group == nil {
		e.group = &logGroup{line: l, matched: matched, count: 1, last: now}
		e.countLive = false
		fmt.Println(renderLogLine(l, e.t, matched, e.id != ""))
		return
	}
	e.group.count++
	e.group.last = now
	if e.tty {
		// update the count line in place; short line, so \r is wrap-safe
		fmt.Print("\r\x1b[2K  " + e.countLabel())
		e.countLive = true
	}
}

// countLabel renders the group's "(×N)" marker.
func (e *logEmitter) countLabel() string {
	dim := lipgloss.NewStyle().Foreground(hexColor(e.t.muted))
	return dim.Render(fmt.Sprintf("(×%d)", e.group.count))
}

// flush closes the group; on pipes it also prints the pending count line, on
// terminals it terminates the live count line so the next line starts fresh.
func (e *logEmitter) flush() {
	if e.group == nil {
		return
	}
	if e.countLive {
		fmt.Println()
	} else if e.group.count > 1 {
		fmt.Println("  " + e.countLabel())
	}
	e.group = nil
	e.countLive = false
}

// stdoutIsTTY reports whether stdout is a terminal (live in-place count
// updates only make sense there; pipes get the count line at burst end).
func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// processExists reports whether a pid is alive (watchdog for shell restarts).
func processExists(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}

// runLog implements `oma log` (one-shot) and `oma tail` (follow alias).
func runLog(dir string, args []string) error {
	n := 100
	follow := false
	all := false
	minLevel := "debug"
	jsonOut := false
	pid := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s needs a value", a)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "-n" || a == "--lines":
			v, err := next()
			if err != nil {
				return err
			}
			n, err = strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("invalid --lines %q", v)
			}
		case strings.HasPrefix(a, "--lines="):
			v, err := strconv.Atoi(strings.TrimPrefix(a, "--lines="))
			if err != nil || v < 0 {
				return fmt.Errorf("invalid --lines %q", strings.TrimPrefix(a, "--lines="))
			}
			n = v
		case a == "-f" || a == "--follow":
			follow = true
		case a == "--all":
			all = true
		case a == "--json":
			jsonOut = true
		case a == "--pid":
			v, err := next()
			if err != nil {
				return err
			}
			pid, err = strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --pid %q", v)
			}
		case strings.HasPrefix(a, "--pid="):
			v, err := strconv.Atoi(strings.TrimPrefix(a, "--pid="))
			if err != nil {
				return fmt.Errorf("invalid --pid %q", strings.TrimPrefix(a, "--pid="))
			}
			pid = v
		case a == "--level":
			v, err := next()
			if err != nil {
				return err
			}
			minLevel = strings.ToLower(v)
		case strings.HasPrefix(a, "--level="):
			minLevel = strings.ToLower(strings.TrimPrefix(a, "--level="))
		default:
			return fmt.Errorf("unknown flag %q", a)
		}
	}
	switch minLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("unknown --level %q (debug, info, warn or error)", minLevel)
	}
	if pid == 0 {
		var err error
		pid, err = findShellPID()
		if err != nil {
			return err
		}
	}

	// plugin id for filtering; run inside a project dir
	id := ""
	if m, err := readManifest(filepath.Join(dir, "manifest.json")); err == nil && m.ID != "" {
		id = m.ID
	}
	if all {
		id = ""
	}
	if id == "" && !all {
		return fmt.Errorf("no manifest.json in %s - run inside a project or use --all", dir)
	}

	t := currentTheme()
	header := fmt.Sprintf("omarchy-shell (pid %d)", pid)
	if id != "" {
		header = fmt.Sprintf("%s — %s", id, header)
	}
	verb := fmt.Sprintf("last %d lines", n)
	if follow {
		verb = "following"
	}
	mark := lipgloss.NewStyle().Foreground(hexColor(t.accent)).Bold(true).Render("○")
	plain := lipgloss.NewStyle().Foreground(hexColor(t.foreground))
	fmt.Println(mark + " " + plain.Render(fmt.Sprintf("%s — %s", header, verb)))

	emitter := newLogEmitter(t, id, all, minLevel, jsonOut, follow)
	emitter.tty = stdoutIsTTY()

	qsArgs := []string{"log", "--pid", strconv.Itoa(pid)}
	if follow {
		qsArgs = append(qsArgs, "-f")
	} else if n > 0 {
		qsArgs = append(qsArgs, "-t", strconv.Itoa(n))
	}
	cmd := exec.Command("qs", qsArgs...)
	cmd.Stderr = os.Stderr

	if !follow {
		// one-shot: lossless — no arrival timestamps, so nothing is deduped
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("qs log: %w", err)
		}
		for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if line == "" {
				continue
			}
			emitter.emit(line)
		}
		return nil
	}

	// follow: stream lines as they arrive. The loop reconnects when the
	// reader dies or the shell restarts (oma restart, session restarts) — a
	// log viewer that dies with the shell is useless. On Ctrl+C/SIGTERM the
	// active reader is killed hard before exiting — an orphaned reader aborts
	// on pipe close (QThread destroyed while running, core dump).
	var mu sync.Mutex
	var active *exec.Cmd
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		mu.Lock()
		c := active
		mu.Unlock()
		if c != nil {
			_ = c.Process.Kill()
		}
		os.Exit(0)
	}()

	shellPID := pid
	firstRun := true
	for {
		if !firstRun {
			// reader ended: re-resolve the shell (it may have restarted) and
			// reconnect, waiting politely while it is down
			time.Sleep(300 * time.Millisecond)
			next, err := findShellPID()
			dim := lipgloss.NewStyle().Foreground(hexColor(t.muted))
			if err != nil {
				fmt.Println(dim.Render("→ omarchy-shell not running — waiting…"))
				for {
					time.Sleep(2 * time.Second)
					if next, err = findShellPID(); err == nil {
						break
					}
				}
			}
			if next != shellPID {
				fmt.Println(dim.Render(fmt.Sprintf("→ omarchy-shell restarted (pid %d) — continuing", next)))
			} else {
				fmt.Println(dim.Render("→ log reader reconnected"))
			}
			shellPID = next
		}
		firstRun = false

		cmd = exec.Command("qs", "log", "--pid", strconv.Itoa(shellPID), "-f")
		cmd.Stderr = os.Stderr
		mu.Lock()
		active = cmd
		mu.Unlock()
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("qs log: %w", err)
		}

		// watchdog: when the followed shell dies, kill the reader so the
		// reconnect loop takes over instead of following a dead log file
		watchdogStop := make(chan struct{})
		watchdogDone := make(chan struct{})
		go func() {
			defer close(watchdogDone)
			for {
				select {
				case <-watchdogStop:
					return
				case <-time.After(2 * time.Second):
					if !processExists(shellPID) {
						mu.Lock()
						if active == cmd {
							_ = cmd.Process.Kill()
						}
						mu.Unlock()
						return
					}
				}
			}
		}()

		lines := make(chan string, 256)
		go func() {
			sc := bufio.NewScanner(stdout)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				lines <- sc.Text()
			}
			close(lines)
		}()

		var timer *time.Timer
		var timerC <-chan time.Time
		armTimer := func() {
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(logDedupeWindow)
			timerC = timer.C
		}
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					emitter.flush()
					goto readerEnd
				}
				if line == "" {
					continue
				}
				emitter.emit(line)
				armTimer()
			case <-timerC:
				emitter.flush()
				timer = nil
				timerC = nil
			}
		}
	readerEnd:
		close(watchdogStop)
		<-watchdogDone
		_ = cmd.Wait()
	}
}
