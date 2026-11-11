import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.MenuItem {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding,
                            painter.minimumWidth)
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
        alignment: Qt.AlignLeft
        icon: control.icon
        text: control.text
        font: control.font
        color: control.highlighted ? control.palette.highlightedText : control.palette.text
    }

    arrow: Item {}
    indicator: Item {}

    background: StyleItem {
        id: painter
        element: StyleItem.MenuItem
        highlighted: control.highlighted
        checkable: control.checkable
        checked: control.checked
        hasSubmenu: control.subMenu !== null
        font: control.font
    }
}
