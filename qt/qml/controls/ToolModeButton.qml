import QtQuick
import QtQuick.Templates as T

T.ToolButton {
    id: root

    property string glyph

    implicitWidth: implicitHeight
    implicitHeight: Theme.icon + 2 * Theme.spacingXSmall
    checkable: true
    hoverEnabled: true

    contentItem: Text {
        text: root.glyph
        color: root.checked ? Theme.selectionText : root.palette.buttonText
        opacity: root.enabled ? 1 : 0.35
        font.family: Theme.iconFont
        font.pixelSize: Theme.iconSmall
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }

    background: Rectangle {
        radius: Theme.radiusLarge
        color: root.checked ? Qt.rgba(Theme.selection.r, Theme.selection.g, Theme.selection.b, root.down ? 0.8 : 1)
             : root.down || root.hovered ? Theme.subtleHover : "transparent"
        border.width: root.visualFocus ? 1 : 0
        border.color: Theme.focusOutline
    }
}
