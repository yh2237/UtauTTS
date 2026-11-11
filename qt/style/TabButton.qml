import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.TabButton {
    id: control

    readonly property int tabIndex: T.TabBar.index
    readonly property int tabCount: T.TabBar.tabBar ? T.TabBar.tabBar.count : 1

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom
    z: checked

    contentItem: IconLabel {
        mirrored: control.mirrored
        display: control.display
        icon: control.icon
        text: control.text
        font: control.font
        color: control.palette.windowText
    }

    background: StyleItem {
        id: painter
        element: StyleItem.Tab
        hovered: control.hovered
        pressed: control.down
        checked: control.checked
        focused: control.visualFocus
        tabPosition: control.tabCount <= 1 ? StyleItem.OnlyOneTab
                     : control.tabIndex === 0 ? StyleItem.Beginning
                     : control.tabIndex === control.tabCount - 1 ? StyleItem.End
                     : StyleItem.Middle
        font: control.font
    }
}
