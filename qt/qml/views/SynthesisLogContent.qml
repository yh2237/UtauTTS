pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: content
    required property var backend
    required property var translator
    signal closeRequested()

    Label {
        Layout.fillWidth: true
        visible: content.backend.busy
        text: content.translator.tr("log.synthesizing")
        font.bold: true
    }

    ScrollView {
        Layout.fillWidth: true
        Layout.fillHeight: true
        TextArea {
            id: synthesisLogText
            width: content.width - 24
            text: content.backend.logLines.join("\n")
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
            text: content.translator.tr("common.close")
            onClicked: content.closeRequested()
        }
    }
}
