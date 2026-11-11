import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.ItemDelegate {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding,
                             implicitIndicatorHeight + topPadding + bottomPadding)

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
        alignment: control.display === IconLabel.IconOnly || control.display === IconLabel.TextUnderIcon ? Qt.AlignCenter : Qt.AlignLeft
        icon: control.icon
        text: control.text
        font: control.font
        color: control.highlighted || control.checked ? control.palette.highlightedText : control.palette.text
    }

    background: StyleItem {
        id: painter
        element: StyleItem.ItemView
        hovered: control.hovered
        highlighted: control.highlighted
        checked: control.checked
        focused: control.visualFocus
        font: control.font
    }
}
