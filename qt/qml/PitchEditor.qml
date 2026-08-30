pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: root
    property var translator: ({})
    property var points: []
    property var autoPoints: []
    property var morae: []
    property var moraDurations: []
    property var moraPositions: []
    property int defaultMoraDuration: 120
    property int defaultPauseDuration: 180
    property int minimumMoraDuration: 20
    property int maximumMoraDuration: 1000
    property int maximumPauseDuration: 3000
    property color accentColor: "#d35f6b"
    property color axisColor: "#c79298"
    property color gridColor: "#eadcdf"
    property color labelColor: "#66565a"
    property real moraWidth: 64
    property real sidePadding: 12
    signal pointsEdited(var points)
    signal pitchPointTouched(int index)
    signal moraDurationsEdited(var durations)
    signal moraPositionsEdited(var positions)
    property alias horizontalOffset: viewport.contentX
    readonly property real contentWidth: viewport.contentWidth
    readonly property real horizontalMaximum: Math.max(0, viewport.contentWidth - viewport.width)
    readonly property real horizontalVisibleRatio: viewport.contentWidth > 0 ? Math.min(1, viewport.width / viewport.contentWidth) : 1
    readonly property real horizontalPosition: horizontalMaximum > 0 ? viewport.contentX / horizontalMaximum : 0

    readonly property real durationScale: defaultMoraDuration > 0 ? moraWidth / defaultMoraDuration : 0.5
    readonly property real graphWidth: Math.max(width, sidePadding * 2 + root.totalDuration() * durationScale)

    function baseDurationAt(index) {
        const mora = index < root.morae.length ? root.morae[index] : null;
        if (!mora)
            return Math.max(1, root.defaultMoraDuration);
        const values = root.moraDurations || [];
        const value = index < values.length ? Number(values[index]) : 0;
        if (Number.isFinite(value) && value > 0)
            return Math.max(root.minimumDurationAt(index), Math.min(root.maximumDurationAt(index), value));
        const fallback = mora.pause ? root.defaultPauseDuration : root.defaultMoraDuration;
        return Math.max(root.minimumDurationAt(index), fallback);
    }

    function hasCompletePositions() {
        if (!root.morae.length || root.moraPositions.length < root.morae.length)
            return false;
        for (let index = 0; index < root.morae.length; ++index) {
            if (root.moraPositions[index] === null || root.moraPositions[index] === undefined)
                return false;
            if (!Number.isFinite(Number(root.moraPositions[index])))
                return false;
        }
        return true;
    }

    function positionAt(index) {
        if (index >= 0 && index < root.moraPositions.length) {
            const value = Number(root.moraPositions[index]);
            if (Number.isFinite(value))
                return Math.max(0, value);
        }
        return root.durationBefore(index);
    }

    function endTime() {
        const count = root.morae.length;
        if (!count)
            return 0;
        return root.positionAt(count - 1) + root.baseDurationAt(count - 1);
    }

    function durationAt(index) {
        if (root.hasCompletePositions())
            return root.durationFromPositions(index);
        return root.baseDurationAt(index);
    }

    function durationFromPositions(index) {
        const count = root.morae.length;
        if (index < 0 || index >= count)
            return 0;
        const start = root.positionAt(index);
        const end = index + 1 < count ? root.positionAt(index + 1) : root.endTime();
        return Math.max(root.minimumDurationAt(index),
                        Math.min(root.maximumDurationAt(index), end - start));
    }

    function totalDuration() {
        if (root.hasCompletePositions() && root.morae.length)
            return root.endTime();
        let total = 0;
        for (let index = 0; index < root.morae.length; ++index)
            total += root.baseDurationAt(index);
        return total;
    }

    function durationBefore(index) {
        let total = 0;
        for (let position = 0; position < index; ++position)
            total += root.baseDurationAt(position);
        return total;
    }

    function durationIsEditable(index) {
        return index >= 0 && index < root.morae.length;
    }

    function minimumDurationAt(index) {
        return root.minimumMoraDuration;
    }

    function maximumDurationAt(index) {
        return index >= 0 && index < root.morae.length && root.morae[index].pause
                ? root.maximumPauseDuration : root.maximumMoraDuration;
    }

    function pointIsEditable(index) {
        if (index < 0 || index >= root.points.length)
            return false;
        return index >= root.morae.length || !root.morae[index].pause;
    }

    function pointX(index) {
        return root.sidePadding + root.positionAt(index) * root.durationScale;
    }

    function durationValuesFromPositions() {
        const values = [];
        for (let index = 0; index < root.morae.length; ++index)
            values.push(Math.round(root.durationFromPositions(index)));
        return values;
    }

    function updatePositionAt(index, x, moveFollowing) {
        if (!root.durationIsEditable(index))
            return;
        const count = root.morae.length;
        const positions = (root.moraPositions || []).slice();
        for (let position = 0; position < count; ++position)
            positions[position] = root.positionAt(position);
        const previousMinimum = index > 0 ? root.minimumDurationAt(index - 1) : 0;
        const previousMaximum = index > 0 ? root.maximumDurationAt(index - 1)
                                          : Number.POSITIVE_INFINITY;
        const currentMinimum = root.minimumDurationAt(index);
        const currentMaximum = root.maximumDurationAt(index);
        const cursor = (x - root.sidePadding) / root.durationScale;
        if (moveFollowing) {
            // Shift時は操作語以降をまとめて動かし、前方の間隔だけを変える。
            const lower = index > 0
                          ? positions[index - 1] + previousMinimum - positions[index]
                          : -positions[index];
            const upper = index > 0
                          ? positions[index - 1] + previousMaximum - positions[index]
                          : Number.POSITIVE_INFINITY;
            const delta = cursor - positions[index];
            const clamped = Math.max(Math.max(-positions[index], lower),
                                     Math.min(upper, delta));
            for (let position = index; position < count; ++position)
                positions[position] += clamped;
        } else {
            // 通常時は語頭線だけを動かし、他の語位置を維持する。
            const following = index + 1 < count ? positions[index + 1] : root.endTime();
            const lower = Math.max(index > 0 ? positions[index - 1] + previousMinimum : 0,
                                   following - currentMaximum);
            const upper = Math.min(index > 0 ? positions[index - 1] + previousMaximum
                                             : Number.POSITIVE_INFINITY,
                                   following - currentMinimum);
            positions[index] = Math.max(lower, Math.min(upper, cursor));
        }
        root.moraPositions = positions;
        root.moraDurations = root.durationValuesFromPositions();
        canvas.requestPaint();
    }

    function updateEndPositionAt(x) {
        const count = root.morae.length;
        if (!count || !root.durationIsEditable(count - 1))
            return;
        const start = root.positionAt(count - 1);
        const cursor = (x - root.sidePadding) / root.durationScale;
        const durations = root.moraDurations.slice();
        durations[count - 1] = Math.max(root.minimumDurationAt(count - 1),
                                        Math.min(root.maximumDurationAt(count - 1),
                                                 cursor - start));
        root.moraDurations = durations;
        canvas.requestPaint();
    }

    function resetSingleDurationAt(index) {
        if (!root.durationIsEditable(index))
            return false;
        const count = root.morae.length;
        const positions = (root.moraPositions || []).slice();
        for (let position = 0; position < count; ++position)
            positions[position] = root.positionAt(position);
        const targetDuration = root.morae[index].pause
                ? root.defaultPauseDuration : root.defaultMoraDuration;
        const current = root.durationFromPositions(index);
        if (Math.abs(current - targetDuration) < 0.5)
            return false;
        if (index + 1 < count) {
            const lower = positions[index] + root.minimumDurationAt(index);
            const upper = index + 2 < count
                          ? positions[index + 2] - root.minimumDurationAt(index + 1)
                          : Math.max(positions[index] + targetDuration,
                                     root.endTime() + root.maximumDurationAt(index));
            positions[index + 1] = Math.max(lower, Math.min(upper, positions[index] + targetDuration));
        } else {
            root.moraDurations[index] = targetDuration;
        }
        root.moraPositions = positions;
        return true;
    }

    function resetDurationAt(index) {
        if (!root.durationIsEditable(index))
            return;
        const start = Math.max(0, index - 1);
        const end = Math.min(root.morae.length - 1, index + 1);
        let changed = false;
        for (let position = start; position <= end; ++position) {
            if (!root.durationIsEditable(position))
                continue;
            if (root.resetSingleDurationAt(position))
                changed = true;
        }
        if (!changed)
            return;
        root.moraDurations = root.durationValuesFromPositions();
        root.moraDurationsEdited(root.moraDurations.slice());
        root.moraPositionsEdited(root.moraPositions.slice());
        canvas.requestPaint();
    }

    function updatePitchAt(index, y) {
        if (index < 0 || index >= root.points.length || !root.pointIsEditable(index))
            return;
        const values = root.points.slice();
        const center = canvas.height / 2;
        const scale = Math.max(.05, Math.min(.36, canvas.height / 760));
        const desired = Math.max(-300, Math.min(300, (center - y) / scale));
        const automatic = index < root.autoPoints.length ? Number(root.autoPoints[index]) : 0;
        values[index] = Math.round(desired - (Number.isFinite(automatic) ? automatic : 0));
        root.pitchPointTouched(index);
        root.points = values;
        canvas.requestPaint();
    }

    function resetPitchAt(index) {
        if (index < 0 || index >= root.points.length || !root.pointIsEditable(index))
            return;
        const values = root.points.slice();
        root.pitchPointTouched(index);
        values[index] = 0;
        root.points = values;
        root.pointsEdited(values.slice());
        canvas.requestPaint();
    }

    function pitchAt(index) {
        const automatic = index < root.autoPoints.length ? Number(root.autoPoints[index]) : 0;
        const manual = index < root.points.length ? Number(root.points[index]) : 0;
        return (Number.isFinite(automatic) ? automatic : 0)
                + (Number.isFinite(manual) ? manual : 0);
    }

    function nearestEditablePoint(x) {
        let best = -1;
        let distance = Number.POSITIVE_INFINITY;
        for (let index = 0; index < root.points.length; ++index) {
            if (!root.pointIsEditable(index))
                continue;
            const candidateDistance = Math.abs(root.pointX(index) - x);
            if (candidateDistance < distance) {
                best = index;
                distance = candidateDistance;
            }
        }
        return best;
    }

    Flickable {
        id: viewport
        anchors.fill: parent
        clip: true
        contentWidth: root.graphWidth
        contentHeight: height
        boundsBehavior: Flickable.StopAtBounds
        interactive: false

        Item {
            id: graph
            width: root.graphWidth
            height: viewport.height

            Canvas {
                id: canvas
                anchors.fill: parent
                anchors.bottomMargin: 64
                onWidthChanged: requestPaint()
                onHeightChanged: requestPaint()
                onPaint: {
                    const ctx = getContext("2d");
                    ctx.reset();
                    ctx.clearRect(0, 0, width, height);
                    const center = height / 2;
                    const scale = Math.max(.05, Math.min(.36, height / 760));
                    for (const cents of [-300, 0, 300]) {
                        ctx.strokeStyle = cents === 0 ? root.axisColor : root.gridColor;
                        ctx.setLineDash(cents === 0 ? [] : [4, 5]);
                        ctx.beginPath();
                        ctx.moveTo(0, center - cents * scale);
                        ctx.lineTo(width, center - cents * scale);
                        ctx.stroke();
                    }
                    ctx.setLineDash([]);
                    ctx.strokeStyle = root.axisColor;
                    ctx.globalAlpha = 0.45;
                    ctx.lineWidth = 1;
                    for (let index = 0; index < root.morae.length; ++index) {
                        if (!root.durationIsEditable(index))
                            continue;
                        const x = root.pointX(index);
                        ctx.beginPath();
                        ctx.moveTo(x, 0);
                        ctx.lineTo(x, height);
                        ctx.stroke();
                    }
                    if (root.durationIsEditable(root.morae.length - 1)) {
                        const endX = root.sidePadding + root.endTime() * root.durationScale;
                        ctx.beginPath();
                        ctx.moveTo(endX, 0);
                        ctx.lineTo(endX, height);
                        ctx.stroke();
                    }
                    ctx.globalAlpha = 1;
                    if (!root.points.length)
                        return;
                    ctx.strokeStyle = root.accentColor;
                    ctx.fillStyle = root.accentColor;
                    ctx.lineWidth = 2;
                    ctx.beginPath();
                    let started = false;
                    for (let index = 0; index < root.points.length; ++index) {
                        if (!root.pointIsEditable(index))
                            continue;
                        const x = root.pointX(index);
                        const y = center - root.pitchAt(index) * scale;
                        if (started)
                            ctx.lineTo(x, y);
                        else {
                            ctx.moveTo(x, y);
                            started = true;
                        }
                    }
                    ctx.stroke();
                    for (let index = 0; index < root.points.length; ++index) {
                        if (!root.pointIsEditable(index))
                            continue;
                        ctx.beginPath();
                        ctx.arc(root.pointX(index), center - root.pitchAt(index) * scale, 6, 0, Math.PI * 2);
                        ctx.fill();
                    }
                }
            }

            Item {
                width: root.graphWidth
                height: 64
                anchors.left: parent.left
                anchors.bottom: parent.bottom
                Repeater {
                    model: root.morae
                    delegate: Column {
                        id: pointColumn
                        required property var modelData
                        required property int index
                        width: root.moraWidth
                        height: parent.height
                        x: root.pointX(pointColumn.index) - width / 2
                        spacing: 1

                        Text {
                            width: parent.width
                            text: pointColumn.modelData.mora || root.translator.tr("pitch.emptyMora")
                            horizontalAlignment: Text.AlignHCenter
                            color: root.labelColor
                            elide: Text.ElideRight
                        }
                        TextInput {
                            width: parent.width - 8
                            height: 24
                            anchors.horizontalCenter: parent.horizontalCenter
                            visible: root.pointIsEditable(pointColumn.index)
                            text: pointColumn.index < root.points.length ? Math.round(root.pitchAt(pointColumn.index)).toString() : "0"
                            horizontalAlignment: TextInput.AlignHCenter
                            color: root.labelColor
                            selectByMouse: true
                            validator: IntValidator {
                                bottom: -300
                                top: 300
                            }
                            onEditingFinished: {
                                const parsed = parseInt(text);
                                if (isNaN(parsed)) {
                                    text = Math.round(root.pitchAt(pointColumn.index)).toString();
                                    return;
                                }
                                const values = root.points.slice();
                                const automatic = pointColumn.index < root.autoPoints.length
                                        ? Number(root.autoPoints[pointColumn.index]) : 0;
                                values[pointColumn.index] = Math.round(
                                        Math.max(-300, Math.min(300, parsed))
                                        - (Number.isFinite(automatic) ? automatic : 0));
                                root.points = values;
                                root.pitchPointTouched(pointColumn.index);
                                root.pointsEdited(values.slice());
                            }
                        }
                        Text {
                            width: parent.width
                            text: root.durationIsEditable(pointColumn.index)
                                  ? Math.round(root.durationAt(pointColumn.index)) + " ms" : ""
                            horizontalAlignment: Text.AlignHCenter
                            color: root.labelColor
                            opacity: 0.75
                            font.pixelSize: 10

                            MouseArea {
                                anchors.fill: parent
                                onDoubleClicked: root.resetDurationAt(pointColumn.index)
                            }
                        }
                    }
                }
            }

            Repeater {
                model: root.morae
                delegate: Item {
                    required property var modelData
                    required property int index
                    visible: root.durationIsEditable(index)
                    x: root.pointX(index) - width / 2
                    width: 14
                    height: parent.height - 64
                    z: 2

                    MouseArea {
                        anchors.fill: parent
                        property real pressX: 0
                        property real pressY: 0
                        property int dragMode: 0
                        property bool shiftFollowing: false
                        cursorShape: dragMode === 2 ? Qt.SizeVerCursor : Qt.SizeHorCursor
                        onPressed: mouse => {
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            pressX = point.x;
                            pressY = point.y;
                            dragMode = 0;
                            shiftFollowing = false;
                        }
                        onPositionChanged: mouse => {
                            if (!pressed)
                                return;
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            const deltaX = point.x - pressX;
                            const deltaY = point.y - pressY;
                            if (dragMode === 0 && Math.max(Math.abs(deltaX), Math.abs(deltaY)) >= 3) {
                                dragMode = Math.abs(deltaX) >= Math.abs(deltaY) ? 1 : 2;
                                shiftFollowing = (mouse.modifiers & Qt.ShiftModifier) !== 0;
                            }
                            if (dragMode === 1)
                                root.updatePositionAt(index, point.x, shiftFollowing);
                            else if (dragMode === 2)
                                root.updatePitchAt(index, mapToItem(canvas, mouse.x, mouse.y).y);
                        }
                        onReleased: {
                            if (dragMode === 1) {
                                root.moraDurationsEdited(root.moraDurations.slice());
                                root.moraPositionsEdited(root.moraPositions.slice());
                            } else if (dragMode === 2) {
                                root.pointsEdited(root.points.slice());
                            }
                            dragMode = 0;
                            shiftFollowing = false;
                        }
                        onCanceled: {
                            dragMode = 0;
                            shiftFollowing = false;
                        }
                        onDoubleClicked: root.resetPitchAt(index)
                    }
                }
            }

            Item {
                visible: root.morae.length > 0 && root.durationIsEditable(root.morae.length - 1)
                x: root.sidePadding + root.endTime() * root.durationScale - width / 2
                width: 14
                height: parent.height - 64
                z: 2

                MouseArea {
                    anchors.fill: parent
                    property real pressX: 0
                    property bool dragging: false
                    cursorShape: Qt.SizeHorCursor
                    onPressed: mouse => {
                        pressX = mapToItem(graph, mouse.x, mouse.y).x;
                        dragging = false;
                    }
                    onPositionChanged: mouse => {
                        if (!pressed)
                            return;
                        const point = mapToItem(graph, mouse.x, mouse.y);
                        if (!dragging && Math.abs(point.x - pressX) < 3)
                            return;
                        dragging = true;
                        root.updateEndPositionAt(point.x);
                    }
                    onReleased: {
                        if (dragging) {
                            root.moraDurationsEdited(root.moraDurations.slice());
                            root.moraPositionsEdited(root.moraPositions.slice());
                        }
                        dragging = false;
                    }
                    onCanceled: dragging = false;
                }
            }

            MouseArea {
                anchors.fill: canvas
                property int dragging: -1
                onPressed: mouse => {
                    dragging = root.nearestEditablePoint(mouse.x);
                    if (dragging >= 0)
                        update(mouse.y);
                }
                onPositionChanged: mouse => {
                    if (dragging >= 0)
                        update(mouse.y);
                }
                onReleased: {
                    if (dragging >= 0)
                        root.pointsEdited(root.points.slice());
                    dragging = -1;
                }
                onDoubleClicked: mouse => {
                    const index = root.nearestEditablePoint(mouse.x);
                    if (index < 0)
                        return;
                    const values = root.points.slice();
                    values[index] = 0;
                    root.pitchPointTouched(index);
                    root.points = values;
                    root.pointsEdited(values.slice());
                    dragging = -1;
                }
                onCanceled: dragging = -1

                function update(y) {
                    root.updatePitchAt(dragging, y);
                }
            }
        }

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                viewport.contentX = Math.max(0, Math.min(viewport.contentWidth - viewport.width, viewport.contentX - delta));
                event.accepted = true;
            }
        }
    }

    onPointsChanged: canvas.requestPaint()
    onAutoPointsChanged: canvas.requestPaint()
    onAccentColorChanged: canvas.requestPaint()
    onAxisColorChanged: canvas.requestPaint()
    onGridColorChanged: canvas.requestPaint()
}
