pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ToolButton {
    id: root
    required property var translator
    signal resetRequested()

    Layout.preferredWidth: 24
    Layout.minimumWidth: 24
    Layout.maximumWidth: 24
    Layout.preferredHeight: 24
    Layout.alignment: Qt.AlignVCenter
    contentItem: BreezeIcon {
        anchors.centerIn: parent
        width: 18
        height: 18
        source: "qrc:/icons/breeze-view-refresh.svg"
        iconColor: root.palette.buttonText
    }
    onClicked: resetRequested()
    ToolTip.visible: hovered
    ToolTip.text: translator.tr("common.reset")
}
