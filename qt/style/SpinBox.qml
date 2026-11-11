import QtQuick
import QtQuick.Templates as T

T.SpinBox {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            contentItem.implicitWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom

    validator: IntValidator {
        locale: control.locale.name
        bottom: Math.min(control.from, control.to)
        top: Math.max(control.from, control.to)
    }

    contentItem: TextInput {
        z: 2
        text: control.displayText
        font: control.font
        color: control.palette.text
        selectionColor: control.palette.highlight
        selectedTextColor: control.palette.highlightedText
        horizontalAlignment: Qt.AlignLeft
        verticalAlignment: Qt.AlignVCenter
        readOnly: !control.editable
        validator: control.validator
        inputMethodHints: control.inputMethodHints
        clip: width < implicitWidth
    }

    up.indicator: Item {
        x: painter.upRect.x
        y: painter.upRect.y
        width: painter.upRect.width
        height: painter.upRect.height
    }

    down.indicator: Item {
        x: painter.downRect.x
        y: painter.downRect.y
        width: painter.downRect.width
        height: painter.downRect.height
    }

    background: StyleItem {
        id: painter
        element: StyleItem.SpinBox
        hovered: control.hovered
        focused: control.activeFocus
        upEnabled: control.value < Math.max(control.from, control.to) || control.wrap
        downEnabled: control.value > Math.min(control.from, control.to) || control.wrap
        pressedControl: control.up.pressed ? StyleItem.Up
                        : control.down.pressed ? StyleItem.Down
                        : StyleItem.None
        hoveredControl: control.up.hovered ? StyleItem.Up
                        : control.down.hovered ? StyleItem.Down
                        : StyleItem.None
        font: control.font
    }
}
