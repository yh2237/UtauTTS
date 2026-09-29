pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: root

    property color surfaceColor: "transparent"
    property color borderColor: "transparent"
    property int contentMargin: 8
    property int topContentMargin: contentMargin
    property bool showSideBorders: true
    default property alias content: contentHost.data

    Rectangle {
        anchors.fill: parent
        radius: root.showSideBorders ? 4 : 0
        color: root.surfaceColor
        border.width: root.showSideBorders ? 1 : 0
        border.color: root.borderColor
    }

    Rectangle {
        visible: !root.showSideBorders
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 1
        color: root.borderColor
    }

    Rectangle {
        visible: !root.showSideBorders
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: root.borderColor
    }

    Item {
        id: contentHost
        anchors.fill: parent
        anchors.margins: root.contentMargin
        anchors.topMargin: root.topContentMargin
        clip: true
    }
}
