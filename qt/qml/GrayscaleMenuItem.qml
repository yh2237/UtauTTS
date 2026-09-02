pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

MenuItem {
    id: root

    contentItem: Text {
        text: root.text
        font: root.font
        color: root.enabled ? root.palette.windowText : root.palette.mid
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }
}
