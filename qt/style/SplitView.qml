import QtQuick
import QtQuick.Templates as T

T.SplitView {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    handle: StyleItem {
        implicitWidth: control.orientation === Qt.Horizontal ? extent : control.width
        implicitHeight: control.orientation === Qt.Horizontal ? control.height : extent
        element: StyleItem.Splitter
        orientation: control.orientation
        hovered: T.SplitHandle.hovered
        pressed: T.SplitHandle.pressed
    }
}
