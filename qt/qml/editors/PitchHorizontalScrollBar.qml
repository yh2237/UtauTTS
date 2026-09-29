pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: root
    required property var editor
    property color trackColor
    property color thumbColor
    implicitHeight: 14
    visible: root.editor.horizontalMaximum > 0

    Rectangle {
        id: track
        anchors.verticalCenter: parent.verticalCenter
        width: parent.width
        height: 4
        radius: height / 2
        color: root.trackColor
    }

    Rectangle {
        id: thumb
        readonly property real minimumWidth: 28
        width: Math.max(minimumWidth, track.width * root.editor.horizontalVisibleRatio)
        height: 10
        radius: height / 2
        y: (parent.height - height) / 2
        x: (track.width - width) * root.editor.horizontalPosition
        color: root.thumbColor
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.SizeHorCursor
        onPressed: mouse => setOffset(mouse.x)
        onPositionChanged: mouse => {
            if (pressed)
                setOffset(mouse.x);
        }

        function setOffset(x) {
            const travel = track.width - thumb.width;
            if (travel <= 0)
                return;
            const position = Math.max(0, Math.min(1, (x - thumb.width / 2) / travel));
            root.editor.horizontalOffset = position * root.editor.horizontalMaximum;
        }
    }
}
