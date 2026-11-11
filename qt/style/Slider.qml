import QtQuick
import QtQuick.Templates as T

T.Slider {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitHandleWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitHandleHeight + topPadding + bottomPadding)

    handle: Item {
        implicitWidth: control.horizontal ? painter.handleLength : painter.extent
        implicitHeight: control.horizontal ? painter.extent : painter.handleLength
        x: control.horizontal ? Math.round(control.visualPosition * (control.availableWidth - width))
                              : painter.handleRect.x
        y: control.horizontal ? painter.handleRect.y
                              : Math.round(control.visualPosition * (control.availableHeight - height))
    }

    background: StyleItem {
        id: painter
        implicitWidth: control.horizontal ? handleLength * 2 : extent
        implicitHeight: control.horizontal ? extent : handleLength * 2
        element: StyleItem.Slider
        orientation: control.orientation
        value: control.position
        focused: control.visualFocus
        hovered: control.hovered
        pressedControl: control.pressed ? StyleItem.Handle : StyleItem.None
        hoveredControl: hover.hovered && hitTest(hover.point.position.x, hover.point.position.y) === StyleItem.Handle
                        ? StyleItem.Handle : StyleItem.None
    }

    HoverHandler {
        id: hover
    }
}
