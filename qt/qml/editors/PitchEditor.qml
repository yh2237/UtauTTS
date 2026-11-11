pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import "../controls"
import "../MoraPositions.js" as MoraPositions

Item {
    id: root
    activeFocusOnTab: true
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
    // 選択中の点（インデックス）。複数選択できる。
    property var selectedPoints: []
    // 矩形選択の範囲（グラフ座標）。ドラッグ中だけ値が入る。
    property var selectionBand: null
    // 縦方向の倍率。1 で全体を表示する。
    property real verticalZoom: 1
    readonly property real minimumVerticalZoom: 1
    readonly property real maximumVerticalZoom: 8
    // ピッチのスナップ。ドラッグ中だけ効く。
    property bool snapEnabled: true
    readonly property int snapCents: 10
    readonly property int keyboardStepCents: 10
    readonly property int fineStepCents: 1
    // 縦の表示範囲と編集できる範囲（±セント）。
    readonly property real pitchRange: 800
    readonly property real maximumPitchCents: 800
    property var morae: []
    property var moraDurations: []
    property var moraPositions: []
    property int defaultMoraDuration: 120
    property int defaultPauseDuration: 180
    property int minimumMoraDuration: 20
    property int maximumMoraDuration: 1000
    property int maximumPauseDuration: 3000
    property color accentColor: Theme.selection
    property color axisColor: Theme.axisLine
    property color labelColor: Theme.viewText
    property real moraWidth: 64
    Behavior on moraWidth {
        // ピンチ中は指の動きに直接追従させる。
        enabled: !pinch.active
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

    function editablePointIndices() {
        const values = [];
        for (let index = 0; index < root.points.length; ++index) {
            if (root.pointIsEditable(index))
                values.push(index);
        }
        return values;
    }

    function isPointSelected(index) {
        return root.selectedPoints.indexOf(index) >= 0;
    }

    function setSelectedPoints(values) {
        const unique = [];
        for (const value of values) {
            const index = Number(value);
            if (index >= 0 && index < root.points.length && root.pointIsEditable(index)
                    && unique.indexOf(index) < 0)
                unique.push(index);
        }
        unique.sort((a, b) => a - b);
        if (unique.length === root.selectedPoints.length
                && unique.every((value, position) => value === root.selectedPoints[position]))
            return;
        root.selectedPoints = unique;
        canvas.requestPaint();
    }

    function selectPoint(index, additive) {
        if (!root.pointIsEditable(index))
            return;
        if (!additive) {
            root.setSelectedPoints([index]);
            return;
        }
        const values = root.selectedPoints.slice();
        const position = values.indexOf(index);
        if (position >= 0)
            values.splice(position, 1);
        else
            values.push(index);
        root.setSelectedPoints(values);
    }

    function clearSelection() {
        root.setSelectedPoints([]);
    }

    function selectAllPoints() {
        root.setSelectedPoints(root.editablePointIndices());
    }

    // stepSelectionは、選択を隣の点へ移す。端まで来たら false を返してフォーカス移動に譲る。
    function stepSelection(direction) {
        const indices = root.editablePointIndices();
        if (!indices.length)
            return false;
        const current = root.selectedPoints.length
                ? root.selectedPoints[root.selectedPoints.length - 1] : -1;
        let position = indices.indexOf(current);
        if (position < 0)
            position = direction > 0 ? -1 : indices.length;
        const next = position + direction;
        if (next < 0 || next >= indices.length)
            return false;
        root.setSelectedPoints([indices[next]]);
        return true;
    }

    // extendSelectionは、Shift+矢印で選択範囲を広げる・縮める。
    function extendSelection(direction) {
        const indices = root.editablePointIndices();
        if (!indices.length)
            return false;
        if (direction < 0) {
            if (!root.selectedPoints.length)
                return false;
            const values = root.selectedPoints.slice();
            values.pop();
            root.setSelectedPoints(values);
            return true;
        }
        const last = root.selectedPoints.length
                ? root.selectedPoints[root.selectedPoints.length - 1] : -1;
        const next = indices.indexOf(last) + 1;
        if (next >= indices.length)
            return false;
        const values = root.selectedPoints.slice();
        values.push(indices[next]);
        root.setSelectedPoints(values);
        return true;
    }

    function pointsInBand(band) {
        const values = [];
        if (!band)
            return values;
        const left = Math.min(band.x0, band.x1);
        const right = Math.max(band.x0, band.x1);
        const top = Math.min(band.y0, band.y1);
        const bottom = Math.max(band.y0, band.y1);
        for (let index = 0; index < root.points.length; ++index) {
            if (!root.pointIsEditable(index))
                continue;
            const x = root.pointX(index);
            const y = root.pointY(index);
            if (x >= left && x <= right && y >= top && y <= bottom)
                values.push(index);
        }
        return values;
    }

    function snapPitch(cents) {
        if (!root.snapEnabled)
            return cents;
        return Math.round(cents / root.snapCents) * root.snapCents;
    }

    function zoomVertical(factor) {
        const next = Math.max(root.minimumVerticalZoom,
                              Math.min(root.maximumVerticalZoom, root.verticalZoom * factor));
        if (next === root.verticalZoom)
            return;
        root.verticalZoom = next;
        canvas.requestPaint();
    }

    // 横方向の倍率を変え、指の下の時刻が動かないようにスクロール位置を合わせる。
    function zoomHorizontal(factor, anchorX, anchorTime) {
        const next = Math.max(32, Math.min(192, root.moraWidth * factor));
        if (next === root.moraWidth)
            return;
        root.moraWidth = next;
        // 倍率は Behavior で補間されるため、新しい倍率から直接計算する。
        const scale = root.defaultMoraDuration > 0 ? next / root.defaultMoraDuration : 0.5;
        const padding = next / 2 + 4;
        const contentWidth = Math.max(root.width, padding * 2 + root.totalDuration() * scale);
        const maximum = Math.max(0, contentWidth - viewport.width);
        viewport.contentX = Math.max(0, Math.min(maximum, padding + anchorTime * scale - anchorX));
    }

    function timeAtViewportX(x) {
        return Math.max(0, (viewport.contentX + x - root.sidePadding) / root.durationScale);
    }

    function tr(key, ...args) {
        if (!root.translator || !root.translator.tr)
            return key;
        return root.translator.tr(key, ...args);
    }

    function pointX(index) {
        return root.sidePadding + root.positionAt(index) * root.durationScale;
    }

    function columnWidth(index) {
        let width = root.moraWidth;
        if (index > 0)
            width = Math.min(width, root.pointX(index) - root.pointX(index - 1));
        if (index + 1 < root.morae.length)
            width = Math.min(width, root.pointX(index + 1) - root.pointX(index));
        return Math.max(0, width);
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

    function updatePitchAt(index, y, snap) {
        if (index < 0 || index >= root.points.length || !root.pointIsEditable(index))
            return;
        const center = canvas.height / 2;
        const desired = Math.max(-root.maximumPitchCents, Math.min(root.maximumPitchCents,
                (center - y) / root.pitchScale(canvas.height)));
        const automatic = index < root.autoPoints.length ? Number(root.autoPoints[index]) : 0;
        const target = Math.round((snap === false ? desired : root.snapPitch(desired))
                - (Number.isFinite(automatic) ? automatic : 0));
        root.moveSelectedPoints(index, target);
    }

    // pitchMoveRangeは、選択中の点が編集範囲を出ない移動量を返す。
    function pitchMoveRange(indices, values) {
        let lower = Number.NEGATIVE_INFINITY;
        let upper = Number.POSITIVE_INFINITY;
        for (const position of indices) {
            const automatic = position < root.autoPoints.length
                    ? Number(root.autoPoints[position]) : 0;
            const base = Number.isFinite(automatic) ? automatic : 0;
            const value = Number(values[position]) || 0;
            lower = Math.max(lower, -root.maximumPitchCents - base - value);
            upper = Math.min(upper, root.maximumPitchCents - base - value);
        }
        return {lower: lower, upper: upper};
    }

    // moveSelectedPointsは、選択中の点を同じ量だけ動かす。
    function moveSelectedPoints(index, target) {
        const values = root.points.slice();
        const current = Number(values[index]);
        const delta = Number(target) - (Number.isFinite(current) ? current : 0);
        if (!Number.isFinite(delta))
            return;
        const indices = root.isPointSelected(index) ? root.selectedPoints.slice() : [index];
        const range = root.pitchMoveRange(indices, values);
        const moved = Math.max(range.lower, Math.min(range.upper, delta));
        if (Math.abs(moved) < 1e-9)
            return;
        for (const position of indices)
            values[position] = (Number(values[position]) || 0) + moved;
        root.pitchPointTouched(index);
        root.points = values;
        canvas.requestPaint();
    }

    function nudgeSelectedPitch(deltaCents) {
        const indices = root.selectedPoints.slice();
        if (!indices.length)
            return;
        const values = root.points.slice();
        const range = root.pitchMoveRange(indices, values);
        const moved = Math.max(range.lower, Math.min(range.upper, deltaCents));
        if (Math.abs(moved) < 1e-9)
            return;
        for (const position of indices)
            values[position] = (Number(values[position]) || 0) + moved;
        root.pitchPointTouched(indices[0]);
        root.points = values;
        root.pointsEdited(values.slice());
        canvas.requestPaint();
    }

    // resetPointsは、指定した点を自動の値に戻す。
    function resetPoints(indices) {
        const values = root.points.slice();
        let changed = false;
        for (const position of indices) {
            if (!root.pointIsEditable(position))
                continue;
            if (Number(values[position]) !== 0) {
                values[position] = 0;
                changed = true;
            }
        }
        if (!changed)
            return;
        root.pitchPointTouched(indices[0]);
        root.points = values;
        root.pointsEdited(values.slice());
        canvas.requestPaint();
    }

    function resetPitchAt(index) {
        if (!root.pointIsEditable(index))
            return;
        root.resetPoints([index]);
    }

    function resetSelectedPoints() {
        root.resetPoints(root.selectedPoints.slice());
    }

    function pitchScale(height) {
        return Math.max(.05, Math.min(.36, height / 760)) * 300 / root.pitchRange
                * root.verticalZoom;
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
                objectName: "pitchCanvas"
                x: viewport.contentX
                width: viewport.width
                height: Math.max(0, viewport.height - 64)
                onWidthChanged: requestPaint()
                onHeightChanged: requestPaint()
                onPaint: {
                    const ctx = getContext("2d");
                    ctx.reset();
                    ctx.clearRect(0, 0, width, height);
                    ctx.translate(-x, 0);
                    const center = height / 2;
                    const scale = root.pitchScale(height);
                    const viewLeft = viewport.contentX - 40;
                    const viewRight = viewport.contentX + viewport.width + 40;
                    const inView = x => x >= viewLeft && x <= viewRight;
                    ctx.strokeStyle = root.axisColor;
                    ctx.beginPath();
                    ctx.moveTo(Math.max(0, viewLeft), center);
                    ctx.lineTo(Math.min(graph.width, viewRight), center);
                    ctx.stroke();
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
                        const py = center - root.pitchAt(index) * scale;
                        const hot = index === root.hoveredPoint;
                        ctx.beginPath();
                        ctx.arc(px, py, hot ? 9 : 6, 0, Math.PI * 2);
                        ctx.fill();
                        if (hot) {
                            ctx.globalAlpha = 0.25;
                            ctx.beginPath();
                            ctx.arc(px, py, 13, 0, Math.PI * 2);
                            ctx.fill();
                            ctx.globalAlpha = 1;
                        }
                        if (root.isPointSelected(index)) {
                            ctx.strokeStyle = root.labelColor;
                            ctx.lineWidth = 2;
                            ctx.beginPath();
                            ctx.arc(px, py, 10, 0, Math.PI * 2);
                            ctx.stroke();
                            ctx.lineWidth = 1;
                        }
                    }
                    if (root.selectionBand) {
                        const band = root.selectionBand;
                        const left = Math.min(band.x0, band.x1);
                        const top = Math.min(band.y0, band.y1);
                        ctx.fillStyle = Qt.rgba(root.accentColor.r, root.accentColor.g,
                                                root.accentColor.b, 0.12);
                        ctx.fillRect(left, top, Math.abs(band.x1 - band.x0),
                                     Math.abs(band.y1 - band.y0));
                        ctx.strokeStyle = root.accentColor;
                        ctx.lineWidth = 1;
                        ctx.strokeRect(left, top, Math.abs(band.x1 - band.x0),
                                       Math.abs(band.y1 - band.y0));
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
                property real pressX: 0
                property real pressY: 0
                property bool moved: false
                property bool banding: false
                property bool additive: false
                onPressed: mouse => {
                    root.forceActiveFocus();
                    startContentX = viewport.contentX;
                    startFingerX = mapToItem(viewport, mouse.x, mouse.y).x;
                    const point = mapToItem(graph, mouse.x, mouse.y);
                    pressX = point.x;
                    pressY = point.y;
                    moved = false;
                    banding = false;
                    additive = (mouse.modifiers & Qt.ShiftModifier) !== 0;
                }
                onPositionChanged: mouse => {
                    if (!pressed)
                        return;
                    const point = mapToItem(graph, mouse.x, mouse.y);
                    if (Math.max(Math.abs(point.x - pressX), Math.abs(point.y - pressY)) >= 3)
                        moved = true;
                    // Shift+ドラッグは矩形選択、それ以外は横スクロール。
                    if (additive) {
                        if (!moved)
                            return;
                        banding = true;
                        root.selectionBand = {x0: pressX, y0: pressY,
                                              x1: point.x, y1: point.y};
                        canvas.requestPaint();
                        return;
                    }
                    const maximum = Math.max(0, viewport.contentWidth - viewport.width);
                    if (maximum <= 0)
                        return;
                    const fingerX = mapToItem(viewport, mouse.x, mouse.y).x;
                    viewport.contentX = Math.max(0, Math.min(maximum,
                            startContentX - (fingerX - startFingerX)));
                }
                onReleased: {
                    if (banding) {
                        const values = root.selectedPoints.slice();
                        for (const index of root.pointsInBand(root.selectionBand)) {
                            if (values.indexOf(index) < 0)
                                values.push(index);
                        }
                        root.setSelectedPoints(values);
                    } else if (!additive && !moved) {
                        root.clearSelection();
                    }
                    banding = false;
                    root.selectionBand = null;
                    canvas.requestPaint();
                }
                onCanceled: {
                    banding = false;
                    root.selectionBand = null;
                    canvas.requestPaint();
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
                        width: root.columnWidth(pointColumn.index)
                        height: parent.height
                        x: root.pointX(pointColumn.index) - width / 2
                        spacing: 1
                        clip: true

                        Text {
                            width: parent.width
                            text: pointColumn.modelData.mora || root.translator.tr("pitch.emptyMora")
                            horizontalAlignment: Text.AlignHCenter
                            color: root.labelColor
                            elide: Text.ElideRight
                        }
                        TextInput {
                            width: Math.max(0, parent.width - 8)
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
                            elide: Text.ElideRight
                            color: root.labelColor
                            opacity: 0.75
                            font.pixelSize: Theme.fontXSmall

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
                            const canvasPoint = mapToItem(graph, mouse.x, mouse.y);
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
                                const canvasPoint = mapToItem(graph, mouse.x, mouse.y);
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
                width: graph.width
                height: canvas.height
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
                        property real pressX: 0
                        property real pressY: 0
                        property bool dragging: false
                        property bool additive: false
                        property bool wasSelected: false

                        onEntered: {
                            root.hoveredPoint = index;
                            canvas.requestPaint();
                        }
                        onExited: {
                            if (!pressed && root.hoveredPoint === index)
                                root.hoveredPoint = -1;
                            canvas.requestPaint();
                        }
                        // 押下では選択だけ行い、値は動かさない。
                        onPressed: mouse => {
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            pressX = point.x;
                            pressY = point.y;
                            dragging = false;
                            additive = (mouse.modifiers & Qt.ShiftModifier) !== 0;
                            wasSelected = root.isPointSelected(index);
                            root.forceActiveFocus();
                            if (additive)
                                root.selectPoint(index, true);
                            else if (!wasSelected)
                                root.selectPoint(index, false);
                            root.hoveredPoint = index;
                            canvas.requestPaint();
                        }
                        onPositionChanged: mouse => {
                            if (!pressed)
                                return;
                            const point = mapToItem(graph, mouse.x, mouse.y);
                            if (!dragging) {
                                if (Math.max(Math.abs(point.x - pressX),
                                             Math.abs(point.y - pressY)) < 3)
                                    return;
                                dragging = true;
                            }
                            root.hoveredPoint = index;
                            root.updatePitchAt(index, point.y,
                                    (mouse.modifiers & Qt.AltModifier) === 0);
                            root.showHud(point.x + 14, point.y - 40, root.pitchTextAt(index));
                        }
                        onReleased: {
                            if (dragging) {
                                root.pointsEdited(root.points.slice());
                                root.hideHud();
                            } else if (!additive && wasSelected) {
                                root.selectPoint(index, false);
                            }
                            dragging = false;
                            if (!containsMouse)
                                root.hoveredPoint = -1;
                            canvas.requestPaint();
                        }
                        onDoubleClicked: root.resetPitchAt(index)
                        onCanceled: {
                            dragging = false;
                            root.hideHud();
                            root.hoveredPoint = -1;
                            canvas.requestPaint();
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

        // 2本指のピンチで横方向に拡大・縮小する。1本指はそのままスクロールになる。
        PinchHandler {
            id: pinch
            property real lastScale: 1
            property real anchorX: 0
            property real anchorTime: 0
            onActiveChanged: {
                if (!active)
                    return;
                lastScale = 1;
                // centroid は contentItem の座標なので、表示位置に直す。
                anchorX = viewport.mapFromItem(null, centroid.scenePosition).x;
                anchorTime = root.timeAtViewportX(anchorX);
            }
            onScaleChanged: {
                if (!active || scale <= 0 || lastScale <= 0)
                    return;
                const factor = scale / lastScale;
                lastScale = scale;
                root.zoomHorizontal(factor, anchorX, anchorTime);
            }
        }

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                if ((event.modifiers & Qt.ControlModifier) !== 0) {
                    const factor = event.angleDelta.y > 0 ? 1.15 : 1 / 1.15;
                    root.moraWidth = Math.max(32, Math.min(192, root.moraWidth * factor));
                } else if ((event.modifiers & Qt.ShiftModifier) !== 0) {
                    const delta = event.angleDelta.y !== 0 ? event.angleDelta.y
                                                           : event.angleDelta.x;
                    if (delta !== 0)
                        root.zoomVertical(delta > 0 ? 1.25 : 1 / 1.25);
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

    // 文字入力中はキー操作を奪わない。
    readonly property bool keyboardActive: {
        if (!root.activeFocus)
            return false;
        let item = Window.activeFocusItem;
        while (item) {
            if (item instanceof TextInput || item instanceof TextEdit)
                return false;
            item = item.parent;
        }
        return true;
    }

    // Delete は発話削除のショートカットと競合する。点を選んでいるときだけ編集側が受け取る。
    Keys.onShortcutOverride: event => {
        if (root.keyboardActive && root.selectedPoints.length > 0
                && (event.key === Qt.Key_Delete || event.key === Qt.Key_Backspace))
            event.accepted = true;
    }

    Keys.onPressed: event => {
        if (!root.keyboardActive)
            return;
        const shift = (event.modifiers & Qt.ShiftModifier) !== 0;
        const control = (event.modifiers & Qt.ControlModifier) !== 0;
        switch (event.key) {
        case Qt.Key_Left:
            if (shift ? root.extendSelection(-1) : root.stepSelection(-1))
                event.accepted = true;
            return;
        case Qt.Key_Right:
            if (shift ? root.extendSelection(1) : root.stepSelection(1))
                event.accepted = true;
            return;
        case Qt.Key_Up:
            root.nudgeSelectedPitch(shift ? root.fineStepCents : root.keyboardStepCents);
            event.accepted = true;
            return;
        case Qt.Key_Down:
            root.nudgeSelectedPitch(shift ? -root.fineStepCents : -root.keyboardStepCents);
            event.accepted = true;
            return;
        case Qt.Key_Delete:
        case Qt.Key_Backspace:
            root.resetSelectedPoints();
            event.accepted = true;
            return;
        case Qt.Key_A:
            if (control) {
                root.selectAllPoints();
                event.accepted = true;
            }
            return;
        case Qt.Key_Escape:
            root.clearSelection();
            event.accepted = true;
            return;
        case Qt.Key_S:
            if (!control) {
                root.snapEnabled = !root.snapEnabled;
                event.accepted = true;
            }
            return;
        case Qt.Key_Plus:
        case Qt.Key_Equal:
            root.zoomVertical(1.25);
            event.accepted = true;
            return;
        case Qt.Key_Minus:
            root.zoomVertical(1 / 1.25);
            event.accepted = true;
            return;
        case Qt.Key_0:
            root.verticalZoom = 1;
            event.accepted = true;
            return;
        }
    }

    onPointsChanged: {
        // 点が減ったら選択から外す。
        if (root.selectedPoints.length)
            root.setSelectedPoints(root.selectedPoints);
        canvas.requestPaint();
    }
    onAutoPointsChanged: canvas.requestPaint()
    onAccentColorChanged: canvas.requestPaint()
    onAxisColorChanged: canvas.requestPaint()
    onMoraeChanged: {
        if (root.selectedPoints.length)
            root.setSelectedPoints(root.selectedPoints);
        canvas.requestPaint();
    }
    onMoraDurationsChanged: canvas.requestPaint()
    onMoraPositionsChanged: canvas.requestPaint()
    onMoraWidthChanged: canvas.requestPaint()
    onDefaultMoraDurationChanged: canvas.requestPaint()
    onDefaultPauseDurationChanged: canvas.requestPaint()
    onSelectedPointsChanged: canvas.requestPaint()
    onVerticalZoomChanged: canvas.requestPaint()
    Connections {
        target: Theme
        function onDarkChanged() { canvas.requestPaint(); }
    }
}
