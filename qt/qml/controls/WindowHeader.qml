pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ToolBar {
    id: root
    property string heading: ""
    property int contentMargin: 12
    property int closeButtonMargin: 0
    signal closeClicked()

    RowLayout {
        anchors.fill: parent
        Label {
            Layout.fillWidth: true
            Layout.leftMargin: root.contentMargin
            text: root.heading
            font.bold: true
            elide: Text.ElideRight
        }
        ToolButton {
            Layout.rightMargin: root.closeButtonMargin
            text: "✕"
            font.pixelSize: 22
            Layout.preferredWidth: 52
            Layout.preferredHeight: 48
            onClicked: root.closeClicked()
        }
    }
}
