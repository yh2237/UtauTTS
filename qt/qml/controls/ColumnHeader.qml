import QtQuick
import QtQuick.Controls
import UtauTTSStyle

Item {
    id: root

    property alias text: label.text
    property int position: StyleItem.Middle
    property int sortIndicator: StyleItem.Unsorted
    property bool clickable: false
    readonly property real textPadding: painter.paddingLeft
    signal clicked()

    implicitHeight: label.implicitHeight + painter.paddingTop + painter.paddingBottom
    implicitWidth: label.implicitWidth + painter.paddingLeft + painter.paddingRight

    StyleItem {
        id: painter
        anchors.fill: parent
        element: StyleItem.HeaderSection
        orientation: Qt.Horizontal
        tabPosition: root.position
        sortIndicator: root.sortIndicator
        hovered: mouse.containsMouse
        pressed: mouse.pressed
        font: label.font
    }

    Label {
        id: label
        anchors.fill: parent
        anchors.leftMargin: painter.paddingLeft
        // SE_HeaderArrow の幅（高さの半分）と左右の余白を、矢印の分として空ける。
        anchors.rightMargin: painter.paddingRight
                + (root.sortIndicator !== StyleItem.Unsorted ? root.height / 2 + 2 * painter.paddingRight : 0)
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    MouseArea {
        id: mouse
        anchors.fill: parent
        enabled: root.clickable
        hoverEnabled: true
        onClicked: root.clicked()
    }
}
