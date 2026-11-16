import QtQuick
import QtQuick.Templates as T

T.ScrollBar {
    id: control

    readonly property real thickness: painter.extent
    readonly property real buttonLength: painter.lineButtonLength
    readonly property real trackLength: (vertical ? height : width) - buttonLength * 2
    readonly property int repeatDelay: 500
    readonly property int repeatInterval: 50

    implicitWidth: vertical ? thickness : thickness * 2 + painter.handleLength
    implicitHeight: vertical ? thickness * 2 + painter.handleLength : thickness

    visible: policy === T.ScrollBar.AlwaysOn || (policy === T.ScrollBar.AsNeeded && size < 1.0)
    topPadding: vertical ? buttonLength : 0
    bottomPadding: vertical ? buttonLength : 0
    leftPadding: vertical ? 0 : buttonLength
    rightPadding: vertical ? 0 : buttonLength
    minimumSize: trackLength > 0 ? Math.min(1, painter.handleLength / trackLength) : 0

    component LineButton: MouseArea {
        id: button
        required property T.ScrollBar bar
        required property bool increasing

        function step() {
            if (increasing)
                bar.increase();
            else
                bar.decrease();
        }

        width: bar.vertical ? bar.width : bar.buttonLength
        height: bar.vertical ? bar.buttonLength : bar.height
        onPressed: {
            step();
            repeatTimer.interval = bar.repeatDelay;
            repeatTimer.start();
        }
        onReleased: repeatTimer.stop()
        onCanceled: repeatTimer.stop()

        Timer {
            id: repeatTimer
            repeat: true
            onTriggered: {
                interval = button.bar.repeatInterval;
                button.step();
            }
        }
    }

    background: StyleItem {
        id: painter
        element: StyleItem.ScrollBar
        orientation: control.orientation
        value: control.position
        size: control.size
        pressedControl: subLineButton.pressed ? StyleItem.SubLine
                        : addLineButton.pressed ? StyleItem.AddLine
                        : control.pressed ? StyleItem.Handle
                        : StyleItem.None
        hoveredControl: hover.hovered ? hitTest(hover.point.position.x, hover.point.position.y)
                                      : StyleItem.None
    }

    // つまみは background が描く。ここはつかむ範囲だけを持つ。
    contentItem: Item {
        implicitWidth: control.thickness
        implicitHeight: control.thickness
    }

    HoverHandler {
        id: hover
    }

    LineButton {
        id: subLineButton
        bar: control
        increasing: false
    }

    LineButton {
        id: addLineButton
        bar: control
        increasing: true
        x: control.vertical ? 0 : control.width - width
        y: control.vertical ? control.height - height : 0
    }
}
