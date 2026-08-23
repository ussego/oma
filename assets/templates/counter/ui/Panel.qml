import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Floating-window counter. Summon with:
//   omarchy-shell shell toggle <id>
Item {
	id: root
	property var shell: null
	readonly property string pluginId: "__ID__"

	__BRIDGE__ {
		id: logic
	}

	// IPC pass-throughs
	function inc() { logic.inc() }
	function dec() { logic.dec() }
	function reset() { logic.reset() }
	function snapshot() { return logic.snapshot() }

	function open(payloadJson) {
		window.visible = true
	}

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
		implicitWidth: 320
		implicitHeight: 220
		minimumSize: Qt.size(280, 180)

		Column {
			anchors.centerIn: parent
			spacing: Style.spacing.lg

			Text {
				anchors.horizontalCenter: parent.horizontalCenter
				text: logic.step
				color: Color.foreground
				font.family: Style.font.family
				font.pixelSize: Style.font.display
				font.bold: true
			}

			Row {
				anchors.horizontalCenter: parent.horizontalCenter
				spacing: Style.spacing.controlGap

				Button {
					text: "−"
					onClicked: root.dec()
				}

				Button {
					text: "+"
					bordered: true
					onClicked: root.inc()
				}

				Button {
					text: "reset"
					onClicked: root.reset()
				}
			}
		}
	}
}
