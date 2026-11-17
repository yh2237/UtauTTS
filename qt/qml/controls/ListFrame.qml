import QtQuick
import QtQuick.Controls
import UtauTTSStyle

Pane {
    id: root
    property bool framed: true

    padding: 0
    background: Rectangle {
        color: root.framed ? root.palette.base : "transparent"
    }

    StyleItem {
        parent: root
        anchors.fill: parent
        z: 1
        visible: root.framed
        element: StyleItem.Frame
    }
}
