import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Popup panel tied to the bar widget that loads it (see BarWidget.qml).
Panel {
	id: root
	moduleName: "examples.stopwatch"
	ipcTarget: "examples.stopwatch"

	// Injected by the host bar widget.
	property var anchorItem: null
	property var hostWidget: null
	readonly property var barIdentity: hostWidget || root
	readonly property color contentForeground: bar ? bar.foreground : Color.foreground
	readonly property string contentFontFamily: bar ? bar.fontFamily : Style.font.family

	// Generated bridge - the shared JS core as a QtObject. Surfaces each
	// instantiate it; the JS module behind it is shared, so both stay in sync.
	Stopwatch {
		id: logic
	}

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
					text: logic.display
					color: Qt.darker(root.contentForeground, 1.4)
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.display
					font.bold: true
					horizontalAlignment: Text.AlignHCenter
				}

				Text {
					width: parent.width
					text: logic.lastEvent
					color: root.contentForeground
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.caption
					horizontalAlignment: Text.AlignHCenter
				}

				Row {
					anchors.horizontalCenter: parent.horizontalCenter
					spacing: Style.space(8)

					Button { text: "Start/Stop"; onClicked: logic.toggle() }
					Button { text: "Reset"; onClicked: logic.reset() }
					Button { text: "Reset via IPC"; onClicked: logic.resetViaIpc() }
				}

				Row {
					anchors.horizontalCenter: parent.horizontalCenter
					spacing: Style.space(8)

					Button { text: "1s ticks"; onClicked: logic.setTickMs(1000) }
					Button { text: "250ms ticks"; onClicked: logic.setTickMs(250) }
				}

				Text {
					width: parent.width
					text: "Ticks: " + logic.getTickCount() + "  ·  every " + logic.tickMs + "ms"
					color: root.contentForeground
					font.family: root.contentFontFamily
					font.pixelSize: Style.font.caption
					horizontalAlignment: Text.AlignHCenter
				}
			}
		}
	}
}
