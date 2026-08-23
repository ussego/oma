import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Floating-window todo panel. Summon with:
//   omarchy-shell shell toggle <id>
Item {
	id: root
	property var shell: null
	readonly property string pluginId: "__ID__"
	readonly property color contentForeground: Color.foreground

	__BRIDGE__ {
		id: logic
	}

	// IPC pass-throughs: scripts drive the app via
	//   omarchy-shell shell call <id> <method> [arg]
	// config() buffers mutations until the settings file has been read, so an
	// early call can never clobber saved todos.
	function addTodo(text) { logic.addTodo(text) }
	function toggleDone(id) { logic.toggleDone(String(id)) }
	function removeTodo(id) { logic.removeTodo(String(id)) }
	function clearDone() { logic.clearDone() }
	function snapshot() { return logic.snapshot() }

	function open(payloadJson) {
		window.visible = true
		input.forceActiveFocus()
	}

	// Guard: the shell's hide() also invokes close(); without this the two
	// calls recurse into a stack overflow.
	function close() {
		if (!window.visible) return
		window.visible = false
		if (typeof shell !== "undefined" && shell && typeof shell.hide === "function")
			shell.hide(root.pluginId)
	}

	// Guard: bridge fields are undefined until the bridge's first push on
	// load, so list reads must not assume items exists yet.
	readonly property int pendingCount: (logic.items || []).filter(function (t) { return !t.done }).length
	readonly property int doneCount: (logic.items || []).length - pendingCount

	Shortcut {
		sequence: "Esc"
		context: Qt.WindowShortcut
		onActivated: root.close()
	}

	FloatingWindow {
		id: window
		visible: false
		color: Color.background
		implicitWidth: 380
		implicitHeight: 480
		minimumSize: Qt.size(320, 300)

		onVisibleChanged: {
			if (!visible && root.shell && typeof root.shell.hide === "function")
				root.shell.hide(root.pluginId)
		}

		Rectangle {
			anchors.fill: parent
			color: Color.background

			// ---- Header + input -------------------------------------------
			Column {
				id: header
				anchors.top: parent.top
				anchors.left: parent.left
				anchors.right: parent.right
				anchors.margins: Style.spacing.panelPadding
				spacing: Style.spacing.panelGap

				Item {
					width: parent.width
					height: title.implicitHeight

					Text {
						id: title
						text: "Todos"
						color: root.contentForeground
						font.family: Style.font.family
						font.pixelSize: Style.font.heading
						font.bold: true
					}

					Text {
						anchors.right: parent.right
						anchors.baseline: title.baseline
						text: root.pendingCount + " open · " + root.doneCount + " done"
						color: Color.muted
						font.family: Style.font.family
						font.pixelSize: Style.font.caption
						visible: (logic.items || []).length > 0
					}
				}

				PanelSeparator {}

				Row {
					width: parent.width
					spacing: Style.spacing.controlGap

					TextField {
						id: input
						width: parent.width - addButton.width - parent.spacing
						placeholderText: "What needs doing?"
						selectByMouse: true
						onAccepted: root.addFromInput()
					}

					Button {
						id: addButton
						iconText: "+"
						tooltipText: "Add todo"
						bordered: true
						focusable: true
						onClicked: root.addFromInput()
					}
				}
			}

			// ---- Todo list -------------------------------------------------
			ListView {
				id: list
				anchors.top: header.bottom
				anchors.bottom: footer.top
				anchors.left: parent.left
				anchors.right: parent.right
				anchors.topMargin: Style.spacing.lg
				anchors.bottomMargin: Style.spacing.sm
				clip: true
				model: logic.items || []
				spacing: 0

				delegate: Item {
					id: del
					width: list.width
					implicitHeight: Style.space(34)
					required property var modelData

					ToggleSwitch {
						id: check
						anchors.left: parent.left
						anchors.leftMargin: Style.spacing.panelPadding
						anchors.verticalCenter: parent.verticalCenter
						checked: del.modelData.done
						onToggled: root.toggleDone(del.modelData.id)
					}

					Text {
						anchors.left: check.right
						anchors.right: deleteButton.left
						anchors.leftMargin: Style.spacing.controlGap
						anchors.rightMargin: Style.spacing.controlGap
						anchors.verticalCenter: parent.verticalCenter
						text: del.modelData.text
						elide: Text.ElideRight
						color: del.modelData.done ? Color.muted : root.contentForeground
						font.family: Style.font.family
						font.pixelSize: Style.font.body
						font.strikeout: del.modelData.done
					}

					PanelActionButton {
						id: deleteButton
						anchors.right: parent.right
						anchors.rightMargin: Style.spacing.rowPaddingX
						anchors.verticalCenter: parent.verticalCenter
						iconText: "✕"
						tooltipText: "Delete"
						foreground: Color.muted
						hoverColor: Color.urgent
						onClicked: root.removeTodo(del.modelData.id)
					}
				}

				// Empty state, centered in the list area.
				Text {
					anchors.centerIn: parent
					text: "Nothing to do — add one above"
					visible: list.count === 0
					color: Color.muted
					font.family: Style.font.family
					font.pixelSize: Style.font.body
				}
			}

			// ---- Footer ------------------------------------------------------
			Column {
				id: footer
				visible: root.doneCount > 0
				anchors.bottom: parent.bottom
				anchors.left: parent.left
				anchors.right: parent.right
				anchors.margins: Style.spacing.panelPadding
				spacing: Style.spacing.panelGap

				PanelSeparator {}

				Item {
					width: parent.width
					height: clearButton.visible ? clearButton.implicitHeight : 0

					Button {
						id: clearButton
						anchors.right: parent.right
						text: "Clear done (" + root.doneCount + ")"
						leftAlign: false
						onClicked: root.clearDone()
					}
				}
			}
		}
	}

	function addFromInput() {
		logic.addTodo(input.text)
		input.text = ""
		input.forceActiveFocus()
	}
}
