pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import "../controls"
import "../MoraPositions.js" as MoraPositions

Item {
    id: root
    property var translator: ({})
    property string hudText: ""
    property real hudX: 0
    property real hudY: 0
    property int hoveredStrip: -1
    property int hoveredPoint: -1
    property bool hoveredFollowing: false
    property bool hoveredEnd: false
    property var points: []
    property var autoPoints: []
    property real pitchRange: 300
    property bool pitchDragging: false
    readonly property real maximumPitchCents: 1200
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
    Behavior on moraWidth {
        NumberAnimation { duration: 90; easing.type: Easing.OutCubic }
    }
    property real sidePadding: moraWidth / 2 + 4
    signal pointsEdited(var points)
    signal pitchPointTouched(int index)
    signal timingEdited(var durations, var positions)
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

    function normalizedPositions(values) {
        return MoraPositions.normalizedMoraPositions(values);
    }

    function positionAt(index) {
        if (index === 0 && root.morae.length)
            return 0;
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
        if (index < 0 || index >= root.points.length || index >= root.morae.length)
            return false;
        return !root.morae[index].pause;
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
        if (index === 0)
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
            const following = index + 1 < count ? positions[index + 1] : root.endTime();
            const lower = Math.max(index > 0 ? positions[index - 1] + previousMinimum : 0,
                                   following - currentMaximum);
            const upper = Math.min(index > 0 ? positions[index - 1] + previousMaximum
                                             : Number.POSITIVE_INFINITY,
                                   following - currentMinimum);
            positions[index] = Math.max(lower, Math.min(upper, cursor));
        }
        // 文頭休止の設定はないため、先頭モーラは0に保つ。
        root.moraPositions = root.normalizedPositions(positions);
        root.moraDurations = root.durationValuesFromPositions();
        canvas.requestPaint();
    }

    function setPositionAtMS(index, value, moveFollowing, preview) {
        if (!Number.isFinite(Number(value)) || !root.durationIsEditable(index))
            return;
        root.updatePositionAt(index,
                root.sidePadding + Math.max(0, Number(value)) * root.durationScale,
                moveFollowing === true);
        if (preview === true) {
            canvas.requestPaint();
            return;
        }
        root.timingEdited(root.moraDurations.slice(), root.moraPositions.slice());
        canvas.requestPaint();
    }

    function setDurationAtMS(index, value, preview) {
        if (!Number.isFinite(Number(value)) || !root.durationIsEditable(index))
            return;
        const count = root.morae.length;
        const positions = (root.moraPositions || []).slice();
        for (let position = 0; position < count; ++position)
            positions[position] = root.positionAt(position);
        const target = Math.max(root.minimumDurationAt(index),
                                Math.min(root.maximumDurationAt(index), Number(value)));
        const current = root.durationFromPositions(index);
        const delta = target - current;
        if (index + 1 < count) {
            for (let position = index + 1; position < count; ++position)
                positions[position] += delta;
        }
        root.moraPositions = root.normalizedPositions(positions);
        if (index === count - 1) {
            const durations = root.moraDurations.slice();
            durations[index] = Math.round(target);
            root.moraDurations = durations;
        } else {
            root.moraDurations = root.durationValuesFromPositions();
        }
        if (preview === true) {
            canvas.requestPaint();
            return;
        }
        root.timingEdited(root.moraDurations.slice(), root.moraPositions.slice());
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
        root.moraPositions = root.normalizedPositions(positions);
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
        root.timingEdited(root.moraDurations.slice(), root.moraPositions.slice());
        canvas.requestPaint();
    }

    function updatePitchAt(index, y) {
        if (index < 0 || index >= root.points.length || !root.pointIsEditable(index))
            return;
        const values = root.points.slice();
        const center = canvas.height / 2;
        const desired = Math.max(-root.maximumPitchCents, Math.min(root.maximumPitchCents,
                (center - y) / root.pitchScale(canvas.height)));
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

    function refitPitchRange() {
        if (root.pitchDragging)
            return;
        let peak = 0;
        for (let index = 0; index < root.points.length; ++index) {
            if (root.pointIsEditable(index))
                peak = Math.max(peak, Math.abs(root.pitchAt(index)));
        }
        root.pitchRange = Math.max(300, Math.min(root.maximumPitchCents, Math.ceil(peak * 1.15 / 100) * 100));
    }

    function pitchScale(height) {
        return Math.max(.05, Math.min(.36, height / 760)) * 300 / root.pitchRange;
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

    function pitchTextAt(index) {
        const value = Math.round(root.pitchAt(index));
        return (value >= 0 ? "+" : "") + value + " cent";
    }

    function timingTextAt(index) {
        return Math.round(root.positionAt(index)) + " ms / "
                + Math.round(root.durationAt(index)) + " ms";
    }

    function showHud(graphX, graphY, text) {
        root.hudText = text;
        root.hudX = graphX;
        root.hudY = graphY;
    }

    function hideHud() {
        root.hudText = "";
    }

    function pointY(index) {
        return canvas.height / 2 - root.pitchAt(index) * root.pitchScale(canvas.height);
    }

    function grabbablePoint(x, y) {
        const best = root.nearestEditablePoint(x);
        if (best < 0)
            return -1;
        if (Math.abs(x - root.pointX(best)) > 16)
            return -1;
        if (Math.abs(y - root.pointY(best)) > 20)
            return -1;
        return best;
    }

    function hoverPointForStrip(index, canvasX, canvasY) {
        if (!root.pointIsEditable(index))
            return -1;
        return root.grabbablePoint(canvasX, canvasY) === index ? index : -1;
    }

    function setShiftPreview(down) {
        if (root.hudText.length > 0 || root.hoveredStrip < 0)
            return;
        root.hoveredFollowing = down;
        canvas.requestPaint();
    }

    function syncHoverModifiers(mods) {
        const shift = (mods & Qt.ShiftModifier) !== 0;
        if (shift !== root.hoveredFollowing) {
            root.hoveredFollowing = shift;
            canvas.requestPaint();
        }
    }

    Flickable {
        id: viewport
        anchors.fill: parent
        clip: true
        contentWidth: root.graphWidth
        contentHeight: height
        boundsBehavior: Flickable.StopAtBounds
        interactive: false
        onContentXChanged: canvas.requestPaint()

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
                    const scale = root.pitchScale(height);
                    const viewLeft = viewport.contentX - 40;
                    const viewRight = viewport.contentX + viewport.width + 40;
                    const inView = x => x >= viewLeft && x <= viewRight;
                    const gridLimit = Math.floor(root.pitchRange / 300) * 300;
                    for (let cents = -gridLimit; cents <= gridLimit; cents += 300) {
                        ctx.strokeStyle = cents === 0 ? root.axisColor : root.gridColor;
                        ctx.setLineDash(cents === 0 ? [] : [4, 5]);
                        ctx.beginPath();
                        ctx.moveTo(Math.max(0, viewLeft), center - cents * scale);
                        ctx.lineTo(Math.min(width, viewRight), center - cents * scale);
                        ctx.stroke();
                    }
                    ctx.setLineDash([]);
                    ctx.strokeStyle = root.axisColor;
                    ctx.globalAlpha = 0.45;
                    ctx.lineWidth = 1;
                    for (let index = 1; index < root.morae.length; ++index) {
                        if (!root.durationIsEditable(index))
                            continue;
                        const hot = index === root.hoveredStrip
                                || (root.hoveredFollowing && index > root.hoveredStrip);
                        ctx.strokeStyle = hot ? root.accentColor : root.axisColor;
                        ctx.globalAlpha = hot ? 0.9 : 0.45;
                        ctx.lineWidth = hot ? 2 : 1;
                        const x = root.pointX(index);
                        if (!inView(x))
                            continue;
                        ctx.beginPath();
                        ctx.moveTo(x, 0);
                        ctx.lineTo(x, height);
                        ctx.stroke();
                    }
                    if (root.durationIsEditable(root.morae.length - 1)) {
                        const endX = root.sidePadding + root.endTime() * root.durationScale;
                        if (inView(endX)) {
                            ctx.strokeStyle = root.hoveredEnd ? root.accentColor : root.axisColor;
                            ctx.lineWidth = root.hoveredEnd ? 2 : 1;
                            ctx.beginPath();
                            ctx.moveTo(endX, 0);
                            ctx.lineTo(endX, height);
                            ctx.stroke();
                            ctx.lineWidth = 1;
                        }
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
                        if (!root.pointIsEditable(index)) {
                            started = false;
                            continue;
                        }
                        const x = root.pointX(index);
                        if (!inView(x)) {
                            started = false;
                            continue;
                        }
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
                        const px = root.pointX(index);
                        if (!inView(px))
                            continue;
                        const hot = index === root.hoveredPoint;
                        ctx.beginPath();
                        ctx.arc(px, center - root.pitchAt(index) * scale,
                                hot ? 9 : 6, 0, Math.PI * 2);
                        ctx.fill();
                        if (hot) {
                            ctx.globalAlpha = 0.25;
                            ctx.beginPath();
                            ctx.arc(px, center - root.pitchAt(index) * scale,
                                    13, 0, Math.PI * 2);
                            ctx.fill();
                            ctx.globalAlpha = 1;
                        }
                    }
                }
            }

            // 編集を優先するため、背景操作はハンドルより下に置く。
            MouseArea {
                id: backgroundPan
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 64
                z: 1
                cursorShape: Qt.OpenHandCursor
                property real startContentX: 0
                property real startFingerX: 0
                onPressed: mouse => {
                    startContentX = viewport.contentX;
                    startFingerX = mapToItem(viewport, mouse.x, mouse.y).x;
                }
                onPositionChanged: mouse => {
                    if (!pressed)
                        return;
                    const maximum = Math.max(0, viewport.contentWidth - viewport.width);
                    if (maximum <= 0)
                        return;
                    const fingerX = mapToItem(viewport, mouse.x, mouse.y).x;
                    viewport.contentX = Math.max(0, Math.min(maximum,
                            startContentX - (fingerX - startFingerX)));
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
                                bottom: -root.maximumPitchCents
                                top: root.maximumPitchCents
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
                                        Math.max(-root.maximumPitchCents, Math.min(root.maximumPitchCents, parsed))
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
                    visible: index > 0 && root.durationIsEditable(index)
                    x: root.pointX(index) - width / 2
                    width: 14
                    height: parent.height - 64
                    z: 2

                    MouseArea {
                        id: stripArea
                        anchors.fill: parent
                        property real pressX: 0
                        property bool dragging: false
                        property bool shiftFollowing: false
                        cursorShape: Qt.SizeHorCursor
                        hoverEnabled: true
                        onContainsMouseChanged: {
                            if (!containsMouse) {
                                if (root.hoveredStrip === index)
                                    root.hoveredStrip = -1;
                                root.hoveredFollowing = false;
                                if (root.hoveredPoint === index)
                                    root.hoveredPoint = -1;
                                canvas.requestPaint();
                            }
                        }
                        onPressed: mouse => {
                            const canvasPoint = mapToItem(canvas, mouse.x, mouse.y);
                            if (root.hoverPointForStrip(index, canvasPoint.x, canvasPoint.y) === index) {
                                mouse.accepted = false;
                                return;
                            }
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            pressX = point.x;
                            dragging = false;
                            shiftFollowing = false;
                            root.hoveredStrip = index;
                            root.syncHoverModifiers(mouse.modifiers);
                            canvas.requestPaint();
                        }
                        onPositionChanged: mouse => {
                            if (!pressed) {
                                root.hoveredStrip = index;
                                root.syncHoverModifiers(mouse.modifiers);
                                const canvasPoint = mapToItem(canvas, mouse.x, mouse.y);
                                root.hoveredPoint = root.hoverPointForStrip(
                                        index, canvasPoint.x, canvasPoint.y);
                                canvas.requestPaint();
                                return;
                            }
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            if (!dragging && Math.abs(point.x - pressX) < 3)
                                return;
                            if (!dragging) {
                                dragging = true;
                                shiftFollowing = (mouse.modifiers & Qt.ShiftModifier) !== 0;
                                root.hoveredFollowing = shiftFollowing;
                            }
                            root.updatePositionAt(index, point.x, shiftFollowing);
                            root.showHud(point.x + 14, point.y - 40,
                                    root.timingTextAt(index)
                                    + (shiftFollowing ? "  (Shift)" : ""));
                        }
                        onReleased: mouse => {
                            if (dragging) {
                                root.timingEdited(root.moraDurations.slice(),
                                                  root.moraPositions.slice());
                            }
                            dragging = false;
                            shiftFollowing = false;
                            root.hideHud();
                            root.syncHoverModifiers(mouse.modifiers);
                        }
                        onCanceled: {
                            dragging = false;
                            shiftFollowing = false;
                            root.hideHud();
                        }
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
                    id: endArea
                    anchors.fill: parent
                    property real pressX: 0
                    property bool dragging: false
                    cursorShape: Qt.SizeHorCursor
                    hoverEnabled: true
                    onContainsMouseChanged: {
                        root.hoveredEnd = containsMouse;
                        canvas.requestPaint();
                    }
                    onPressed: mouse => {
                        pressX = mapToItem(graph, mouse.x, mouse.y).x;
                        dragging = false;
                        root.hoveredEnd = true;
                        canvas.requestPaint();
                    }
                    onPositionChanged: mouse => {
                        if (!pressed)
                            return;
                        const point = mapToItem(graph, mouse.x, mouse.y);
                        if (!dragging && Math.abs(point.x - pressX) < 3)
                            return;
                        dragging = true;
                        root.updateEndPositionAt(point.x);
                        root.showHud(point.x + 14, point.y - 40,
                                Math.round(root.durationAt(root.morae.length - 1)) + " ms");
                    }
                    onReleased: {
                        if (dragging) {
                            root.timingEdited(root.moraDurations.slice(),
                                              root.moraPositions.slice());
                        }
                        dragging = false;
                        root.hideHud();
                        if (!containsMouse) {
                            root.hoveredEnd = false;
                            canvas.requestPaint();
                        }
                    }
                    onCanceled: {
                        dragging = false;
                        root.hideHud();
                        root.hoveredEnd = false;
                        canvas.requestPaint();
                    }
                    onDoubleClicked: root.resetDurationAt(root.morae.length - 1)
                }
            }

            Item {
                anchors.fill: canvas
                // 点の周囲だけ前面に置き、線と背景の操作を遮らない。
                z: 3

                Repeater {
                    model: root.morae
                    delegate: MouseArea {
                        required property int index
                        visible: root.pointIsEditable(index)
                        x: root.pointX(index) - width / 2
                        y: root.pointY(index) - height / 2
                        width: 32
                        height: 40
                        hoverEnabled: true
                        preventStealing: true
                        cursorShape: Qt.SizeVerCursor

                        onEntered: {
                            root.hoveredPoint = index;
                            canvas.requestPaint();
                        }
                        onExited: {
                            if (!pressed && root.hoveredPoint === index)
                                root.hoveredPoint = -1;
                            canvas.requestPaint();
                        }
                        onPressed: mouse => {
                            root.pitchDragging = true;
                            updatePitch(mouse);
                        }
                        onPositionChanged: mouse => {
                            if (pressed)
                                updatePitch(mouse);
                        }
                        onReleased: {
                            root.pitchDragging = false;
                            root.pointsEdited(root.points.slice());
                            root.hideHud();
                            if (!containsMouse)
                                root.hoveredPoint = -1;
                            canvas.requestPaint();
                        }
                        onDoubleClicked: root.resetPitchAt(index)
                        onCanceled: {
                            root.pitchDragging = false;
                            root.hideHud();
                            root.hoveredPoint = -1;
                            canvas.requestPaint();
                        }

                        function updatePitch(mouse) {
                            const point = mapToItem(canvas, mouse.x, mouse.y);
                            root.hoveredPoint = index;
                            root.updatePitchAt(index, point.y);
                            root.showHud(point.x + 14, point.y - 40, root.pitchTextAt(index));
                        }
                    }
                }
            }

            DragHud {
                x: Math.max(4, Math.min(graph.width - width - 4, root.hudX))
                y: Math.max(4, Math.min(graph.height - height - 4, root.hudY))
                hudText: root.hudText
                z: 10
            }
        }

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                if ((event.modifiers & Qt.ControlModifier) !== 0) {
                    const factor = event.angleDelta.y > 0 ? 1.15 : 1 / 1.15;
                    root.moraWidth = Math.max(32, Math.min(192, root.moraWidth * factor));
                } else {
                    const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                    const maximum = Math.max(0, viewport.contentWidth - viewport.width);
                    scrollAnimation.stop();
                    scrollAnimation.from = viewport.contentX;
                    scrollAnimation.to = Math.max(0, Math.min(maximum, viewport.contentX - delta));
                    scrollAnimation.restart();
                }
                event.accepted = true;
            }
        }

        NumberAnimation {
            id: scrollAnimation
            target: viewport
            property: "contentX"
            duration: 130
            easing.type: Easing.OutCubic
        }
    }

    onPointsChanged: {
        root.refitPitchRange();
        canvas.requestPaint();
    }
    onAutoPointsChanged: {
        root.refitPitchRange();
        canvas.requestPaint();
    }
    onPitchDraggingChanged: root.refitPitchRange()
    onPitchRangeChanged: canvas.requestPaint()
    onAccentColorChanged: canvas.requestPaint()
    onAxisColorChanged: canvas.requestPaint()
    onGridColorChanged: canvas.requestPaint()
    onMoraeChanged: {
        root.refitPitchRange();
        canvas.requestPaint();
    }
    onMoraDurationsChanged: canvas.requestPaint()
    onMoraPositionsChanged: canvas.requestPaint()
    onMoraWidthChanged: canvas.requestPaint()
    onDefaultMoraDurationChanged: canvas.requestPaint()
    onDefaultPauseDurationChanged: canvas.requestPaint()

    function refresh() {
        canvas.requestPaint();
    }
}
