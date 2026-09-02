pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects

Item {
    id: root
    property url source
    property color iconColor: "black"

    implicitWidth: 22
    implicitHeight: 22

    Image {
        id: iconSource
        anchors.fill: parent
        source: root.source
        sourceSize: Qt.size(Math.ceil(width * Screen.devicePixelRatio),
                            Math.ceil(height * Screen.devicePixelRatio))
        visible: false
    }

    MultiEffect {
        anchors.fill: iconSource
        source: iconSource
        colorization: 1.0
        colorizationColor: root.iconColor
    }
}
