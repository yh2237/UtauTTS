import QtQuick
import QtQuick.Templates as T

T.TabButton {
    id: control

    implicitWidth: implicitContentWidth + leftPadding + rightPadding
    implicitHeight: implicitContentHeight + topPadding + bottomPadding
    leftPadding: Theme.spacingXLarge
    rightPadding: Theme.spacingXLarge
    topPadding: Theme.spacing
    bottomPadding: Theme.spacing
    hoverEnabled: true

    contentItem: Text {
        text: control.text
        font: control.font
        color: control.checked ? Theme.viewText : Theme.inactiveText
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }

    background: Rectangle {
        color: control.checked ? Theme.view : control.hovered ? Theme.subtleHover : "transparent"

        Rectangle {
            visible: control.visualFocus
            anchors.fill: parent
            anchors.margins: Theme.spacingXSmall
            radius: Theme.radius
            color: "transparent"
            border.color: Theme.focusOutline
        }

        Rectangle {
            visible: control.checked
            width: 1
            height: parent.height
            color: Theme.frameLine
        }
        Rectangle {
            visible: control.checked
            width: parent.width
            height: 1
            color: Theme.frameLine
        }
        Rectangle {
            visible: control.checked
            x: parent.width - 1
            width: 1
            height: parent.height
            color: Theme.frameLine
        }
    }
}
