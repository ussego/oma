import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Bar widget surface: live stopwatch readout, plus the QML Timer that drives
// the shared core (setInterval is unavailable in QJSEngine module scope, so
// the UI layer owns the loop and calls logic.tick()).
BarWidget {
	id: root
	moduleName: "examples.stopwatch"

	// Generated bridge - the shared JS core as a QtObject.
	Stopwatch {
		id: logic
	}

	// The loop follows state: both interval and running bind to bridged
	// properties, so changing tickMs or stopping from the panel takes effect
	// here immediately.
	Timer {
		interval: logic.tickMs
		running: logic.running
		repeat: true
		onTriggered: logic.tick()
	}

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
		tooltipText: logic.getTitle() + " - click to start/stop"

		// WidgetButton has no clicked signal - use onPressed
		onPressed: function(button) {
			if (!root.bar) return
			logic.toggle()
		}

		Text {
			anchors.centerIn: parent
			text: logic.display
			color: button.foreground
			font.family: button.fontFamily
			font.pixelSize: Style.bar.iconFont
			font.bold: true
		}
	}
}
