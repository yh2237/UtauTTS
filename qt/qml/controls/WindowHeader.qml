pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ToolBar {
    id: root
    property string heading: ""
    signal closeClicked()

    RowLayout {
        anchors.fill: parent
        Label {
            Layout.fillWidth: true
            Layout.leftMargin: 8
            text: root.heading
            font.bold: true
            elide: Text.ElideRight
        }
        ToolButton {
            text: "✕"
            font.pixelSize: 22
            Layout.preferredWidth: 52
            Layout.preferredHeight: 48
            onClicked: root.closeClicked()
        }
    }
}
