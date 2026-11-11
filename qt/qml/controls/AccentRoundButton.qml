import QtQuick
import QtQuick.Templates as T

T.Button {
    id: root

    property string glyph
    property int glyphSize: Theme.iconLarge

    implicitWidth: Theme.largeButton
    implicitHeight: Theme.largeButton
    hoverEnabled: true

    contentItem: Text {
        text: root.glyph
        color: root.enabled ? Theme.buttonText : Theme.disabledText
        font.family: Theme.iconFont
        font.pixelSize: root.glyphSize
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }

    background: Rectangle {
        radius: width / 2
        color: !root.enabled ? Theme.disabledButton
             : root.down ? Theme.buttonPressed : root.hovered ? Theme.buttonHover : Theme.button
        border.width: root.visualFocus ? 2 : 1
        border.color: root.enabled ? Theme.selection : Theme.frameLine
    }
}
