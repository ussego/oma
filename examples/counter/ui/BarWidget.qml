import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Bar button that opens/closes the attached panel (see Panel.qml) and shows
// the live count from the shared core.
BarWidget {
	id: root
	moduleName: "examples.counter"

	// Generated bridge - the shared JS core as a QtObject. Exists after
	// "oma build"; bindings below react to it like any QObject property.
	Counter {
		id: logic

		// Saved config seeds asynchronously once the settings file loads;
		// re-apply the persisted step when the bridge has bound.
		onOmaBoundChanged: if (omaBound) Qt.callLater(logic.applySavedStep)
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
		tooltipText: "Counter - click to open"

		// WidgetButton has no clicked signal - use onPressed
		onPressed: function(button) {
			if (!root.bar) return
			root.togglePanel()
		}

		Text {
			anchors.centerIn: parent
			text: logic.count
			color: button.foreground
			font.family: button.fontFamily
			font.pixelSize: Style.bar.iconFont
			font.bold: true
		}
	}
}
