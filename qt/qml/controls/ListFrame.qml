import QtQuick
import QtQuick.Controls
import UtauTTSStyle

Pane {
    id: root
    property bool framed: true

    padding: framed ? frame.paddingLeft : 0
    background: Rectangle {
        color: root.framed ? root.palette.base : "transparent"

        StyleItem {
            id: frame
            anchors.fill: parent
            visible: root.framed
            element: StyleItem.Frame
        }
    }
}
