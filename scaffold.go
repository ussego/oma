package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Author        string            `json:"author,omitempty"`
	Description   string            `json:"description"`
	Kinds         []string          `json:"kinds"`
	EntryPoints   map[string]string `json:"entryPoints"`
	Framework     string            `json:"framework,omitempty"`
}

// Entry point keys are camelCase per Omarchy's plugin schema.
var entryPointKey = map[string]string{
	"bar":        "bar",
	"bar-widget": "barWidget",
	"menu":       "menu",
	"panel":      "panel",
	"overlay":    "overlay",
	"service":    "service",
}

var qmlName = map[string]string{
	"bar":        "Bar.qml",
	"bar-widget": "BarWidget.qml",
	"menu":       "Menu.qml",
	"panel":      "Panel.qml",
	"overlay":    "Overlay.qml",
	"service":    "Service.qml",
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

// qmlSafeBridge returns a valid QML component identifier for a bridge name
// that may contain hyphens (e.g. "oma-smoketest-window" → "OmaSmoketestWindow").
func qmlSafeBridge(bridge string) string {
	var out []rune
	capNext := true
	for _, r := range bridge {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if capNext {
				out = append(out, unicode.ToUpper(r))
				capNext = false
			} else {
				out = append(out, r)
			}
		} else {
			capNext = true
		}
	}
	if len(out) == 0 {
		return "Bridge"
	}
	// ensure starts with letter
	if out[0] >= '0' && out[0] <= '9' {
		out = append([]rune{'B'}, out...)
	}
	return string(out)
}

type scaffoldOptions struct {
	Description string
	Version     string
	Author      string
	PanelMode   string // attached | window | both
}

func scaffold(name string, kinds []string) error {
	_, err := scaffoldWithOptions(name, kinds, scaffoldOptions{})
	return err
}

// scaffoldWithOptions writes the project and returns the created file paths
// (relative to the project dir), so callers can print a vite-style log.
func scaffoldWithOptions(name string, kinds []string, opts scaffoldOptions) ([]string, error) {
	var created []string
	add := func(rel string, err error) error {
		if err != nil {
			return err
		}
		created = append(created, rel)
		return nil
	}

	dir := filepath.Clean(name)
	project := filepath.Base(dir)
	for _, sub := range []string{"src", "ui"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}

	mode := opts.PanelMode
	if mode == "" {
		mode = "attached"
	}
	// validate panel mode; fall back to attached on unknown
	switch mode {
	case "attached", "window", "both":
	default:
		mode = "attached"
	}
	hasPanel := contains(kinds, "panel")

	var pluginKinds []string
	for _, k := range kinds {
		if _, ok := entryPointKey[k]; ok {
			pluginKinds = append(pluginKinds, k)
		}
	}
	pluginKinds = manifestKindsFor(pluginKinds, mode)

	entryPoints := map[string]string{}
	for _, k := range pluginKinds {
		entryPoints[entryPointKey[k]] = "ui/" + qmlName[k]
	}
	// both mode needs a second panel entry: ui/PanelWindow.qml
	if hasPanel && mode == "both" {
		entryPoints["panel"] = "ui/PanelWindow.qml"
	}

	desc := opts.Description
	if desc == "" {
		desc = "A custom plugin for Omarchy - built with Oma"
	}
	ver := opts.Version
	if ver == "" {
		ver = "0.1.0"
	}

	authorPart := sanitize(opts.Author)
	if authorPart == "" {
		authorPart = userNamespace()
	}
	m := manifest{
		SchemaVersion: 1,
		ID:            authorPart + "." + sanitize(project),
		Name:          project,
		Version:       ver,
		Description:   desc,
		Kinds:         pluginKinds,
		EntryPoints:   entryPoints,
		Framework:     "oma",
	}
	if opts.Author != "" {
		m.Author = opts.Author
	}
	if err := add("manifest.json", writeJSON(filepath.Join(dir, "manifest.json"), m)); err != nil {
		return nil, err
	}

	if err := add("src/index.js", writeFile(filepath.Join(dir, "src", "index.js"), indexJS())); err != nil {
		return nil, err
	}
	if err := add("package.json", writeFile(filepath.Join(dir, "package.json"), packageJSONFor(project))); err != nil {
		return nil, err
	}
	if err := add("oma.json", writeFile(filepath.Join(dir, "oma.json"), omaJSONStubFor(hasPanel, mode))); err != nil {
		return nil, err
	}
	if err := add(".gitignore", writeFile(filepath.Join(dir, ".gitignore"), gitignore())); err != nil {
		return nil, err
	}
	bridge := bridgeBaseName(capitalize(project), pluginKinds)

	// bar-widget glyph comes from the global oma.json icon, else a cog
	icon := "\uf013"
	if cfg, err := loadOMAConfig(dir); err == nil && cfg.Icon != "" {
		icon = cfg.Icon
	}

	for _, k := range kinds {
		var files []genFile
		if k == "panel" {
			files = panelSurfaceFiles(mode, m.ID, bridge, icon)
		} else {
			files = surfaceFiles(k, m.ID, bridge, icon)
		}
		for _, f := range files {
			if err := add(f.rel, writeFile(filepath.Join(dir, f.rel), f.content)); err != nil {
				return nil, err
			}
		}
	}
	return created, nil
}

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// userNamespace namespaces new plugins under the OS user (<user>.<name>);
// the omarchy.* namespace is reserved and rejected by omarchy plugin validate.
func userNamespace() string {
	if u := os.Getenv("USER"); u != "" {
		return sanitize(u)
	}
	return "user"
}

func omaJSONStubFor(hasPanel bool, mode string) string {
	schema, err := ensureSchema()
	if err != nil {
		schema = ""
	}
	stub := map[string]any{}
	if schema != "" {
		stub["$schema"] = schema
	}
	if hasPanel && mode != "" && mode != "attached" {
		stub["panel"] = map[string]any{"mode": mode}
	}
	data, err := json.MarshalIndent(stub, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(data) + "\n"
}

// indexJS is the starter plugin source. The `@oma/runtime` specifier resolves
// to the runtime bundled inside the oma binary (esbuild Alias), so no import
// map or config file is needed.
func indexJS() string {
	return `import { config, state } from "@oma/runtime";

// Reactive state - every field becomes a property on the generated bridge,
// so QML can bind and react to it directly.
export const music = state({
  playing: false,
  song: "",
  volume: 100,
});

export function toggle() {
  music.playing = !music.playing;
}

// Config survives restarts automatically (~/.config/omarchy/<id>.json).
export const settings = config({ openOnStartup: false });

export function toggleStartup() {
  settings.set("openOnStartup", !settings.get("openOnStartup"));
}
`
}

// packageJSONFor emits the minimal manifest for a JS plugin project: ESM,
// private (never published), name only so editors and package managers are
// happy. The runtime lib itself is optional — installing @jsr/oma__runtime
// gives editors types for the "@oma/runtime" specifier.
func packageJSONFor(project string) string {
	data, _ := json.Marshal(map[string]any{
		"name":    strings.ToLower(project),
		"private": true,
		"type":    "module",
	})
	return string(data) + "\n"
}

type genFile struct {
	rel     string
	content string
}

// surfaceFiles returns the QML files a kind scaffolds. "panel" emits the
// bar-attached pair (button widget + anchored popup).
func surfaceFiles(kind, id, bridge, icon string) []genFile {
	switch kind {
	case "panel":
		return []genFile{
			{rel: "ui/" + qmlName["bar-widget"], content: barPanelHostSkeleton(id)},
			{rel: "ui/" + qmlName["panel"], content: attachedPanelSkeleton(id)},
		}
	case "bar-widget":
		return []genFile{{rel: "ui/" + qmlName[kind], content: barWidgetSkeleton(id, bridge, icon)}}
	case "overlay", "menu":
		return []genFile{{rel: "ui/" + qmlName[kind], content: overlaySkeleton(kind, id)}}
	default:
		return []genFile{{rel: "ui/" + qmlName[kind], content: stubSkeleton(kind, bridge)}}
	}
}

// panelSurfaceFiles returns the QML files for a panel based on mode.
// attached: bar-widget host + attached popup (default)
// window:   standalone FloatingWindow panel (no bar widget)
// both:     bar-widget host + attached popup + standalone FloatingWindow (PanelWindow.qml)
func panelSurfaceFiles(mode, id, bridge, icon string) []genFile {
	switch mode {
	case "window":
		return []genFile{
			{rel: "ui/" + qmlName["panel"], content: floatingPanelSkeleton(id, bridge)},
		}
	case "both":
		return []genFile{
			{rel: "ui/" + qmlName["bar-widget"], content: barPanelHostSkeletonWithIcon(id, icon)},
			{rel: "ui/" + qmlName["panel"], content: attachedPanelSkeleton(id)},
			{rel: "ui/PanelWindow.qml", content: floatingPanelSkeleton(id, bridge)},
		}
	default: // attached
		return []genFile{
			{rel: "ui/" + qmlName["bar-widget"], content: barPanelHostSkeletonWithIcon(id, icon)},
			{rel: "ui/" + qmlName["panel"], content: attachedPanelSkeleton(id)},
		}
	}
}

// manifestKindsFor translates scaffold kinds into manifest kinds: a "panel"
// ships as a bar-widget button plus its popup, so the manifest declares
// bar-widget. In window mode it declares panel (standalone FloatingWindow),
// in both it declares both.
func manifestKindsFor(kinds []string, mode ...string) []string {
	m := ""
	if len(mode) > 0 {
		m = mode[0]
	}
	if m == "" {
		m = "attached"
	}
	var out []string
	for _, k := range kinds {
		if k == "panel" {
			switch m {
			case "window":
				if !contains(out, "panel") {
					out = append(out, "panel")
				}
			case "both":
				if !contains(out, "bar-widget") {
					out = append(out, "bar-widget")
				}
				if !contains(out, "panel") {
					out = append(out, "panel")
				}
			default: // attached
				if !contains(out, "bar-widget") {
					out = append(out, "bar-widget")
				}
			}
			continue
		}
		if !contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// barWidgetSkeleton is a standalone bar-widget button wired to the shared core.
func barWidgetSkeleton(id, bridge, icon string) string {
	return fmt.Sprintf(`import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Bar widget surface. Run "oma build" once so %s (the shared core) exists.
BarWidget {
	id: root
	moduleName: %q

	// Generated bridge exposes the shared core.
	%s {
		id: logic
	}

	implicitWidth: button.implicitWidth
	implicitHeight: button.implicitHeight

	WidgetButton {
		id: button
		anchors.fill: parent
		bar: root.bar
		text: ""
		labelVisible: false
		hasVisualContent: true
		tooltipText: %q

		// WidgetButton has no clicked signal - use onPressed
		onPressed: function(button) {
			logic.toggle()
		}

		OpticalGlyph {
			anchors.centerIn: parent
			width: Style.bar.iconCanvas
			height: Style.bar.iconCanvas
			text: %q
			fontFamily: button.fontFamily
			fontSize: Style.bar.iconFont
			color: button.foreground
		}
	}
}
`, bridge, id, bridge, id, icon)
}

// barPanelHostSkeleton is the bar button that owns an attached Panel.qml.
func barPanelHostSkeleton(id string) string {
	return barPanelHostSkeletonWithIcon(id, "\uf013")
}

func barPanelHostSkeletonWithIcon(id, icon string) string {
	return fmt.Sprintf(`import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Bar button that opens/closes the attached panel (see Panel.qml).
BarWidget {
	id: root
	moduleName: %q

	function injectPanel() {
		var t = panelLoader.item
		if (!t) return
		if ("bar" in t) t.bar = root.bar
		if ("settings" in t) t.settings = root.settings
		if ("anchorItem" in t) t.anchorItem = button
		if ("hostWidget" in t) t.hostWidget = root
	}

	// Exposed for the shell's panel routing (summon/toggle/hide by id).
	readonly property bool opened: panelLoader.item ? panelLoader.item.opened === true : false

	function open() {
		root.injectPanel()
		if (panelLoader.item && panelLoader.item.open) panelLoader.item.open()
	}

	function close() {
		if (panelLoader.item && panelLoader.item.close) panelLoader.item.close()
	}

	function toggle() { root.opened ? root.close() : root.open() }

	function togglePanel() { root.toggle() }

	onBarChanged: root.injectPanel()

	implicitWidth: button.implicitWidth
	implicitHeight: button.implicitHeight

	Loader {
		id: panelLoader
		active: true
		source: Qt.resolvedUrl("Panel.qml")
		visible: false
		onLoaded: root.injectPanel()
	}

	WidgetButton {
		id: button
		anchors.fill: parent
		bar: root.bar
		text: ""
		labelVisible: false
		hasVisualContent: true
		tooltipText: %q

		// WidgetButton has no clicked signal - use onPressed
		onPressed: function(button) {
			if (!root.bar) return
			root.togglePanel()
		}

		OpticalGlyph {
			anchors.centerIn: parent
			width: Style.bar.iconCanvas
			height: Style.bar.iconCanvas
			text: %q
			fontFamily: button.fontFamily
			fontSize: Style.bar.iconFont
			color: button.foreground
		}
	}
}
`, id, "Toggle "+id, icon)
}

// attachedPanelSkeleton is a qs.Ui.Panel popup anchored under the bar widget
// that owns it (injected via anchorItem).
func attachedPanelSkeleton(id string) string {
	return fmt.Sprintf(`import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Popup panel tied to the bar widget that loads it (see BarWidget.qml).
Panel {
	id: root
	moduleName: %q
	ipcTarget: %q

	// Injected by the host bar widget.
	property var anchorItem: null
	property var hostWidget: null
	readonly property var barIdentity: hostWidget || root
	readonly property color contentForeground: bar ? bar.foreground : Color.foreground
	readonly property string contentFontFamily: bar ? bar.fontFamily : Style.font.family

	function open() {
		root.controller.show()
		Qt.callLater(function() {
			if (keyCatcher) keyCatcher.forceActiveFocus()
		})
	}

	function close() { root.controller.hide() }

	function dismiss() { root.close() }

	KeyboardPanel {
		id: panel
		anchorItem: root.anchorItem
		owner: root.barIdentity
		bar: root.bar
		open: root.opened
		focusTarget: keyCatcher
		contentWidth: panel.fittedContentWidth(Style.space(360))
		contentHeight: panel.fittedContentHeight(content.implicitHeight)

		PanelKeyCatcher {
			id: keyCatcher
			anchors.fill: parent
			onCloseRequested: root.close()

			Column {
				id: content
				width: parent.width
				spacing: Style.space(12)

				Text {
					width: parent.width
					text: %q
					color: Qt.darker(root.contentForeground, 1.4)
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.caption
					font.bold: true
					horizontalAlignment: Text.AlignHCenter
				}

				Text {
					width: parent.width
					text: "Panel content goes here."
					color: root.contentForeground
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.body
					horizontalAlignment: Text.AlignHCenter
				}
			}
		}
	}
}
`, id, id, id)
}

// floatingPanelSkeleton is a standalone draggable window panel (native-app style).
// Root is an Item so the shell can inject shell/manifest and route summon→open.
func floatingPanelSkeleton(id, bridge string) string {
	safe := qmlSafeBridge(bridge)
	return fmt.Sprintf(`import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Standalone draggable panel window. Run "oma build" once so %s (the shared core) exists.
// Summon with: omarchy-shell shell summon %s '{}'
Item {
	id: root
	property var shell: null
	readonly property string pluginId: %q
	readonly property color contentForeground: Color.foreground
	readonly property string contentFontFamily: Style.font.family

	// Generated bridge exposes the shared core.
	%s {
		id: logic
	}

	function open(payloadJson) {
		window.visible = true
	}

	// Guard: the shell's hide() also invokes close(); without this the two
	// calls recurse into a stack overflow.
	function close() {
		if (!window.visible) return
		window.visible = false
		if (typeof shell !== "undefined" && shell && typeof shell.hide === "function")
			shell.hide(root.pluginId)
	}

	FloatingWindow {
		id: window
		visible: false
		color: Color.background
		implicitWidth: 380
		implicitHeight: 420
		minimumSize: Qt.size(320, 280)

		onVisibleChanged: {
			if (!visible && root.shell && typeof root.shell.hide === "function")
				root.shell.hide(root.pluginId)
		}

		Rectangle {
			anchors.fill: parent
			color: Color.background

			Column {
				anchors.centerIn: parent
				width: parent.width - Style.space(24)
				spacing: Style.space(12)

				OpticalGlyph {
					anchors.horizontalCenter: parent.horizontalCenter
					text: "\uf013"
					fontSize: 32
					color: root.contentForeground
				}

				Text {
					width: parent.width
					text: %q
					color: root.contentForeground
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.title
					font.bold: true
					horizontalAlignment: Text.AlignHCenter
				}

				Text {
					width: parent.width
					text: "Floating panel content goes here. Shared logic is available via the bridge."
					color: Qt.darker(root.contentForeground, 1.1)
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.body
					wrapMode: Text.Wrap
					horizontalAlignment: Text.AlignHCenter
				}

				Button {
					anchors.horizontalCenter: parent.horizontalCenter
					text: "Close"
					onClicked: root.close()
				}
			}
		}
	}
}
`, bridge, id, id, safe, id)
}

// overlaySkeleton is the fullscreen-scrim surface used by overlays/menus.
func overlaySkeleton(kind, id string) string {
	return fmt.Sprintf(`import QtQuick
import Quickshell
import Quickshell.Wayland
import qs.Commons
import qs.Ui

// Overlay/menu surface. The shell injects "shell" and "manifest" (and
// "settings" for widgets). Run "oma build" once so the shared core exists.
//
// Contract (see omarchy-shell): summon calls open(), hide calls close().
// Dismissing from inside must call shell.hide(pluginId) so the shell's
// open-state stays in sync.
Item {
	id: root

	// Plugin identity - single source for layer-shell namespacing and shell IPC.
	readonly property string pluginId: "%s"

	property bool opened: false

	function open(payloadJson) {
		root.opened = true
	}

	// Guard: shell.hide() invokes close() back - stop the ping-pong here.
	function close() {
		if (!root.opened) return
		root.opened = false
		if (typeof shell !== "undefined")
			shell.hide(root.pluginId)
	}

	PanelWindow {
		id: panel
		visible: root.opened
		anchors { top: true; bottom: true; left: true; right: true }
		color: "transparent"
		WlrLayershell.namespace: root.pluginId
		WlrLayershell.layer: WlrLayer.Overlay
		WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
		exclusionMode: ExclusionMode.Ignore

		// click-away scrim
		Rectangle {
			anchors.fill: parent
			color: Color.menu.scrim

			MouseArea {
				anchors.fill: parent
				onClicked: root.close()
			}
		}

		Rectangle {
			anchors.centerIn: parent
			width: Math.min(640, panel.width - Style.space(32))
			height: Math.min(420, panel.height - Style.space(32))
			radius: Style.cornerRadius
			color: Color.menu.background
			border.width: 1
			border.color: Color.menu.border

			Item {
				id: keyCatcher
				anchors.fill: parent
				focus: true
				Keys.onPressed: function(event) {
					if (event.key === Qt.Key_Escape)
						root.close()
				}

				Text {
					anchors.centerIn: parent
					text: root.pluginId
					color: Color.menu.text
					font.pixelSize: Style.font.heading
				}
			}
		}
	}
}
`, id)
}

// stubSkeleton is the fallback for service/bar/other kinds.
func stubSkeleton(kind, bridge string) string {
	return fmt.Sprintf(`import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// %s surface. The shell injects "shell" and "manifest" (and "settings" for
// widgets). Run "oma build" once so %s (the shared core) exists.
Item {
	id: root
}
`, kind, bridge)
}

// launcherWriterMarker marks files generated for launcher entries.
const launcherWriterMarker = "// Generated by oma build - launcher writer"

// launcherServiceSkeleton returns a Service.qml that delegates to LauncherWriter.
// It is a one-liner stub; the actual writer is LauncherWriter.qml (single source of truth).
// Devs can add more to this service - oma will preserve extra content and only
// ensures LauncherWriter {} is present.
func launcherServiceSkeleton(id string) string {
	return fmt.Sprintf(`import QtQuick

%s
// Service that ensures launcher entries for %s exist on fresh installs.
// Delegates to LauncherWriter - you can add more items to this service; oma will preserve them.
Item {
	id: root
	property var shell
	property var manifest
	LauncherWriter {}
}
`, launcherWriterMarker+" - service stub", id)
}

// launcherWriterSkeleton returns a helper LauncherWriter.qml for custom Service.qml.
// Custom Service.qml should instantiate LauncherWriter {} to get auto-create.
func launcherWriterSkeleton(id string, entries []struct{ Filename, Content string }) string {
	var procs strings.Builder
	for i, e := range entries {
		esc := strings.ReplaceAll(e.Content, "'", "'\\''")
		cmd := fmt.Sprintf("mkdir -p $HOME/.local/share/applications && printf '%%s' '%s' > $HOME/.local/share/applications/%s", esc, e.Filename)
		qmlCmd := strings.ReplaceAll(cmd, "\"", "\\\"")
		fmt.Fprintf(&procs, "  Process {\n    command: [\"bash\", \"-c\", \"%s\"]\n    running: true\n  }\n", qmlCmd)
		if i < len(entries)-1 {
			procs.WriteString("\n")
		}
	}
	return fmt.Sprintf(`import QtQuick
import Quickshell
import Quickshell.Io

%s
// Helper for custom Service.qml - add LauncherWriter {} to your Service.qml root.
// Writes launcher entries for %s; runs once on load.
Item {
	id: root
	Component.onCompleted: console.log("oma: LauncherWriter ensuring entries for %s")

%s
}
`, launcherWriterMarker, id, id, procs.String())
}

func gitignore() string {
	return `# oma: pkg/ is local validation (manifest.json + ui/ via oma package)
# built ui/index.mjs + ui/<Name>.qml must be committed — omarchy plugin add
# clones HEAD and runs omarchy plugin validate without building
pkg/
`
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, string(data)+"\n")
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
