import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Floating-window settings panel. Summon with:
//   omarchy-shell shell toggle <id>
Item {
	id: root
	property var shell: null
	readonly property string pluginId: "__ID__"

	__BRIDGE__ {
		id: logic
	}

	// IPC pass-throughs
	function setEnabled(v) { logic.setEnabled(v) }
	function setVolume(v) { logic.setVolume(v) }
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
		implicitWidth: 340
		implicitHeight: 260
		minimumSize: Qt.size(300, 220)

		Column {
			anchors.fill: parent
			anchors.margins: Style.spacing.panelPadding
			spacing: Style.spacing.panelGap

			Text {
				text: "Settings"
				color: Color.foreground
				font.family: Style.font.family
				font.pixelSize: Style.font.heading
				font.bold: true
			}

			PanelSeparator {}

			Row {
				width: parent.width
				spacing: Style.spacing.controlGap

				Text {
					anchors.verticalCenter: parent.verticalCenter
					text: "Enabled"
					color: Color.foreground
					font.family: Style.font.family
					font.pixelSize: Style.font.body
				}

				ToggleSwitch {
					anchors.right: parent.right
					checked: logic.enabled
					onToggled: root.setEnabled(!checked)
				}
			}

			NumberField {
				width: parent.width
				label: "Volume"
				value: logic.volume
				from: 0
				to: 100
				onModified: root.setVolume(value)
			}

			Item {
				width: parent.width
				height: Style.spacing.sm
			}

			Text {
				width: parent.width
				wrapMode: Text.WordWrap
				text: "Changes persist to ~/.config/omarchy/" + root.pluginId + ".json"
				color: Color.muted
				font.family: Style.font.family
				font.pixelSize: Style.font.caption
			}
		}
	}
}
