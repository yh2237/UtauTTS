import QtQuick
import QtQuick.Templates as T

T.TabButton {
    id: control

    implicitWidth: implicitContentWidth + leftPadding + rightPadding
    implicitHeight: implicitContentHeight + topPadding + bottomPadding
    leftPadding: Theme.spacingLarge
    rightPadding: Theme.spacingLarge
    topPadding: Theme.spacingMedium + (checked ? 0 : inactiveInset)
    bottomPadding: Theme.spacingMedium
    hoverEnabled: true
    z: checked ? 1 : 0

    readonly property int inactiveInset: 2
    // glyph を入れると、文字の代わりにアイコンを出す。
    property string glyph

    contentItem: Text {
        text: control.glyph || control.text
        font.family: control.glyph ? Theme.iconFont : control.font.family
        font.pixelSize: control.glyph ? Theme.iconSmall : control.font.pixelSize
        color: control.checked ? Theme.viewText : Theme.inactiveText
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }

    background: Item {
        Rectangle {
            id: shape
            anchors.fill: parent
            anchors.topMargin: control.checked ? 0 : control.inactiveInset
            topLeftRadius: Theme.radiusLarge
            topRightRadius: Theme.radiusLarge
            color: control.checked ? Theme.view : control.hovered ? Theme.subtleHover : Theme.window
            border.width: 1
            border.color: Theme.frameLine
        }

        // 選んだタブは、下の枠とつながって見えるように下の線を消す。
        Rectangle {
            visible: control.checked
            x: 1
            y: parent.height - 1
            width: parent.width - 2
            height: 1
            color: Theme.view
        }

        Rectangle {
            visible: control.visualFocus
            anchors.fill: shape
            anchors.margins: Theme.spacingXSmall + 1
            radius: Theme.radius
            color: "transparent"
            border.color: Theme.focusOutline
        }
    }
}
