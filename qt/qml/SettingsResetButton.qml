pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ToolButton {
    id: root
    required property var translator
    signal resetRequested()

    Layout.preferredWidth: 24
    Layout.minimumWidth: 24
    Layout.maximumWidth: 24
    Layout.preferredHeight: 24
    Layout.alignment: Qt.AlignVCenter
    contentItem: Text {
        anchors.centerIn: parent
        width: 18
        height: 18
        text: "↻"
        color: root.palette.buttonText
        font.pixelSize: 17
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    onClicked: resetRequested()
    ToolTip.visible: hovered
    ToolTip.text: translator.tr("common.reset")
}
