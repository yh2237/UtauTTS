pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ToolBar {
    id: root
    property string heading: ""
    property int contentMargin: Theme.spacingLarge
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
        IconButton {
            Layout.rightMargin: root.closeButtonMargin
            glyph: "\ue5cd"
            glyphSize: Theme.icon
            Layout.preferredWidth: Theme.touchTarget
            Layout.preferredHeight: Theme.touchTarget
            onClicked: root.closeClicked()
        }
    }
}
