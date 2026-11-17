import QtQuick

Item {
    id: root

    readonly property int overlap: 1
    property int contentMargin: Theme.spacing
    default property alias content: contentHost.data

    Rectangle {
        anchors.fill: parent
        color: Theme.view
    }

    Item {
        id: contentHost
        anchors.fill: parent
        anchors.margins: root.contentMargin
        clip: true
    }

    Rectangle {
        anchors.fill: parent
        color: "transparent"
        border.width: 1
        border.color: Theme.frameLine
    }
}
