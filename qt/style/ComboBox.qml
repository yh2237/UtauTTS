pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Window
import QtQuick.Templates as T

T.ComboBox {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom

    delegate: MenuItem {
        required property var model
        required property int index
        width: ListView.view.width
        leftPadding: control.leftPadding - control.popup.leftPadding
        text: model[control.textRole]
        highlighted: control.highlightedIndex === index
        hoverEnabled: control.hoverEnabled
    }

    indicator: Item {}

    contentItem: T.TextField {
        implicitWidth: contentWidth + leftPadding + rightPadding
        implicitHeight: contentHeight + topPadding + bottomPadding
        text: control.editable ? control.editText : control.displayText
        enabled: control.editable
        autoScroll: control.editable
        readOnly: control.down
        inputMethodHints: control.inputMethodHints
        validator: control.validator
        selectByMouse: control.selectTextByMouse
        font: control.font
        color: control.editable ? control.palette.text : control.palette.buttonText
        selectionColor: control.palette.highlight
        selectedTextColor: control.palette.highlightedText
        verticalAlignment: Text.AlignVCenter
    }

    background: StyleItem {
        id: painter
        element: StyleItem.ComboBox
        hovered: control.hovered
        pressed: control.pressed || control.popup.visible
        focused: control.visualFocus || control.contentItem.activeFocus
        editable: control.editable
        flat: control.flat
        font: control.font
    }

    popup: T.Popup {
        width: control.width
        height: Math.min(contentItem.implicitHeight + topPadding + bottomPadding,
                         control.Window.height - topMargin - bottomMargin)
        leftPadding: panel.paddingLeft
        topPadding: panel.paddingTop
        rightPadding: panel.paddingRight
        bottomPadding: panel.paddingBottom
        palette: control.palette

        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: control.delegateModel
            currentIndex: control.highlightedIndex
            highlightRangeMode: ListView.ApplyRange
            highlightMoveDuration: 0
            T.ScrollBar.vertical: ScrollBar {}
        }

        background: StyleItem {
            id: panel
            element: StyleItem.MenuPanel
        }
    }
}
