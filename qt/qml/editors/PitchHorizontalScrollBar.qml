pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

ScrollBar {
    id: root
    required property var editor

    orientation: Qt.Horizontal
    policy: ScrollBar.AlwaysOn
    size: root.editor.horizontalVisibleRatio

    Binding on position {
        value: (1 - root.size) * root.editor.horizontalPosition
        when: !root.pressed
        restoreMode: Binding.RestoreNone
    }

    onPositionChanged: {
        const travel = 1 - root.size;
        if (travel <= 0)
            return;
        const offset = Math.max(0, Math.min(1, root.position / travel)) * root.editor.horizontalMaximum;
        if (Math.abs(offset - root.editor.horizontalOffset) > 0.5) {
            if (root.editor.stopScrollAnimation)
                root.editor.stopScrollAnimation();
            root.editor.horizontalOffset = offset;
        }
    }
}
