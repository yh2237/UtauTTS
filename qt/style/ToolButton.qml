import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.ToolButton {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom
    spacing: painter.spacing
    icon.width: painter.extent
    icon.height: painter.extent

    contentItem: IconLabel {
        spacing: control.spacing
        mirrored: control.mirrored
        display: control.display
        icon: control.icon
        text: control.text
        font: control.font
        color: control.palette.buttonText
    }

    background: StyleItem {
        id: painter
        element: StyleItem.ToolButton
        hovered: control.hovered
        pressed: control.down
        checked: control.checked
        focused: control.visualFocus
        flat: true
        font: control.font
    }
}
