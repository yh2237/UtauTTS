pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Window {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator

    title: root.translator.tr("log.title")
    visible: false
    transientParent: hostWindow
    width: 720
    height: 420
    minimumWidth: 720
    maximumWidth: 720
    minimumHeight: 420
    maximumHeight: 420
    palette: hostPalette
    color: palette.window

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 8

        Label {
            Layout.fillWidth: true
            text: root.backend.busy ? root.translator.tr("log.synthesizing") : root.translator.tr("log.title")
            font.bold: true
        }

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            TextArea {
                id: synthesisLogText
                width: root.width - 36
                text: root.backend.logLines.join("\n")
                readOnly: true
                selectByMouse: true
                wrapMode: TextEdit.Wrap
                onTextChanged: cursorPosition = length
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Item {
                Layout.fillWidth: true
            }
            Button {
                text: root.translator.tr("common.close")
                onClicked: root.close()
            }
        }
    }
}
