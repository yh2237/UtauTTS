pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

Rectangle {
    id: root
    property string hudText: ""

    visible: hudText.length > 0
    width: hudLabel.implicitWidth + Theme.spacingXLarge
    height: hudLabel.implicitHeight + Theme.spacing
    radius: Theme.radiusLarge
    color: Theme.overlay

    Label {
        id: hudLabel
        anchors.centerIn: parent
        text: root.hudText
        color: Theme.overlayText
    }
}
