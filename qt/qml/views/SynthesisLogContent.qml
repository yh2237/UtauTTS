pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: content
    required property var backend
    required property var translator
    signal closeRequested()

    // 成否はbackend.errorで判断する。合成を始めるときに消える。
    readonly property bool failed: !content.backend.busy && content.backend.error.length > 0

    Label {
        Layout.fillWidth: true
        visible: content.backend.busy || content.backend.logLines.length > 0
        text: content.backend.busy
              ? content.translator.tr("log.synthesizing")
              : content.failed ? content.translator.tr("log.failed")
                               : content.translator.tr("log.completed")
        color: content.failed ? Theme.negative : content.palette.text
        font.bold: true
        wrapMode: Text.Wrap
    }

    ScrollView {
        id: synthesisLogScroll
        Layout.fillWidth: true
        Layout.fillHeight: true
        TextArea {
            id: synthesisLogText
            width: synthesisLogScroll.availableWidth
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
