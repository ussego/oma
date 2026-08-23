package main

import (
	"strings"
	"testing"
)

func scan(t *testing.T, src string) (ModuleMeta, error) {
	t.Helper()
	meta, _, err := scanBridge(src)
	return meta, err
}

func TestScanStatesAndFunctions(t *testing.T) {
	meta, err := scan(t, `import { state } from "@oma/runtime";

export const music = state({
  playing: false,
  song: "",
  volume: 100,
});

export function toggle() {
  music.playing = !music.playing;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 1 || meta.States[0].Name != "music" {
		t.Fatalf("states = %+v", meta.States)
	}
	want := []FieldMeta{{"playing", "bool", "false"}, {"song", "string", `""`}, {"volume", "double", "100"}}
	f := meta.States[0].Fields
	if len(f) != 3 {
		t.Fatalf("fields = %+v", f)
	}
	for i := range want {
		if f[i] != want[i] {
			t.Fatalf("field %d = %+v, want %+v", i, f[i], want[i])
		}
	}
	if len(meta.Functions) != 1 || meta.Functions[0] != "toggle" {
		t.Fatalf("functions = %+v", meta.Functions)
	}
}

func TestScanNestedAndDerivedSkipped(t *testing.T) {
	meta, err := scan(t, `
export const s = state({ a: true, nested: { x: 1 }, list: [1, 2] });
export const d = derived(() => s.a ? 1 : 0);
const hidden = state({ nope: false });
export function visible() {}
export function Hidden() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 1 {
		t.Fatalf("states = %+v", meta.States)
	}
	types := map[string]string{}
	for _, f := range meta.States[0].Fields {
		types[f.Name] = f.Type
	}
	if types["a"] != "bool" || types["nested"] != "var" || types["list"] != "var" {
		t.Fatalf("types = %v", types)
	}
	if len(meta.Functions) != 1 || meta.Functions[0] != "visible" {
		t.Fatalf("functions = %+v", meta.Functions)
	}
}

func TestScanArrowFunctions(t *testing.T) {
	meta, err := scan(t, `import { state } from "@oma/runtime";
export const play = () => {};
export const stop = async () => {};
export const add = (a, b) => a + b;
export const named = async function (x) {};
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"play", "stop", "add", "named"}
	if strings.Join(meta.Functions, ",") != strings.Join(want, ",") {
		t.Fatalf("functions = %v", meta.Functions)
	}
}

func TestScanErrorsOnBadState(t *testing.T) {
	cases := []struct{ label, body, want string }{
		{"arr", `import { state } from "@oma/runtime"; export const arr = state([1, 2]);`, "arr"},
		{"ident", `import { state } from "@oma/runtime"; export const ident = state(someVar);`, "ident"},
		{"spread", `import { state } from "@oma/runtime"; const base = {}; export const spread = state({...base});`, "spread"},
		{"computed", `import { state } from "@oma/runtime"; export const computed = state({ ["k" + 1]: true });`, "computed"},
		{"Upper_field", `import { state } from "@oma/runtime"; export const Upper = state({ Field: true });`, "Field"},
		{"dup", `import { state } from "@oma/runtime"; export const dup = state({ a: 1, a: 2 });`, "duplicate"},
		{"empty", `import { state } from "@oma/runtime"; export const empty = state({});`, "no fields"},
	}
	for _, c := range cases {
		if _, err := scan(t, c.body); err == nil {
			t.Fatalf("%s: expected error", c.label)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: error %q missing %q", c.label, err.Error(), c.want)
		}
	}
}

func TestScanIgnoresStringsAndComments(t *testing.T) {
	meta, err := scan(t, `
// export const commented = state({ no: false });
/* export const blocked = state({ no: false }); */
const greeting = "export const instr = state({ no: false });";
export const real = state({ on: true }); // trailing
export function act() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 1 || meta.States[0].Name != "real" {
		t.Fatalf("states = %+v", meta.States)
	}
	if len(meta.Functions) != 1 || meta.Functions[0] != "act" {
		t.Fatalf("functions = %+v", meta.Functions)
	}
}

// Comments INSIDE a state({...}) literal must scan cleanly: the comment
// markers themselves used to survive stripping, so parseFields saw a stray
// `/` as a field start and reported a bogus "shorthand fields" error.
func TestScanCommentsInsideStateLiteral(t *testing.T) {
	meta, err := scan(t, `export const todo = state({
  // leading comment
  items: [], // trailing comment on the field
  /* block comment */ done: false,
});
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 1 || meta.States[0].Name != "todo" {
		t.Fatalf("states = %+v", meta.States)
	}
	fields := meta.States[0].Fields
	if len(fields) != 2 || fields[0].Name != "items" || fields[1].Name != "done" {
		t.Fatalf("fields = %+v", fields)
	}
	if fields[0].Type != "var" || fields[1].Type != "bool" {
		t.Fatalf("types = %+v", fields)
	}
}

// stripAndMask must blank line-comment markers too (the body was blanked,
// the `//` itself used to survive and was scanned as code).
func TestStripBlanksCommentMarkers(t *testing.T) {
	src := "const a = 1; // hi\nexport const b = 2; /* x */\n"
	out, _ := stripAndMask(src)
	if strings.Contains(out, "//") {
		t.Fatalf("line comment marker survived:\n%q", out)
	}
	if strings.Contains(out, "/*") || strings.Contains(out, "*/") {
		t.Fatalf("block comment marker survived:\n%q", out)
	}
}

// derived(fn, { bridge: "prop" }) surfaces the value as a read-only QML
// property; plain derived() keeps the skip note.
func TestScanDerivedBridgeOption(t *testing.T) {
	meta, notes, err := scanBridge(`export const count = derived(() => 1, { bridge: "openCount" });
export const plain = derived(() => 2);
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Deriveds) != 1 || meta.Deriveds[0].Name != "count" || meta.Deriveds[0].Prop != "openCount" {
		t.Fatalf("deriveds = %+v", meta.Deriveds)
	}
	if len(notes.ignored) != 1 || !strings.Contains(notes.ignored[0], "plain") {
		t.Fatalf("ignored notes = %+v", notes.ignored)
	}
	qml := renderBridge(meta, "index.mjs", "usse.x")
	for _, want := range []string{
		"property var openCount",
		"root.openCount = Logic.__omaSnap(Logic.count.value)",
		"unsubscribers.push(Logic.count.subscribe(applyD0))",
	} {
		if !strings.Contains(qml, want) {
			t.Errorf("missing %q in:\n%s", want, qml)
		}
	}
}

// A bridged derived colliding with a state field name would generate two QML
// declarations with one id - reject it at scan time.
func TestScanDerivedPropCollision(t *testing.T) {
	_, _, err := scanBridge(`export const s = state({ open: 0 });
export const d = derived(() => 1, { bridge: "open" });
`)
	if err == nil || !strings.Contains(err.Error(), `bridge property "open" is declared twice`) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

// Scan errors show the offending source line so a bogus message is
// debuggable at a glance.
func TestScanErrorShowsSourceLine(t *testing.T) {
	_, _, err := scanBridge("export const a = state({ x: 1 });\nexport const bad = state([1]);\n")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "export const bad = state([1]);") {
		t.Fatalf("error missing source line: %v", err)
	}
}

// config() exports flag usesConfig for build-time notes.
func TestScanConfigFlagsUsesConfig(t *testing.T) {
	_, notes, err := scanBridge(`export const settings = config({ volume: 80 });
export const ns = config("ui", { shown: true });
export const opts = config({ v: 1 }, { validate: () => 1, debounceMs: 500 });
`)
	if err != nil {
		t.Fatal(err)
	}
	if !notes.usesConfig {
		t.Fatal("usesConfig not set")
	}
}

func TestScanMultipleExportsOneLine(t *testing.T) {
	meta, err := scan(t, `import { state } from "@oma/runtime"; export const a = state({ x: 1 }); export const b = state({ y: "z" }); export function f() {}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 2 || meta.States[0].Name != "a" || meta.States[1].Fields[0].Type != "string" {
		t.Fatalf("states = %+v", meta.States)
	}
	if len(meta.Functions) != 1 {
		t.Fatalf("functions = %+v", meta.Functions)
	}
}

func TestRenderBridgeShape(t *testing.T) {
	meta := ModuleMeta{
		States:    []StateMeta{{Name: "music", Fields: []FieldMeta{{"playing", "bool", "false"}}}},
		Functions: []string{"toggle"},
	}
	qml := renderBridge(meta, "index.mjs", "usse.boe")
	for _, want := range []string{
		`import "index.mjs" as Logic`,
		"import Quickshell.Io",
		"property bool playing",
		"function toggle() { return Logic.toggle.apply(null, arguments) }",
		"root.playing = Logic.__omaSnap(Logic.music.playing)",
		"unsubscribers.push(Logic.music.subscribe(apply0))",
		"Component.onDestruction",
		"FileView {",
		".config/omarchy/usse.boe.json",
		"atomicWrites: true",
		"Logic.__omaBindRef(saved, root.__omaPersist)",
		"Logic.__omaUnbindRef(root.omaSink)",
		"property bool omaReady: false",
		"interval: Logic.__omaDebounceMsRef()",
		"onLoaded: root.__omaLoad(text())",
	} {
		if !strings.Contains(qml, want) {
			t.Fatalf("missing %q in:\n%s", want, qml)
		}
	}
}

func TestStripCommentsPreservesLengthAndCode(t *testing.T) {
	src := "const a = 1; // hi\n/* block\nspan */ const b = `t ${x /* inner */} v`;"
	out, _ := stripAndMask(src)
	if len(out) != len(src) {
		t.Fatalf("length changed: %d -> %d", len(src), len(out))
	}
	if !strings.Contains(out, "const a = 1;") || !strings.Contains(out, "${x") || !strings.Contains(out, "} v`;") {
		t.Fatalf("code damaged:\n%q", out)
	}
	if strings.Contains(out, "hi\n") || strings.Contains(out, "block") || strings.Contains(out, "inner") {
		t.Fatalf("comment content survived:\n%q", out)
	}
}

func TestScanTypeScript(t *testing.T) {
	meta, err := scan(t, `import { config, state } from "@oma/runtime";

interface Music {
	playing: boolean;
}

export type Alias = Music;

export enum Kind { A = 1 }

export const music = state<Music>({
	playing: false,
});

export const flags = state({ on: true as const, label: "" as string | undefined });

export const settings = config<{ volume: number }>({ volume: 80 });

export function toggle(): void {
	music.playing = !music.playing;
}

export const setVolume = (v: number): Promise<void> => {
	return Promise.resolve();
};

export async function* events(): AsyncGenerator<number> {
	yield 1;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 2 {
		t.Fatalf("states = %+v", meta.States)
	}
	if meta.States[0].Name != "music" || meta.States[0].Fields[0].Type != "bool" {
		t.Fatalf("music = %+v", meta.States[0])
	}
	types := map[string]string{}
	for _, f := range meta.States[1].Fields {
		types[f.Name] = f.Type
	}
	if types["on"] != "bool" || types["label"] != "string" {
		t.Fatalf("flags types = %v", types)
	}
	want := []string{"toggle", "setVolume", "events"}
	if strings.Join(meta.Functions, ",") != strings.Join(want, ",") {
		t.Fatalf("functions = %v", meta.Functions)
	}
}

// The bridge name must dodge surface-file collisions everywhere it is
// computed (build, surface add, scaffold, status) — one helper, one rule.
func TestBridgeBaseName(t *testing.T) {
	cases := []struct {
		name  string
		kinds []string
		want  string
	}{
		{"Music", []string{"panel"}, "Music"},
		{"Panel", []string{"panel"}, "PanelBridge"},
		{"Overlay", []string{"overlay", "bar-widget"}, "OverlayBridge"},
		{"Bar Widget", []string{"bar-widget"}, "BarWidgetBridge"}, // sanitizes onto the surface name
	}
	for _, c := range cases {
		if got := bridgeBaseName(c.name, c.kinds); got != c.want {
			t.Errorf("bridgeBaseName(%q, %v) = %q, want %q", c.name, c.kinds, got, c.want)
		}
	}
	// qmlSafeBridge never returns empty: a blank name falls back to "Bridge"
	if got := bridgeBaseName("", nil); got != "Bridge" {
		t.Errorf("blank name should fall back to Bridge, got %q", got)
	}
}

// Unbridged export forms surface as build notes, never as errors.
func TestScanExportFormsNotes(t *testing.T) {
	meta, notes, err := scanBridge(`
export default function helper() {}
export { music };
export * from "./extra.js";
export const music = state({ on: true });
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.States) != 1 || meta.States[0].Name != "music" {
		t.Fatalf("states = %+v", meta.States)
	}
	joined := strings.Join(notes.ignored, "\n")
	for _, want := range []string{"export default is not bridged", "export {music} form is not scanned", "export * is not scanned"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notes missing %q:\n%s", want, joined)
		}
	}
}

// `satisfies` clauses are tolerated both inside the literal and on the arg.
func TestScanSatisfiesClause(t *testing.T) {
	meta, err := scan(t, `export const flags = state({ on: true satisfies Flag, label: "x" satisfies Str });`)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, f := range meta.States[0].Fields {
		types[f.Name] = f.Type
	}
	if types["on"] != "bool" || types["label"] != "string" {
		t.Fatalf("types = %v", types)
	}
}

// `export let` and template-literal field values scan cleanly.
func TestScanLetAndTemplateFields(t *testing.T) {
	meta, err := scan(t, "export let n = state({ name: `hello`, a: .5, b: -3, c: 0x1F });")
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, f := range meta.States[0].Fields {
		types[f.Name] = f.Type
	}
	if types["name"] != "string" || types["a"] != "double" || types["b"] != "double" || types["c"] != "double" {
		t.Fatalf("types = %v", types)
	}
}

// The generated bridge wires every state and action, in order, and cleans up
// its subscriptions on destruction.
func TestRenderBridgeMultiState(t *testing.T) {
	meta := ModuleMeta{
		States: []StateMeta{
			{Name: "music", Fields: []FieldMeta{{"playing", "bool", "false"}, {"song", "string", `""`}}},
			{Name: "todos", Fields: []FieldMeta{{"open", "string", `""`}}},
		},
		Functions: []string{"toggle", "add"},
	}
	qml := renderBridge(meta, "index.mjs", "usse.x")
	for _, want := range []string{
		"property var unsubscribers: []",
		"property bool playing",
		"property string song",
		"property string open",
		"function toggle() { return Logic.toggle.apply(null, arguments) }",
		"function add() { return Logic.add.apply(null, arguments) }",
		"var apply0 = function()",
		"var apply1 = function()",
		"root.playing = Logic.__omaSnap(Logic.music.playing)",
		"root.song = Logic.__omaSnap(Logic.music.song)",
		"root.open = Logic.__omaSnap(Logic.todos.open)",
		"unsubscribers.push(Logic.music.subscribe(apply0))",
		"unsubscribers.push(Logic.todos.subscribe(apply1))",
		"for (var i = 0; i < unsubscribers.length; i++) unsubscribers[i]()",
	} {
		if !strings.Contains(qml, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// Scan errors carry the source line of the offending export.
func TestScanErrorLineNumber(t *testing.T) {
	src := "export const ok = state({ x: 1 });\n\nexport const bad = state([1]);\n"
	_, _, err := scanBridge(src)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error missing line number: %v", err)
	}
}
