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
            text: root.heading
            font.bold: true
            elide: Text.ElideRight
        }
        ToolButton {
            text: "✕"
            onClicked: root.closeClicked()
        }
    }
}
