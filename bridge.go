package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Static analysis of src/index.js: exports declared as `state({...})` become
// reactive QML properties on the generated bridge QtObject; exported functions
// become bridge methods. Plugin code is never evaluated, so builds stay fast,
// deterministic and dependency-free.

type FieldMeta struct{ Name, Type string }

type StateMeta struct {
	Name   string
	Fields []FieldMeta
}

type ModuleMeta struct {
	States    []StateMeta
	Functions []string
}

// scanResult carries notes surfaced to the developer during build.
type scanNotes struct {
	ignored []string // exports recognized but deliberately not bridged
}

var qmlPropRe = regexp.MustCompile(`^[a-z_][A-Za-z0-9_$]*$`)
var identRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
var lowerIdentRe = regexp.MustCompile(`^[a-z_$][A-Za-z0-9_$]*`)
var numStartRe = regexp.MustCompile(`^[+-]?(\.\d|\d)`)

// scanBridge extracts the bridge metadata from plugin source. Comments and
// strings are neutralized first; a parallel mask marks which bytes are
// top-level code so export keywords inside literals are never matched.
func scanBridge(src string) (ModuleMeta, scanNotes, error) {
	var meta ModuleMeta
	var notes scanNotes
	seenState := map[string]bool{}
	seenFn := map[string]bool{}

	cleaned, mask := stripAndMask(src)
	for _, pos := range findExports(cleaned, mask) {
		rest := cleaned[pos+6:]
		line := offsetLine(cleaned, pos)
		word, after := readWord(rest)
		tail := rest[after:]
		// readWord only reads identifiers: `export {` and `export *` forms
		// surface as single-char tokens
		if word == "" && len(tail) > 0 && (tail[0] == '{' || tail[0] == '*') {
			word = string(tail[0])
		}
		switch word {
		case "default":
			notes.ignored = append(notes.ignored, fmt.Sprintf("line %d: export default is not bridged", line))
			continue
		case "{":
			end := matchBracket(tail, 0, '{', '}')
			notes.ignored = append(notes.ignored, fmt.Sprintf("line %d: export {%s} form is not scanned; declare exports directly in src/index.js", line, strings.TrimSpace(tail[1:end-1])))
			continue
		case "*":
			notes.ignored = append(notes.ignored, fmt.Sprintf("line %d: export * is not scanned; declare exports directly in src/index.js", line))
			continue
		case "async":
			w2, w2len := readWord(tail)
			if w2 != "function" {
				continue // async IIFE or stray - not a declaration
			}
			fnName(tail[w2len:], &meta, seenFn)
			continue
		case "function":
			fnName(tail, &meta, seenFn)
			continue
		case "const", "let", "var":
			name, p2 := readWord(tail)
			if name == "" || !identRe.MatchString(name) {
				continue
			}
			expr, err := initializerExpr(tail[p2:])
			if err != nil {
				return meta, notes, fmt.Errorf("export %s (line %d): %w", name, line, err)
			}
			if err := classifyInit(name, expr, &meta, &notes, seenState, seenFn); err != nil {
				return meta, notes, fmt.Errorf("export %s (line %d): %w", name, line, err)
			}
		}
	}
	return meta, notes, nil
}

// findExports returns offsets of top-level `export` keyword tokens.
func findExports(s string, mask []bool) []int {
	var out []int
	for i := 0; i+6 <= len(s); {
		if !mask[i] {
			i++
			continue
		}
		if s[i:i+6] == "export" &&
			(i == 0 || !isIdentByte(s[i-1])) &&
			(i+6 >= len(s) || !isIdentByte(s[i+6])) {
			out = append(out, i)
			i += 6
			continue
		}
		i++
	}
	return out
}

func fnName(s string, meta *ModuleMeta, seen map[string]bool) {
	s = strings.TrimLeft(strings.TrimLeft(s, " \t"), "*") // generator star
	name, _ := readWord(s)
	if name == "" || !lowerIdentRe.MatchString(name) {
		return // anonymous or uppercase-initial: not bridged as an action
	}
	if !seen[name] {
		seen[name] = true
		meta.Functions = append(meta.Functions, name)
	}
}

// classifyInit decides what an exported initializer means for the bridge.
func classifyInit(name, expr string, meta *ModuleMeta, notes *scanNotes, seenState map[string]bool, seenFn map[string]bool) error {
	head, arg, hasArg := callHead(expr)
	switch head {
	case "state":
		if !hasArg {
			return fmt.Errorf("state() needs an argument")
		}
		body := strings.TrimSpace(arg)
		// matchBracket tolerates TS suffixes inside the parens (`{...} as const`,
		// `{...} satisfies Shape`) by taking exactly the literal's extent.
		if !strings.HasPrefix(body, "{") {
			return fmt.Errorf("state() needs an object literal ({...}); arrays, class instances and identifiers can't be bridged")
		}
		litEnd := matchBracket(body, 0, '{', '}')
		if litEnd < 0 {
			return fmt.Errorf("unterminated object literal")
		}
		fields, err := parseFields(body[1:litEnd-1], name)
		if err != nil {
			return err
		}
		if !seenState[name] {
			seenState[name] = true
			meta.States = append(meta.States, StateMeta{Name: name, Fields: fields})
		}
	case "derived":
		notes.ignored = append(notes.ignored, fmt.Sprintf("derived state %q is not bridged (only state({...}) exports become QML properties)", name))
	case "config":
		// methods-only instance, nothing to bridge
	default:
		if isFunctionExpr(expr) {
			if lowerIdentRe.MatchString(name) && !seenFn[name] {
				seenFn[name] = true
				meta.Functions = append(meta.Functions, name)
			}
		}
		// everything else (plain data, imports, components) is not bridged
	}
	return nil
}

// callHead reports the callee identifier of an expression like `state({...})`,
// plus its first parenthesized argument when present.
func callHead(expr string) (head string, arg string, ok bool) {
	i := 0
	for i < len(expr) && (expr[i] == ' ' || expr[i] == '\t') {
		i++
	}
	start := i
	for i < len(expr) && isIdentByte(expr[i]) {
		i++
	}
	head = expr[start:i]
	j := i
	for j < len(expr) && (expr[j] == ' ' || expr[j] == '\t') {
		j++
	}
	// TypeScript: skip generic arguments (state<T>({...})) when they sit
	// between the callee and its parens. A bare `<` could be a comparison, so
	// only consume it when a matching `>` is followed by `(`.
	if j < len(expr) && expr[j] == '<' {
		if end := matchAngle(expr, j); end > 0 {
			k := end
			for k < len(expr) && (expr[k] == ' ' || expr[k] == '\t' || expr[k] == '\n' || expr[k] == '\r') {
				k++
			}
			if k < len(expr) && expr[k] == '(' {
				j = k
			}
		}
	}
	if j >= len(expr) || expr[j] != '(' {
		return head, "", false
	}
	end := matchBracket(expr, j, '(', ')')
	if end < 0 {
		return head, "", false
	}
	return head, expr[j+1 : end-1], true
}

// isFunctionExpr detects `function` declarations and arrow functions,
// tolerating TypeScript annotations (`(a: T): R => ...`, generator stars).
func isFunctionExpr(expr string) bool {
	t := strings.TrimLeft(expr, " \t")
	if strings.HasPrefix(t, "async") {
		rest := strings.TrimSpace(t[5:])
		if strings.HasPrefix(rest, "function") {
			return true
		}
		t = rest
	} else if strings.HasPrefix(t, "function") {
		return true
	}
	// arrow: `x =>`, `(params) =>`, or `(params): RetType =>` at expression start
	if strings.HasPrefix(t, "(") {
		end := matchBracket(t, 0, '(', ')')
		if end < 0 {
			return false
		}
		return hasArrow(t[end:])
	}
	if m := lowerIdentRe.FindString(t); m != "" {
		return hasArrow(t[len(m):])
	}
	return false
}

// hasArrow reports whether `=>` appears at bracket depth 0 within the first
// stretch of s (after a possible TS return-type annotation). Depth counting
// keeps arrows inside default values or type literals from qualifying.
func hasArrow(s string) bool {
	depth := 0
	for i := 0; i < len(s) && i < 512; i++ {
		c := s[i]
		if c == '\'' || c == '"' || c == '`' {
			j := skipString(s, i)
			if j < 0 {
				return false
			}
			i = j - 1
			continue
		}
		switch c {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return false
			}
		case '>':
			if depth > 0 {
				depth--
			}
		case ';':
			return false
		case '=':
			if i+1 < len(s) && s[i+1] == '>' && depth == 0 {
				return true
			}
			// other operators around a type annotation: keep scanning
		}
	}
	return false
}

// matchAngle returns the index just past the `>` closing the generic argument
// list opening at start, or -1. Nested angles count; a statement terminator
// bails out so comparisons never match.
func matchAngle(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return i + 1
			}
		case ';', '\n':
			return -1
		}
	}
	return -1
}

// initializerExpr captures the expression after `=`: balanced groups greedily,
// stopping at a top-level `;` or a newline not continued by an operator.
func initializerExpr(s string) (string, error) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || s[i] != '=' || (i+1 < len(s) && s[i+1] == '=') {
		return "", fmt.Errorf("expected `=` initializer")
	}
	i++ // skip '='
	start := i
	depth := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			j := skipString(s, i)
			if j < 0 {
				return "", fmt.Errorf("unterminated string")
			}
			i = j
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth < 0 {
				return "", fmt.Errorf("unbalanced brackets")
			}
		case depth == 0 && c == ';':
			return s[start:i], nil
		case depth == 0 && c == '\n':
			next := i + 1
			for next < len(s) && (s[next] == ' ' || s[next] == '\t' || s[next] == '\n' || s[next] == '\r') {
				next++
			}
			if next >= len(s) {
				return s[start:i], nil
			}
			if !strings.ContainsRune(".),:}+?|&=", rune(s[next])) {
				return s[start:i], nil
			}
		}
		i++
	}
	return s[start:], nil
}

// parseFields reads the top-level entries of an object literal body and infers
// QML property types from the initializers (nested objects/arrays collapse to
// type `var`).
func parseFields(body, owner string) ([]FieldMeta, error) {
	var fields []FieldMeta
	seen := map[string]bool{}
	i := 0
	for i < len(body) {
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r' || body[i] == ',') {
			i++
		}
		if i >= len(body) {
			break
		}
		// key
		var key string
		switch {
		case body[i] == '.':
			return nil, fmt.Errorf("spread fields (...%s) can't be bridged statically; list the fields literally", restOf(body, i))
		case body[i] == '[':
			return nil, fmt.Errorf("computed keys ([...]) can't be bridged statically; use fixed names")
		case body[i] == '\'' || body[i] == '"':
			j := skipString(body, i)
			if j < 0 {
				return nil, fmt.Errorf("unterminated string")
			}
			key = body[i+1 : j-1]
			i = j
		default:
			k := i
			for i < len(body) && isIdentByte(body[i]) {
				i++
			}
			key = body[k:i]
		}
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
			i++
		}
		if i >= len(body) || body[i] != ':' {
			return nil, fmt.Errorf("field %q: expected `:` (shorthand fields aren't supported)", key)
		}
		i++ // skip ':'
		// value runs to the next top-level comma
		valStart := i
		depth := 0
		for i < len(body) {
			c := body[i]
			if c == '\'' || c == '"' || c == '`' {
				j := skipString(body, i)
				if j < 0 {
					return nil, fmt.Errorf("unterminated string")
				}
				i = j
				continue
			}
			switch c {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			case ',':
				if depth == 0 {
					goto valueDone
				}
			}
			i++
		}
	valueDone:
		value := strings.TrimSpace(body[valStart:i])
		typ := inferType(value)
		if !qmlPropRe.MatchString(key) {
			return nil, fmt.Errorf("field %q is not a valid QML property name (must start lowercase)", key)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate field %q", key)
		}
		seen[key] = true
		fields = append(fields, FieldMeta{Name: key, Type: typ})
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("has no fields to bridge")
	}
	return fields, nil
}

func inferType(value string) string {
	v := trimTSAs(strings.TrimLeft(value, " \t\n\r"))
	switch {
	case v == "true" || v == "false":
		return "bool"
	case numStartRe.MatchString(v):
		return "double"
	case strings.HasPrefix(v, "'"), strings.HasPrefix(v, "\""), strings.HasPrefix(v, "`"):
		return "string"
	default:
		return "var"
	}
}

// trimTSAs removes a trailing TypeScript `as T` / `satisfies T` clause so
// literal inference still works (`true as const`, `0 as number`). Only a
// top-level (bracket-depth 0) clause counts.
func trimTSAs(v string) string {
	depth := 0
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '\'' || c == '"' || c == '`' {
			j := skipString(v, i)
			if j < 0 {
				return v
			}
			i = j - 1
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			depth--
			continue
		}
		if depth != 0 || c != ' ' {
			continue
		}
		rest := v[i+1:]
		if strings.HasPrefix(rest, "as ") || strings.HasPrefix(rest, "as\t") ||
			strings.HasPrefix(rest, "as\n") || strings.HasPrefix(rest, "satisfies ") {
			return strings.TrimRight(v[:i], " \t")
		}
	}
	return v
}

func restOf(s string, i int) string {
	r := s[i:]
	if len(r) > 20 {
		r = r[:20] + "..."
	}
	return r
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// readWord returns the identifier starting at s (possibly "") and the offset
// just past it.
func readWord(s string) (string, int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	start := i
	for i < len(s) && isIdentByte(s[i]) {
		i++
	}
	return s[start:i], i
}

// matchBracket returns the index just past the bracket matching the one at
// start, or -1. Strings are skipped; comments are assumed stripped.
func matchBracket(s string, start int, open, close byte) int {
	depth := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '\'' || c == '"' || c == '`' {
			j := skipString(s, i)
			if j < 0 {
				return -1
			}
			i = j - 1
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// skipString returns the index just past the closing quote of the string
// starting at s[i], or -1.
func skipString(s string, i int) int {
	q := s[i]
	i++
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == q {
			return i + 1
		}
		i++
	}
	return -1
}

// offsetLine converts a byte offset to a 1-based line number for messages.
func offsetLine(src string, pos int) int {
	line := 1
	for i := 0; i < pos && i < len(src); i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}

// stripAndMask replaces comment bytes with spaces (keeping newlines so
// offsets survive) while leaving string and template-literal contents alone,
// including ${...} interpolations via a mode stack. The returned mask marks
// which bytes were scanned as top-level code.
func stripAndMask(src string) (string, []bool) {
	const (
		mCode = iota
		mSingle
		mDouble
		mTemplate
		mLine
		mBlock
	)
	type frame struct {
		mode  int
		depth int // brace depth for code frames opened from ${ }
	}
	out := []byte(src)
	mask := make([]bool, len(src))
	stack := []frame{{mode: mCode}}
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		top := &stack[len(stack)-1]
		if len(stack) == 1 && top.mode == mCode {
			mask[i] = true
		}
		switch top.mode {
		case mSingle:
			if c == '\\' {
				i += 2
				continue
			}
			if c == '\'' {
				stack = stack[:len(stack)-1]
			}
			i++
		case mDouble:
			if c == '\\' {
				i += 2
				continue
			}
			if c == '"' {
				stack = stack[:len(stack)-1]
			}
			i++
		case mTemplate:
			if c == '\\' {
				i += 2
				continue
			}
			if c == '`' {
				stack = stack[:len(stack)-1]
				i++
				continue
			}
			if c == '$' && i+1 < n && src[i+1] == '{' {
				stack = append(stack, frame{mode: mCode})
				i += 2
				continue
			}
			i++
		case mLine:
			if c == '\n' {
				stack = stack[:len(stack)-1]
				i++
				continue
			}
			out[i] = ' '
			i++
		case mBlock:
			if c == '*' && i+1 < n && src[i+1] == '/' {
				out[i] = ' '
				out[i+1] = ' '
				stack = stack[:len(stack)-1]
				i += 2
				continue
			}
			if c == '\n' {
				i++ // keep newlines
				continue
			}
			out[i] = ' '
			i++
		default: // code
			switch {
			case c == '\'':
				stack = append(stack, frame{mode: mSingle})
				i++
			case c == '"':
				stack = append(stack, frame{mode: mDouble})
				i++
			case c == '`':
				stack = append(stack, frame{mode: mTemplate})
				i++
			case c == '/' && i+1 < n && src[i+1] == '/':
				stack = append(stack, frame{mode: mLine})
				i += 2
			case c == '/' && i+1 < n && src[i+1] == '*':
				stack = append(stack, frame{mode: mBlock})
				out[i] = ' '
				out[i+1] = ' '
				i += 2
			case len(stack) > 1: // ${ } interpolation frame: count braces to its close
				if c == '{' {
					top.depth++
				} else if c == '}' {
					if top.depth == 0 {
						stack = stack[:len(stack)-1]
						// the '}' belongs to ${ } and stays in the template
					} else {
						top.depth--
					}
				}
				i++
			default:
				i++
			}
		}
	}
	return string(out), mask
}

// renderBridge emits the generated bridge QtObject: one auto-NOTIFY property
// per state field, one delegating method per action, a subscription loop that
// pushes JS-side changes into the properties, and the persistence bootstrap
// that feeds config() stores from the standard per-plugin settings file
// ~/.config/omarchy/<id>.json.
func renderBridge(meta ModuleMeta, jsModule, pluginID string) string {
	props := make([]string, 0, len(meta.States))
	for _, s := range meta.States {
		for _, f := range s.Fields {
			props = append(props, fmt.Sprintf("  property %s %s", f.Type, f.Name))
		}
	}
	fns := make([]string, 0, len(meta.Functions))
	for _, f := range meta.Functions {
		fns = append(fns, fmt.Sprintf("  function %s() { return Logic.%s.apply(null, arguments) }", f, f))
	}
	sync := make([]string, 0, len(meta.States))
	for i, s := range meta.States {
		assigns := make([]string, 0, len(s.Fields))
		for _, f := range s.Fields {
			assigns = append(assigns, fmt.Sprintf("        root.%s = Logic.%s.%s", f.Name, s.Name, f.Name))
		}
		sync = append(sync,
			fmt.Sprintf("    var apply%d = function() {\n%s\n    }\n    apply%d()\n    unsubscribers.push(Logic.%s.subscribe(apply%d))",
				i, strings.Join(assigns, "\n"), i, s.Name, i))
	}
	body := strings.Join(append([]string{"  property var unsubscribers: []"}, append(props, fns...)...), "\n")

	persistence := strings.Join([]string{
		"  // Persistence: config() stores survive restarts via ~/.config/omarchy/" + pluginID + ".json.",
		"  readonly property string omaHome: Quickshell.env(\"HOME\")",
		"  property bool omaBound: false",
		"  property var omaData: null",
		"",
		"  function __omaPersist(data) {",
		"    root.omaData = data",
		"    omaSaveTimer.restart()",
		"  }",
		"",
		"  function __omaLoad(raw) {",
		"    if (root.omaBound) return",
		"    var saved = {}",
		"    try { saved = JSON.parse(String(raw || \"{}\")) } catch (e) {}",
		// Bind BEFORE raising omaBound: property-change handlers run
		// synchronously, so flipping the flag first would let consumers read
		// half-seeded stores.
		"    Logic.__omaBindRef(saved, root.__omaPersist)",
		"    root.omaBound = true",
		"  }",
		"",
		// Held as properties, not child objects: QtObject has no default
		// property, so nested FileView{} would fail to instantiate.
		"  property FileView omaSettingsFile: FileView {",
		"    path: root.omaHome + \"/.config/omarchy/" + pluginID + ".json\"",
		"    watchChanges: false",
		"    atomicWrites: true",
		"    printErrors: false",
		"    onLoaded: root.__omaLoad(text())",
		"    onLoadFailed: root.__omaLoad(\"\")",
		"  }",
		"",
		"  // Debounced like other omarchy plugins; a slider dragging across set()",
		"  // calls would otherwise hit the disk on every tick.",
		"  property Timer omaSaveTimer: Timer {",
		"    interval: 200",
		"    onTriggered: omaSettingsFile.setText(JSON.stringify(root.omaData, null, 2) + \"\\n\")",
		"  }",
	}, "\n")

	return strings.Join([]string{
		"import QtQuick",
		"import Quickshell",
		"import Quickshell.Io",
		fmt.Sprintf("import \"%s\" as Logic", jsModule),
		"",
		"QtObject {",
		"  id: root",
		"",
		body,
		"",
		persistence,
		"",
		"  Component.onCompleted: {",
		strings.Join(sync, "\n"),
		"  }",
		"",
		"  Component.onDestruction: {",
		"    for (var i = 0; i < unsubscribers.length; i++) unsubscribers[i]()",
		"  }",
		"}",
	}, "\n")
}
