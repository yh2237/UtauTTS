import QtQuick
import QtQuick.Controls

ToolButton {
    id: root

    property string glyph
    property int glyphSize: Theme.iconSmall
    implicitHeight: implicitWidth

    contentItem: Text {
        text: root.glyph
        color: root.palette.buttonText
        opacity: root.enabled ? 1 : 0.35
        font.family: Theme.iconFont
        font.pixelSize: root.glyphSize
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
}
