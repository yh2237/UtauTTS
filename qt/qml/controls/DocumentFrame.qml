import QtQuick

// 上の線は、選んだ DocumentTabButton が重なって隠す。
Item {
    id: root

    readonly property int overlap: 1
    readonly property int frameWidth: 1
    property int contentMargin: Theme.spacing
    default property alias content: contentHost.data

    Rectangle {
        anchors.fill: parent
        color: Theme.view
        border.width: root.frameWidth
        border.color: Theme.frameLine
    }

    Item {
        id: contentHost
        anchors.fill: parent
        anchors.margins: root.contentMargin
        clip: true
    }
}
