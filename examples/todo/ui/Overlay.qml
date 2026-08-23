import QtQuick
import Quickshell
import Quickshell.Wayland
import qs.Commons
import qs.Ui

// Overlay surface. The shell injects "shell" and "manifest".
// Run "oma build" once so the shared core (Todo) exists.
//
// Contract (see omarchy-shell): summon calls open(), hide calls close().
// Dismissing from inside must call shell.hide(pluginId) so the shell's
// open-state stays in sync.
Item {
	id: root

	// Plugin identity - single source for layer-shell namespacing and shell IPC.
	readonly property string pluginId: "examples.todo"

	property bool opened: false

	// Generated bridge - the shared JS core as a QtObject.
	Todo {
		id: logic
	}

	function open(payloadJson) {
		root.opened = true
		input.forceActiveFocus()
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
			height: Math.min(480, panel.height - Style.space(32))
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

				Column {
					anchors.fill: parent
					anchors.margins: Style.space(16)
					spacing: Style.space(10)

					Text {
						text: root.pluginId
						color: Color.menu.text
						font.pixelSize: Style.font.heading
						font.bold: true
					}

					TextField {
						id: input
						width: parent.width
						placeholderText: "Add a todo and press enter"
						onAccepted: {
							logic.add(text)
							text = ""
						}
					}

					ListView {
						id: list
						width: parent.width
						height: Math.max(0, parent.height - y - Style.space(8))
						clip: true
						spacing: Style.space(4)

						// one model per section, rebuilt when the bridge updates
						model: logic.open.split("\n").filter(function(t) { return t !== "" })

						delegate: Row {
							required property string modelData
							width: list.width
							spacing: Style.space(8)

							Text {
								width: parent.width - doneButton.width - parent.spacing
								text: parent.modelData
								elide: Text.ElideRight
								color: Color.menu.text
								font.pixelSize: Style.font.body
								anchors.verticalCenter: parent.verticalCenter
							}

							Button {
								id: doneButton
								text: "Done"
								onClicked: logic.complete(parent.modelData)
							}
						}
					}

					Text {
						visible: logic.done !== ""
						text: logic.done === "" ? "" : "Done:"
						color: Color.menu.text
						font.pixelSize: Style.font.caption
						font.bold: true
					}

					ListView {
						id: doneList
						width: parent.width
						height: visible ? contentHeight : 0
						interactive: false
						spacing: Style.space(4)

						model: logic.done.split("\n").filter(function(t) { return t !== "" })

						delegate: Row {
							required property string modelData
							width: doneList.width
							spacing: Style.space(8)

							Text {
								width: parent.width - reopenButton.width - clearButton.width - 2 * parent.spacing
								text: parent.modelData
								elide: Text.ElideRight
								color: Color.menu.text
								font.pixelSize: Style.font.body
								font.strikeout: true
								anchors.verticalCenter: parent.verticalCenter
							}

							Button {
								id: reopenButton
								text: "Reopen"
								onClicked: logic.reopen(parent.modelData)
							}

							Button {
								id: clearButton
								text: "Clear"
								onClicked: logic.clearDone()
							}
						}
					}
				}
			}
		}
	}
}
