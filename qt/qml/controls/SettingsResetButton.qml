pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

IconButton {
    id: root
    required property var translator
    signal resetRequested()

    Layout.alignment: Qt.AlignVCenter
    glyph: "\ue5d5"
    onClicked: resetRequested()
    ToolTip.visible: hovered
    ToolTip.text: translator.tr("common.reset")
}
