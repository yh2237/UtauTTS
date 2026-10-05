pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

Slider {
    id: root
    signal resetRequested()

    focusPolicy: Qt.StrongFocus
    hoverEnabled: true
    stepSize: 1
    snapMode: Slider.SnapAlways
    property double previousTapTime: 0
    onPressedChanged: {
        if (pressed)
            forceActiveFocus();
    }

    // 入力を奪わず、クリックとドラッグを区別する。
    PointHandler {
        id: pointer
        acceptedButtons: Qt.LeftButton
        property point pressPosition
        property point lastPosition
        property point previousTapPosition
        property bool dragged: false

        onPointChanged: {
            if (active) {
                lastPosition = point.position;
                if (Math.hypot(lastPosition.x - pressPosition.x,
                               lastPosition.y - pressPosition.y) > Qt.styleHints.startDragDistance)
                    dragged = true;
            }
        }
        onActiveChanged: {
            if (active) {
                pressPosition = point.position;
                lastPosition = point.position;
                dragged = false;
            } else {
                const now = Date.now();
                const distance = Math.hypot(lastPosition.x - previousTapPosition.x,
                                            lastPosition.y - previousTapPosition.y);
                if (!dragged && root.previousTapTime > 0
                        && now - root.previousTapTime <= Qt.styleHints.mouseDoubleClickInterval
                        && distance <= Qt.styleHints.mouseDoubleClickDistance) {
                    root.previousTapTime = 0;
                    // releaseによる値更新でリセットが上書きされないよう遅延する。
                    Qt.callLater(root.resetRequested);
                } else {
                    root.previousTapTime = dragged ? 0 : now;
                    previousTapPosition = lastPosition;
                }
            }
        }
    }
}
