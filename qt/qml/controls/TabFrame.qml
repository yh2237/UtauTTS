pragma ComponentBehavior: Bound

import QtQuick
import UtauTTSStyle

Item {
    id: root

    readonly property int overlap: frame.pixelMetric(StyleItem.TabBarBaseOverlap)
    readonly property real frameWidth: frame.paddingLeft
    property int contentMargin: Theme.spacing
    default property alias content: contentHost.data

    StyleItem {
        id: frame
        anchors.fill: parent
        element: StyleItem.TabFrame
    }

    Item {
        id: contentHost
        anchors.fill: parent
        anchors.margins: root.contentMargin
        clip: true
    }
}
