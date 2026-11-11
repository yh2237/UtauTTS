import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.MenuBarItem {
    id: control

    readonly property bool open: control.down || (control.menu !== null && control.menu.visible)

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom
    icon.width: painter.extent
    icon.height: painter.extent

    contentItem: IconLabel {
        mirrored: control.mirrored
        display: control.display
        alignment: Qt.AlignCenter
        icon: control.icon
        text: control.text
        font: control.font
        color: control.open ? control.palette.highlightedText : control.palette.windowText
    }

    background: StyleItem {
        id: painter
        element: StyleItem.MenuBarItem
        highlighted: control.open
        pressed: control.open
        font: control.font
    }
}
