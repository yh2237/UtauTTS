import QtQuick
import QtQuick.Templates as T
import UtauTTS.Platform 1.0

T.ScrollBar {
    id: control

    readonly property bool touch: Platform.isMobile
    readonly property real thickness: touch ? touchThickness : painter.extent
    readonly property real buttonLength: touch ? 0 : painter.lineButtonLength
    readonly property real trackLength: (vertical ? height : width) - buttonLength * 2
    readonly property int repeatDelay: 500
    readonly property int repeatInterval: 50
    readonly property real touchThickness: 6

    implicitWidth: vertical ? thickness : thickness * 2 + painter.handleLength
    implicitHeight: vertical ? thickness * 2 + painter.handleLength : thickness

    visible: policy === T.ScrollBar.AlwaysOn || (policy === T.ScrollBar.AsNeeded && size < 1.0)
    topPadding: vertical ? buttonLength : 0
    bottomPadding: vertical ? buttonLength : 0
    leftPadding: vertical ? 0 : buttonLength
    rightPadding: vertical ? 0 : buttonLength
    minimumSize: !touch && trackLength > 0 ? Math.min(1, painter.handleLength / trackLength) : 0

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

        visible: !bar.touch
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
        visible: !control.touch
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

    contentItem: Rectangle {
        implicitWidth: control.thickness
        implicitHeight: control.thickness
        radius: width / 2
        color: control.pressed ? control.palette.dark : control.palette.mid
        opacity: control.touch && control.active ? 0.75 : 0
        Behavior on opacity { NumberAnimation { duration: 200 } }
    }

    HoverHandler {
        id: hover
        enabled: !control.touch
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
