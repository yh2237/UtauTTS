pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root

    property var translator: ({})
    property var units: []
    property var waveformMin: []
    property var waveformMax: []
    property real waveformDuration: 0
    property real leadingMargin: 0
    property var morae: []
    property var moraDurations: []
    property var moraPositions: []
    property var autoFrames: []
    property var manualFrames: []
    property real frameMs: 10
    property bool framePaintMode: false
    property var overrides: []
    property color accentColor: "#d35f6b"
    property color axisColor: "#c79298"
    property color gridColor: "#eadcdf"
    property color labelColor: "#66565a"
    property color mutedText: "#777777"
    property color dividerColor: "#c79298"
    property bool showTimelineFrame: true
    property int selectedUnitIndex: -1
    property real zoomFactor: 1
    property int hoveredBoundary: -1
    property bool hoveredEdgeFollowing: false
    property int hoveredUnit: -1
    property int timelineCursor: Qt.PointingHandCursor
    readonly property real noteLaneTop: 20
    readonly property real noteLaneH: 110
    readonly property real exprTop: 156
    Behavior on zoomFactor {
        NumberAnimation { duration: 120 }
    }
    property bool snapEnabled: true
    property real currentScrollX: 0
    property var timingEditor: null
    property var gesture: null
    property real gestureDuration: 0
    property var gesturePositions: []
    property var gestureDurations: []
    property var previewUnit: null
    property var valueGesture: null
    property int selectedBoundary: -1
    property real playbackMs: -1
    property bool showDetails: false
    property string hudText: ""
    property real hudX: 0
    property real hudY: 0
    signal seekRequested(real positionMs)
    readonly property var otoKeys: ["offset_ms", "cutoff_ms", "consonant_ms", "preutterance_ms", "overlap_ms"]
    readonly property real contentDuration: Math.max(1000, root.waveformDuration,
            root.leadingMargin + (root.timingEditor ? root.timingEditor.totalDuration() : 0) + 200)
    readonly property real pixelsPerMs: root.timingEditor && root.timingEditor.durationScale > 0
            ? root.timingEditor.durationScale : 0.5
    readonly property var exprLanes: [
        {key: "pitch_factor", code: "PIT"},
        {key: "resampler_volume", code: "VOL"}
    ]
    readonly property real exprLaneH: 22
    readonly property real exprAreaH: root.exprLanes.length * root.exprLaneH + 8
    property var hoveredExpr: ({u: -1, l: -1})

    function noteCenterY() {
        return root.noteLaneTop + root.noteLaneH / 2;
    }

    function noteY(position) {
        if (!root.timingEditor)
            return root.noteCenterY();
        return root.noteCenterY() - root.timingEditor.pitchAt(position) * root.noteLaneH / 600;
    }

    readonly property int frameDisplayCount: Math.max(root.autoFrames.length,
            root.manualFrames.length, root.frameCapacity(), 2)

    function frameStepMs() {
        const step = Number(root.frameMs);
        return step > 0 ? step : 10;
    }

    function framePaintRequested(modifiers) {
        return root.framePaintMode || (modifiers & Qt.ControlModifier) !== 0;
    }

    function frameCapacity() {
        return Math.max(2, Math.ceil(root.contentDuration / root.frameStepMs()) + 1);
    }

    function trimmedFrames(values) {
        const result = values.slice();
        while (result.length > 0 && Number(result[result.length - 1]) === 0)
            result.pop();
        return result;
    }

    function frameAutoAt(index) {
        const value = index < root.autoFrames.length ? Number(root.autoFrames[index]) : 0;
        return Number.isFinite(value) ? value : 0;
    }

    function frameManualAt(index) {
        const value = index < root.manualFrames.length ? Number(root.manualFrames[index]) : 0;
        return Number.isFinite(value) ? value : 0;
    }

    function frameTotalAt(index) {
        return root.frameAutoAt(index) + root.frameManualAt(index);
    }

    function frameTimeMs(index) {
        return index * root.frameStepMs();
    }

    function frameY(cents) {
        return root.noteCenterY() - cents * root.noteLaneH / 600;
    }

    function frameAtTime(timeMs) {
        const target = Math.round(timeMs / root.frameStepMs());
        return Math.max(0, Math.min(root.frameDisplayCount - 1, target));
    }

    function frameHasPronunciation(index) {
        if (index < 0 || index >= root.frameDisplayCount)
            return false;
        const time = root.frameTimeMs(index) + root.leadingMargin;
        for (const unit of root.units || []) {
            if (!unit || unit.silent)
                continue;
            const fallbackStart = root.unitNoteStart(unit) + root.leadingMargin;
            const renderStart = Number(unit.render_start_ms);
            const start = Number.isFinite(renderStart) ? renderStart : fallbackStart;
            const end = root.unitNoteStart(unit) + root.leadingMargin
                    + Math.max(1, root.unitDuration(unit));
            if (time >= start && time <= end)
                return true;
        }
        return false;
    }

    function paddedFrameManual() {
        const values = [];
        for (let index = 0; index < root.frameDisplayCount; ++index)
            values.push(root.frameManualAt(index));
        return values;
    }

    function setFrameRange(from, to, cents) {
        const values = root.paddedFrameManual();
        const lo = Math.max(0, Math.min(from, to));
        const hi = Math.min(root.frameDisplayCount - 1, Math.max(from, to));
        for (let index = lo; index <= hi; ++index) {
            if (root.frameHasPronunciation(index))
                values[index] = cents;
        }
        root.manualFrames = values;
        waveformCanvas.requestPaint();
    }

    function paintFrameTo(x, y, fromFrame) {
        const target = root.frameAtTime(root.xToTime(x) - root.leadingMargin);
        if (!root.frameHasPronunciation(target)) {
            root.hudText = "";
            return -1;
        }
        const desired = Math.max(-600, Math.min(600,
                (root.noteCenterY() - y) / root.noteLaneH * 600));
        const values = root.paddedFrameManual();
        const lo = Math.max(0, fromFrame < 0 ? target : Math.min(fromFrame, target));
        const hi = Math.min(root.frameDisplayCount - 1,
                fromFrame < 0 ? target : Math.max(fromFrame, target));
        for (let index = lo; index <= hi; ++index) {
            if (root.frameHasPronunciation(index))
                values[index] = Math.round((desired - root.frameAutoAt(index)) * 10) / 10;
        }
        root.manualFrames = values;
        const total = root.frameTotalAt(target);
        root.updateHud(x, y, (total >= 0 ? "+" : "") + Math.round(total) + " cent");
        waveformCanvas.requestPaint();
        return target;
    }

    function exprLaneY(lane) {
        return root.exprTop + 6 + lane * root.exprLaneH;
    }

    signal unitValueEdited(int unitIndex, string key, var value)
    signal framesEdited(var frames)
    signal moraStartEdited(int position, real startMs)
    signal moraDurationEdited(int position, real durationMs)
    signal noteGestureEdited(var durations, var positions, var points)
    signal resetUnitRequested(int unitIndex)
    signal fitRequested()

    readonly property real minimumZoom: 1
    readonly property real maximumZoom: 24
    readonly property real displayDuration: Math.max(1, Number(root.waveformDuration) || 1)
    readonly property real timelineDuration: root.gesture ? root.gestureDuration : root.contentDuration
    readonly property var selectedUnit: root.unitAt(root.selectedUnitIndex)
    readonly property real msPerZoomStep: 1.25
    readonly property int dragSnapMs: 10
    readonly property var unitOptions: {
        const values = [];
        for (let index = 0; index < root.units.length; ++index) {
            const unit = root.units[index];
            const name = root.displayName(unit);
            values.push({
                label: (index + 1) + ": " + (name.length ? name : "(silent)"),
                index: index
            });
        }
        return values;
    }

    function unitAt(index) {
        if (index < 0 || index >= root.units.length)
            return null;
        return root.units[index];
    }

    function overrideAt(index) {
        for (const value of root.overrides || []) {
            if (Number(value.unit_index) === Number(index))
                return value;
        }
        return null;
    }

    function unitValue(index, key) {
        if (root.previewUnit && Number(root.previewUnit.index) === Number(index)
                && String(root.previewUnit.key) === String(key))
            return root.previewUnit.value;
        const override = root.overrideAt(index);
        if (override && override[key] !== undefined)
            return override[key];
        const unit = root.unitAt(index);
        const overrideFlag = String(key) + "_override";
        if (String(key).indexOf("resampler_") === 0 && unit
                && unit[overrideFlag] !== true)
            return root.paramRange(key).def;
        return unit && unit[key] !== undefined ? unit[key] : 0;
    }

    function paramRange(key) {
        switch (String(key)) {
        case "pitch_factor":
        case "energy_factor":
            return {min: 0.1, max: 4.0, def: 1.0, isFloat: true};
        case "resampler_velocity":
        case "resampler_volume":
            return {min: 0, max: 200, def: 100, isFloat: false};
        case "resampler_modulation":
            return {min: 0, max: 100, def: 0, isFloat: false};
        case "resampler_tempo":
            return {min: 40, max: 300, def: 120, isFloat: true};
        case "consonant_ms":
        case "preutterance_ms":
        case "overlap_ms":
            return {min: 0, max: 1000, def: 0, isFloat: false};
        case "offset_ms":
        case "cutoff_ms":
            return {min: -2000, max: 5000, def: 0, isFloat: false};
        default:
            return {min: 0, max: 100, def: 0, isFloat: false};
        }
    }

    function isOtoKey(key) {
        return root.otoKeys.indexOf(String(key)) >= 0;
    }

    function paramLabel(key) {
        const names = {
            offset_ms: "offset", cutoff_ms: "cutoff", consonant_ms: "consonant",
            preutterance_ms: "preutterance", overlap_ms: "overlap",
            pitch_factor: "pitchFactor", energy_factor: "energyFactor",
            resampler_velocity: "velocity", resampler_volume: "volume",
            resampler_modulation: "modulation", resampler_tempo: "tempo"
        };
        const suffix = names[String(key)] || String(key);
        return root.translator.tr("main.pitch." + suffix);
    }

    function updateHud(canvasX, canvasY, text) {
        root.hudText = text;
        root.hudX = canvasX;
        root.hudY = canvasY;
    }

    function formatParamValue(key, value) {
        const range = root.paramRange(key);
        if (range.isFloat)
            return (Math.round(Number(value) * 1000) / 1000).toString();
        return Math.round(Number(value)).toString();
    }

    function beginUnitValueDrag(unitIndex, key, x, y) {
        const range = root.paramRange(key);
        root.valueGesture = {
            index: unitIndex, key: String(key), x: x, y: y,
            start: root.unitNumber(unitIndex, key, range.def),
            unitCount: root.units.length
        };
    }

    readonly property real exprBarMaxPx: 160

    function previewUnitValueDrag(x, y, modifiers) {
        const vg = root.valueGesture;
        if (!vg)
            return;
        const range = root.paramRange(vg.key);
        const useSnap = (modifiers & Qt.AltModifier) === 0;
        let value;
        if (root.isOtoKey(vg.key)) {
            value = vg.start + (x - vg.x) / root.pixelsPerMs;
            value = root.snapTime(value, useSnap);
        } else {
            const pxPerUnit = root.exprBarMaxPx / Math.max(1e-9, range.max - range.min);
            value = vg.start + (x - vg.x) / pxPerUnit;
        }
        value = Math.max(range.min, Math.min(range.max, value));
        value = range.isFloat ? Math.round(value * 1000) / 1000 : Math.round(value);
        root.previewUnit = {index: vg.index, key: vg.key, value: value};
        root.updateHud(x, y, root.paramLabel(vg.key) + "  "
                + root.formatParamValue(vg.key, value)
                + (root.isOtoKey(vg.key) ? " ms" : ""));
        waveformCanvas.requestPaint();
        return value;
    }

    function finishUnitValueDrag(commit) {
        const vg = root.valueGesture;
        const pv = root.previewUnit;
        root.valueGesture = null;
        root.previewUnit = null;
        if (commit && vg && pv && Number(pv.index) === Number(vg.index)
                && String(pv.key) === String(vg.key)) {
            const range = root.paramRange(vg.key);
            const current = root.unitNumber(vg.index, vg.key, range.def);
            if (Math.abs(current - Number(pv.value)) > 1e-9)
                root.unitValueEdited(vg.index, vg.key, pv.value);
        }
        root.hudText = "";
        waveformCanvas.requestPaint();
    }

    function syncEdgeModifiers(mods) {
        const shift = (mods & Qt.ShiftModifier) !== 0;
        if (shift !== root.hoveredEdgeFollowing) {
            root.hoveredEdgeFollowing = shift;
            waveformCanvas.requestPaint();
        }
    }

    function setShiftPreview(down) {
        if (!!root.gesture || !!root.valueGesture || root.hoveredBoundary < 0)
            return;
        root.hoveredEdgeFollowing = down;
        waveformCanvas.requestPaint();
    }

    function noteHit(canvasX, canvasY) {
        for (let i = 0; i < root.units.length; ++i) {
            const unit = root.units[i];
            if (!unit || unit.silent || String(unit.role || "") !== "mora")
                continue;
            const startX = root.timeToX(root.unitNoteStart(unit) + root.leadingMargin);
            const endX = root.timeToX(root.unitNoteStart(unit) + root.leadingMargin
                    + Math.max(1, root.unitDuration(unit)));
            if (canvasX < startX || canvasX > endX)
                continue;
            if (Math.abs(canvasY - root.noteY(Number(unit.position))) > 11)
                continue;
            const w = Math.max(1, endX - startX);
            return {
                pos: Number(unit.position),
                unit: i,
                edge: canvasX >= endX - Math.min(8, Math.max(4, w * 0.3))
            };
        }
        return null;
    }

    function boundaryHit(canvasX) {
        let best = -1;
        let bestDistance = 7;
        // The first pronunciation always starts at audio time zero.
        for (let i = 1; i < root.morae.length; ++i) {
            const distance = Math.abs(root.timeToX(root.boundaryTime(i)) - canvasX);
            if (distance <= bestDistance) {
                bestDistance = distance;
                best = i;
            }
        }
        return best;
    }

    function exprBarEnd(unitIndex, lane) {
        const range = root.paramRange(root.exprLanes[lane].key);
        const raw = root.unitNumber(unitIndex, root.exprLanes[lane].key, range.def);
        const norm = Math.max(0, Math.min(1,
                (Number(raw) - range.min) / Math.max(1e-9, range.max - range.min)));
        return Math.max(3, norm * root.exprBarMaxPx);
    }

    function exprHit(canvasX, canvasY) {
        for (let u = 0; u < root.units.length; ++u) {
            const unit = root.units[u];
            if (!unit || unit.silent)
                continue;
            const cx = root.timeToX(root.unitNoteStart(unit) + root.leadingMargin);
            for (let l = 0; l < root.exprLanes.length; ++l) {
                const top = root.exprLaneY(l);
                if (canvasY < top || canvasY > top + root.exprLaneH - 2)
                    continue;
                const endX = cx + root.exprBarEnd(u, l) + 6;
                if (canvasX >= cx - 6 && canvasX <= endX)
                    return {u: u, l: l, key: String(root.exprLanes[l].key)};
            }
        }
        return null;
    }

    function updateTimelineHover(canvasX, canvasY, mods) {
        const lineX = (index) => root.timeToX(root.boundaryTime(index));
        const b = root.boundaryHit(canvasX);
        const n = root.noteHit(canvasX, canvasY);
        root.hoveredUnit = n ? n.unit : -1;
        if (b >= 0 && (!n || Math.abs(canvasX - lineX(b)) <= 4)) {
            root.hoveredBoundary = b;
            root.syncEdgeModifiers(mods);
        } else if (n) {
            root.hoveredBoundary = n.pos;
            root.syncEdgeModifiers(mods);
        } else {
            root.hoveredBoundary = -1;
            root.hoveredEdgeFollowing = false;
        }
        root.hoveredUnit = n ? n.unit : -1;
        if (!n) {
            for (let i = 0; i < root.units.length; ++i) {
                const unit = root.units[i];
                if (!unit || unit.silent)
                    continue;
                const mora = String(unit.role || "") === "mora";
                const sx = root.timeToX(root.unitNoteStart(unit) + root.leadingMargin);
                const ex = root.timeToX(root.unitNoteStart(unit) + root.leadingMargin
                        + Math.max(1, root.unitDuration(unit)));
                const topY = mora ? root.noteY(Number(unit.position)) - 11 : 24;
                const botY = mora ? root.noteY(Number(unit.position)) + 11 : 34;
                if (canvasX >= sx && canvasX <= ex && canvasY >= topY && canvasY <= botY) {
                    root.hoveredUnit = i;
                    break;
                }
            }
        }
        const e = (!n && b < 0) ? root.exprHit(canvasX, canvasY) : null;
        root.hoveredExpr = e ? {u: e.u, l: e.l} : {u: -1, l: -1};
        if (root.framePaintRequested(mods))
            root.timelineCursor = Qt.SizeVerCursor;
        else if (e)
            root.timelineCursor = Qt.SizeVerCursor;
        else if (n)
            root.timelineCursor = n.edge ? Qt.SizeHorCursor
                                         : n.pos === 0 ? Qt.SizeVerCursor : Qt.SizeAllCursor;
        else if (b >= 0)
            root.timelineCursor = Qt.SizeHorCursor;
        else
            root.timelineCursor = Qt.PointingHandCursor;
        waveformCanvas.requestPaint();
    }

    function unitIndexForPosition(position) {
        for (let i = 0; i < root.units.length; ++i) {
            const unit = root.units[i];
            if (!unit || unit.silent)
                continue;
            if (Number(unit.position) === Number(position))
                return i;
        }
        return -1;
    }

    function boundaryTime(position) {
        return root.moraStartAt(position, 0) + root.leadingMargin;
    }

    function unitNumber(index, key, fallback) {
        const value = Number(root.unitValue(index, key));
        return Number.isFinite(value) ? value : fallback;
    }

    function moraStartAt(position, fallback) {
        if (position === 0)
            return 0;
        const positions = root.gesture && root.gesture.mode === "note"
                && root.gesturePositions.length ? root.gesturePositions : root.moraPositions;
        if (position >= 0 && position < positions.length) {
            const value = Number(positions[position]);
            if (Number.isFinite(value))
                return Math.max(0, value);
        }
        return Number.isFinite(Number(fallback)) ? Math.max(0, Number(fallback)) : 0;
    }

    function moraDurationAt(position, fallback) {
        const durations = root.gesture && root.gesture.mode === "note"
                && root.gestureDurations.length ? root.gestureDurations : root.moraDurations;
        if (position >= 0 && position < durations.length) {
            const value = Number(durations[position]);
            if (Number.isFinite(value) && value > 0)
                return value;
        }
        return Number.isFinite(Number(fallback)) ? Math.max(1, Number(fallback)) : 1;
    }

    function unitNoteStart(unit) {
        return root.moraStartAt(Number(unit.position), unit.note_start_ms);
    }

    function unitDuration(unit) {
        return root.moraDurationAt(Number(unit.position), unit.duration_ms);
    }

    function zoomedWidth() {
        return Math.max(1, timelineViewport.width) * root.zoomFactor;
    }

    function timeToX(value) {
        return Number(value) / root.timelineDuration * root.zoomedWidth();
    }

    function xToTime(value) {
        return Math.max(0, Math.min(root.timelineDuration,
                                    Number(value) / Math.max(1, root.zoomedWidth()) * root.timelineDuration));
    }

    function snapTime(value, enabled) {
        if (enabled === false || !root.snapEnabled)
            return value;
        return Math.round(value / root.dragSnapMs) * root.dragSnapMs;
    }

    function zoomAt(factor, anchorX) {
        const next = Math.max(root.minimumZoom,
                              Math.min(root.maximumZoom, root.zoomFactor * factor));
        if (next === root.zoomFactor)
            return;
        const anchorTime = root.xToScrollTime(anchorX);
        root.zoomFactor = next;
        timelineViewport.contentX = Math.max(0,
                Math.min(timelineViewport.contentWidth - timelineViewport.width,
                         root.timeToX(anchorTime) - anchorX));
    }

    function xToScrollTime(anchorX) {        const total = root.zoomedWidth();
        if (total <= 0)
            return 0;
        return (root.currentScrollX + anchorX) / total * root.timelineDuration;
    }

    onFitRequested: {
        root.zoomFactor = 1;
        timelineViewport.contentX = 0;
    }

    function beginDrag(position, mode, x, y, following) {
        if (!root.timingEditor)
            return;
        if (mode === "start" && position === 0)
            return;
        const positions = [];
        const durations = [];
        for (let index = 0; index < root.timingEditor.morae.length; ++index) {
            positions.push(Number(root.timingEditor.positionAt(index)));
            durations.push(Number(root.timingEditor.durationAt(index)));
        }
        root.gestureDuration = root.contentDuration;
        root.gesturePositions = positions.slice();
        root.gestureDurations = durations.slice();
        root.gesture = {
            position: position, mode: mode, x: x, y: y, following: following,
            positions: positions,
            durations: durations,
            points: root.timingEditor.points.slice(),
            start: root.timingEditor.positionAt(position),
            duration: root.timingEditor.durationAt(position),
            pitch: root.timingEditor.pitchAt(position),
            moraCount: root.timingEditor.morae.length
        };
    }

    function gestureModelMatches() {
        const g = root.gesture;
        if (!g || !root.timingEditor)
            return false;
        return root.timingEditor.morae.length === g.moraCount;
    }

    function restoreDrag() {
        const g = root.gesture;
        if (!g || !root.timingEditor)
            return;
        root.timingEditor.moraPositions = g.positions.slice();
        root.timingEditor.moraDurations = g.durations.slice();
        root.timingEditor.points = g.points.slice();
    }

    function durationsForPositions(positions, fallbackDurations) {
        const values = [];
        const count = root.timingEditor ? root.timingEditor.morae.length : 0;
        for (let index = 0; index < count; ++index) {
            let duration = index + 1 < count
                    ? Number(positions[index + 1]) - Number(positions[index])
                    : Number(fallbackDurations[index]);
            if (!Number.isFinite(duration) || duration <= 0)
                duration = root.timingEditor.durationAt(index);
            values.push(Math.round(Math.max(root.timingEditor.minimumDurationAt(index),
                    Math.min(root.timingEditor.maximumDurationAt(index), duration))));
        }
        return values;
    }

    function previewDrag(x, y, modifiers) {
        const g = root.gesture;
        if (!g)
            return;
        if (g.mode === "note") {
            root.timingEditor.points = g.points.slice();
            root.gesturePositions = g.positions.slice();
            root.gestureDurations = g.durations.slice();
        } else {
            root.restoreDrag();
        }
        const snap = (modifiers & Qt.AltModifier) === 0;
        const pointerTime = root.xToTime(x) - root.leadingMargin;
        if (g.mode === "start") {
            root.timingEditor.setPositionAtMS(g.position,
                    root.snapTime(pointerTime, snap), g.following, true);
        } else if (g.mode === "duration") {
            root.timingEditor.setDurationAtMS(g.position,
                    root.snapTime(pointerTime - g.start, snap), true);
        } else if (g.mode === "note") {
            if (g.lock !== "pitch") {
                const count = root.timingEditor.morae.length;
                const base = g.positions.slice();
                let delta = root.xToTime(x) - root.xToTime(g.x);
                if (!Number.isFinite(delta))
                    return;
                if (snap)
                    delta = Math.round(delta / root.dragSnapMs) * root.dragSnapMs;
                const prevMin = g.position > 0
                        ? root.timingEditor.minimumDurationAt(g.position - 1) : 0;
                let lower = -base[g.position];
                if (g.position > 0)
                    lower = Math.max(lower, base[g.position - 1] + prevMin - base[g.position]);
                let upper = Number.POSITIVE_INFINITY;
                if (g.position + 1 < count) {
                    const nextMin = root.timingEditor.minimumDurationAt(g.position + 1);
                    const nextNext = g.position + 2 < count
                            ? base[g.position + 2] - nextMin : Number.POSITIVE_INFINITY;
                    upper = Math.min(upper, nextNext - base[g.position + 1]);
                }
                const clamped = Math.max(lower, Math.min(upper, delta));
                const moved = base.slice();
                moved[g.position] += clamped;
                if (g.position + 1 < count)
                    moved[g.position + 1] += clamped;
                root.gesturePositions = moved;
                root.gestureDurations = root.durationsForPositions(moved, g.durations);
            }
            if (g.lock !== "timing") {
                const rate = (modifiers & Qt.ShiftModifier) !== 0 ? 0.15 : 0.5;
                const values = g.points.slice();
                const automatic = Number(root.timingEditor.autoPoints[g.position]) || 0;
                values[g.position] = Math.round(Math.max(-300, Math.min(300,
                        g.pitch - (y - g.y) / root.noteLaneH * 600 * rate)) - automatic);
                root.timingEditor.points = values;
            }
        }
        const editor = root.timingEditor;
        if (g.mode === "note") {
            const cents = Math.round(editor.pitchAt(g.position));
            const centsText = (cents >= 0 ? "+" : "") + cents + " cent";
            if (g.lock === "pitch")
                root.updateHud(x, y, centsText);
            else if (g.lock === "timing")
                root.updateHud(x, y, Math.round(root.gesturePositions[g.position]) + " ms / "
                        + Math.round(root.gestureDurations[g.position]) + " ms"
                        + (g.following ? "  (Shift)" : ""));
            else
                root.updateHud(x, y, Math.round(root.gesturePositions[g.position]) + " ms / "
                        + Math.round(root.gestureDurations[g.position]) + " ms / " + centsText
                        + (g.following ? "  (Shift)" : ""));
        } else if (g.mode === "start") {
            root.updateHud(x, y, root.translator.tr("main.pitch.position") + "  "
                    + Math.round(editor.positionAt(g.position)) + " ms"
                    + (g.following ? "  (Shift)" : ""));
        } else if (g.mode === "duration") {
            root.updateHud(x, y, root.translator.tr("main.pitch.duration") + "  "
                    + Math.round(editor.durationAt(g.position)) + " ms");
        }
        waveformCanvas.requestPaint();
    }

    function beginNoteDrag(position, x, y, following, lock) {
        if (!root.timingEditor)
            return;
        root.beginDrag(position, "note", x, y, following);
        if (root.gesture)
            root.gesture.lock = lock === "pitch" || lock === "timing" ? lock : "both";
    }

    function finishNoteDrag(cancel) {
        const g = root.gesture;
        if (!g || g.mode !== "note")
            return;
        const editor = root.timingEditor;
        if (cancel) {
            root.restoreDrag();
        } else {
            const positions = root.gesturePositions.slice();
            const durations = root.gestureDurations.slice();
            editor.moraPositions = positions.slice();
            editor.moraDurations = durations.slice();
            root.noteGestureEdited(durations, positions, editor.points.slice());
        }
        root.gesture = null;
        root.gesturePositions = [];
        root.gestureDurations = [];
        root.hudText = "";
        waveformCanvas.requestPaint();
    }

    function finishDrag(cancel) {
        const g = root.gesture;
        if (!g)
            return;
        const editor = root.timingEditor;
        if (cancel) {
            root.restoreDrag();
        } else if (JSON.stringify(g.positions) !== JSON.stringify(editor.moraPositions)
                || JSON.stringify(g.durations) !== JSON.stringify(editor.moraDurations)) {
            editor.timingEdited(editor.moraDurations.slice(), editor.moraPositions.slice());
        }
        root.gesture = null;
        root.gesturePositions = [];
        root.gestureDurations = [];
        root.hudText = "";
        waveformCanvas.requestPaint();
    }

    function commitNumber(unitIndex, key, text, fallback) {
        const value = Number(text);
        root.unitValueEdited(unitIndex, key, Number.isFinite(value) ? value : fallback);
    }

    function displayName(unit) {
        if (!unit)
            return "";
        const alias = String(unit.alias || "");
        const mora = String(unit.mora || "");
        return alias.length ? alias : mora;
    }

    function refreshSelectedFields() {
        const unit = root.selectedUnit;
        const enabled = root.selectedUnitIndex >= 0 && !!unit;
        positionSpin.value = enabled
                ? Math.round(root.moraStartAt(Number(unit.position), unit.note_start_ms)) : 0;
        durationSpin.value = enabled
                ? Math.round(root.moraDurationAt(Number(unit.position), unit.duration_ms)) : 1;
        offsetSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "offset_ms", 0)) : 0;
        cutoffSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "cutoff_ms", 0)) : 0;
        consonantSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "consonant_ms", 0)) : 0;
        preutteranceSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "preutterance_ms", 0)) : 0;
        overlapSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "overlap_ms", 0)) : 0;
        pitchFactorField.text = enabled ? root.unitNumber(root.selectedUnitIndex, "pitch_factor", 1).toFixed(3) : "";
        energyFactorField.text = enabled ? root.unitNumber(root.selectedUnitIndex, "energy_factor", 1).toFixed(3) : "";
        velocitySpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_velocity", 100)) : 100;
        volumeSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_volume", 100)) : 100;
        modulationSpin.value = enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_modulation", 0)) : 0;
        tempoField.text = enabled ? root.unitNumber(root.selectedUnitIndex, "resampler_tempo", 120).toFixed(2) : "";
        flagsField.text = enabled ? String(root.unitValue(root.selectedUnitIndex, "resampler_flags") || "") : "";
    }

    function scheduleSelectedFieldRefresh() {
        Qt.callLater(function() { root.refreshSelectedFields(); });
    }

    function ensureUnitSelection() {
        if (!root.units.length) {
            root.selectedUnitIndex = -1;
            return;
        }
        if (root.selectedUnitIndex >= 0 && root.selectedUnitIndex < root.units.length)
            return;
        root.selectedUnitIndex = 0;
        for (let index = 0; index < root.units.length; ++index) {
            if (!root.units[index].silent) {
                root.selectedUnitIndex = index;
                break;
            }
        }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 6

        Rectangle {
            id: waveformFrame
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.minimumHeight: 150
            color: "transparent"
            border.width: root.showTimelineFrame ? 1 : 0
            border.color: root.dividerColor
            clip: true

            Flickable {
                id: timelineViewport
                anchors.fill: parent
                anchors.margins: 1
                clip: true
                contentWidth: Math.max(width, root.zoomedWidth())
                contentHeight: height
                boundsBehavior: Flickable.StopAtBounds
                interactive: false
                onContentXChanged: {
                    root.currentScrollX = contentX;
                    waveformCanvas.requestPaint();
                }

            Canvas {
                id: waveformCanvas
                width: timelineViewport.contentWidth
                height: timelineViewport.height

                onPaint: {
                    const ctx = getContext("2d");
                    ctx.reset();
                    ctx.clearRect(0, 0, width, height);
                    const center = 55;
                    const viewLeft = timelineViewport.contentX - 60;
                    const viewRight = timelineViewport.contentX + timelineViewport.width + 60;
                    const inView = x => x >= viewLeft && x <= viewRight;
                    const step = Math.pow(10, Math.floor(Math.log10(100 / root.pixelsPerMs)));
                    const tick = step * (step * root.pixelsPerMs < 50 ? 5 : 1);
                    const tickStart = Math.max(0, Math.floor(root.xToTime(viewLeft) / tick) * tick);
                    for (let time = tickStart; time <= root.timelineDuration; time += tick) {
                        const x = root.timeToX(time);
                        if (!inView(x))
                            continue;
                        ctx.strokeStyle = root.gridColor;
                        ctx.beginPath();
                        ctx.moveTo(x, 20);
                        ctx.lineTo(x, height);
                        ctx.stroke();
                    }
                    for (let li = 0; li < root.exprLanes.length; ++li) {
                        const laneKey = root.exprLanes[li].key;
                        const range = root.paramRange(laneKey);
                        const laneTop = root.exprLaneY(li);
                        const laneH = root.exprLaneH - 2;
                        ctx.fillStyle = root.mutedText;
                        ctx.font = "9px sans-serif";
                        ctx.fillText(root.exprLanes[li].code,
                                timelineViewport.contentX + 4, laneTop + 12);
                        ctx.strokeStyle = root.gridColor;
                        ctx.beginPath();
                        ctx.moveTo(0, laneTop + laneH);
                        ctx.lineTo(width, laneTop + laneH);
                        ctx.stroke();
                        for (let ei = 0; ei < root.units.length; ++ei) {
                            const eunit = root.units[ei];
                            if (!eunit || eunit.silent)
                                continue;
                            const cx = root.timeToX(root.unitNoteStart(eunit) + root.leadingMargin);
                            if (!inView(cx) && !inView(cx + root.exprBarMaxPx))
                                continue;
                            const raw = root.unitNumber(ei, laneKey, range.def);
                            const norm = Math.max(0, Math.min(1,
                                    (Number(raw) - range.min) / Math.max(1e-9, range.max - range.min)));
                            const bl = Math.max(3, norm * root.exprBarMaxPx);
                            const eHot = ei === root.selectedUnitIndex
                                    || (root.hoveredExpr.u === ei && root.hoveredExpr.l === li);
                            ctx.fillStyle = Qt.rgba(root.accentColor.r, root.accentColor.g,
                                                    root.accentColor.b, eHot ? 0.6 : 0.3);
                            ctx.fillRect(cx, laneTop + (laneH - 10) / 2, bl, 10);
                            if (eHot) {
                                ctx.fillStyle = root.accentColor;
                                ctx.fillRect(cx + bl - 2, laneTop + (laneH - 14) / 2, 4, 14);
                            }
                        }
                    }
                    ctx.font = "10px sans-serif";
                    if (root.timingEditor) {
                        const middle = root.noteCenterY();
                        for (const cents of [-300, 0, 300]) {
                            const y = middle - cents * root.noteLaneH / 600;
                            ctx.strokeStyle = root.gridColor;
                            ctx.beginPath();
                            ctx.moveTo(Math.max(0, viewLeft), y);
                            ctx.lineTo(Math.min(width, viewRight), y);
                            ctx.stroke();
                        }
                        ctx.strokeStyle = root.axisColor;
                        ctx.lineWidth = 1;
                        ctx.beginPath();
                        let autoStarted = false;
                        for (let fi = 0; fi < root.autoFrames.length; ++fi) {
                            if (!root.frameHasPronunciation(fi)) {
                                autoStarted = false;
                                continue;
                            }
                            const fx = root.timeToX(root.frameTimeMs(fi) + root.leadingMargin);
                            if (!inView(fx)) {
                                autoStarted = false;
                                continue;
                            }
                            const fy = root.frameY(root.frameAutoAt(fi));
                            if (autoStarted) ctx.lineTo(fx, fy);
                            else ctx.moveTo(fx, fy);
                            autoStarted = true;
                        }
                        ctx.stroke();
                        ctx.strokeStyle = root.accentColor;
                        ctx.lineWidth = 2;
                        ctx.beginPath();
                        let frameStarted = false;
                        for (let fj = 0; fj < root.frameDisplayCount; ++fj) {
                            if (!root.frameHasPronunciation(fj)) {
                                frameStarted = false;
                                continue;
                            }
                            const fx = root.timeToX(root.frameTimeMs(fj) + root.leadingMargin);
                            if (!inView(fx)) {
                                frameStarted = false;
                                continue;
                            }
                            const fy = root.frameY(root.frameTotalAt(fj));
                            if (frameStarted) ctx.lineTo(fx, fy);
                            else ctx.moveTo(fx, fy);
                            frameStarted = true;
                        }
                        ctx.stroke();
                        ctx.fillStyle = root.accentColor;
                        for (let fk = 0; fk < root.frameDisplayCount; ++fk) {
                            if (!root.frameHasPronunciation(fk)
                                    || Math.abs(root.frameManualAt(fk)) <= 0.5)
                                continue;
                            const fx = root.timeToX(root.frameTimeMs(fk) + root.leadingMargin);
                            if (!inView(fx))
                                continue;
                            ctx.beginPath();
                            ctx.arc(fx, root.frameY(root.frameTotalAt(fk)), 2.5, 0, Math.PI * 2);
                            ctx.fill();
                        }
                    }
                    const minimums = root.waveformMin || [];
                    const maximums = root.waveformMax || [];
                    const count = Math.min(minimums.length, maximums.length);
                    if (count > 1 && root.waveformDuration > 0) {
                        ctx.strokeStyle = root.accentColor;
                        ctx.globalAlpha = 0.22;
                        ctx.lineWidth = 1;
                        ctx.beginPath();
                        const frac0 = Math.max(0, root.xToTime(viewLeft) / root.waveformDuration);
                        const frac1 = Math.min(1, root.xToTime(viewRight) / root.waveformDuration);
                        const idx0 = Math.max(0, Math.floor(frac0 * (count - 1)));
                        const idx1 = Math.min(count - 1, Math.ceil(frac1 * (count - 1)));
                        for (let index = idx0; index <= idx1; ++index) {
                            const x = root.timeToX(index / (count - 1) * root.waveformDuration);
                            const low = center - Number(minimums[index]) * 27;
                            const high = center - Number(maximums[index]) * 27;
                            ctx.moveTo(x, low);
                            ctx.lineTo(x, high);
                        }
                        ctx.stroke();
                        ctx.globalAlpha = 1;
                    }

                    for (let mi = 1; mi < root.morae.length; ++mi) {
                        const hot = mi === root.selectedBoundary || mi === root.hoveredBoundary
                                || (root.hoveredEdgeFollowing && root.hoveredBoundary >= 0
                                    && mi > root.hoveredBoundary);
                        const bx = root.timeToX(root.boundaryTime(mi));
                        if (!inView(bx))
                            continue;
                        ctx.strokeStyle = hot ? root.accentColor : root.axisColor;
                        ctx.globalAlpha = hot ? 0.9 : 0.45;
                        ctx.lineWidth = hot ? 2 : 1;
                        ctx.beginPath();
                        ctx.moveTo(bx, 0);
                        ctx.lineTo(bx, height);
                        ctx.stroke();
                    }
                    ctx.lineWidth = 1;
                    ctx.globalAlpha = 1;
                    if (root.playbackMs >= 0 && root.playbackMs <= root.timelineDuration) {
                        const px = root.timeToX(root.playbackMs);
                        if (inView(px)) {
                            ctx.strokeStyle = root.accentColor;
                            ctx.lineWidth = 2;
                            ctx.beginPath();
                            ctx.moveTo(px, 0);
                            ctx.lineTo(px, height);
                            ctx.stroke();
                            ctx.fillStyle = root.accentColor;
                            ctx.beginPath();
                            ctx.moveTo(px - 5, 0);
                            ctx.lineTo(px + 5, 0);
                            ctx.lineTo(px, 8);
                            ctx.closePath();
                            ctx.fill();
                        }
                    }
                    for (let ui = 0; ui < root.units.length; ++ui) {
                        if (!root.overrideAt(ui))
                            continue;
                        const u = root.units[ui];
                        if (!u || u.silent)
                            continue;
                        const ux = root.timeToX(root.unitNoteStart(u) + root.leadingMargin);
                        const uw = Math.max(2, root.timeToX(
                                root.unitNoteStart(u) + root.leadingMargin
                                + Math.max(1, root.unitDuration(u))) - ux - 1);
                        if (!inView(ux + uw) && !inView(ux))
                            continue;
                        ctx.fillStyle = root.accentColor;
                        ctx.beginPath();
                        ctx.arc(ux + uw - 7, root.noteY(Number(u.position)), 2.5, 0, Math.PI * 2);
                        ctx.fill();
                    }
                }

                onWidthChanged: requestPaint()
                onHeightChanged: requestPaint()
            }

            MouseArea {
                id: timelineMouse
                x: 0
                y: 0
                width: root.zoomedWidth()
                height: timelineViewport.height
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                property real pressCX: 0
                property real pressCY: 0
                property string pendingKind: ""
                property int pendingPos: -1
                property bool pendingEdge: false
                property int pendingEU: -1
                property int pendingEL: -1
                property string activeKind: ""
                property bool pressMoved: false
                property bool doublePending: false
                property real lastPX: -1
                property real lastPY: -1
                property real pendingSeekX: 0
                property int dragCursor: 0
                property var frameBackup: []
                property int frameLast: -1
                cursorShape: dragCursor !== 0 ? dragCursor : root.timelineCursor
                hoverEnabled: true
                onPressed: mouse => {
                    doublePending = seekTimer.running;
                    seekTimer.stop();
                    if (!!root.gesture || !!root.valueGesture)
                        return;
                    const p = mapToItem(waveformCanvas, mouse.x, mouse.y);
                    if (mouse.button === Qt.RightButton) {
                        const n = root.noteHit(p.x, p.y);
                        if (n) {
                            root.selectedUnitIndex = n.unit;
                            root.selectedBoundary = n.pos;
                            if (root.overrideAt(n.unit))
                                root.resetUnitRequested(n.unit);
                        }
                        return;
                    }
                    pressCX = p.x;
                    pressCY = p.y;
                    lastPX = p.x;
                    lastPY = p.y;
                    pressMoved = false;
                    pendingKind = "";
                    frameLast = -1;
                    if (root.framePaintRequested(mouse.modifiers)) {
                        const target = root.frameAtTime(root.xToTime(p.x) - root.leadingMargin);
                        if (!root.frameHasPronunciation(target))
                            return;
                        pendingKind = "frame";
                        pressMoved = true;
                        frameBackup = root.manualFrames.slice();
                        dragCursor = Qt.SizeVerCursor;
                        return;
                    }
                    root.updateTimelineHover(p.x, p.y, mouse.modifiers);
                    const b = root.boundaryHit(p.x);
                    const n = root.noteHit(p.x, p.y);
                    if (b >= 0 && (!n || Math.abs(p.x - root.timeToX(root.boundaryTime(b))) <= 4)) {
                        root.selectedBoundary = b;
                        const ui = root.unitIndexForPosition(b);
                        if (ui >= 0)
                            root.selectedUnitIndex = ui;
                        pendingKind = "boundary";
                        pendingPos = b;
                    } else if (n) {
                        root.selectedUnitIndex = n.unit;
                        root.selectedBoundary = n.pos;
                        pendingKind = "note";
                        pendingPos = n.pos;
                        const w = Math.max(1, root.timeToX(root.unitNoteStart(root.units[n.unit])
                                + root.leadingMargin + Math.max(1, root.unitDuration(root.units[n.unit])))
                                - root.timeToX(root.unitNoteStart(root.units[n.unit]) + root.leadingMargin));
                        pendingEdge = p.x >= root.timeToX(root.unitNoteStart(root.units[n.unit])
                                + root.leadingMargin) + w - Math.min(8, Math.max(4, w * 0.3));
                    } else {
                        const e = root.exprHit(p.x, p.y);
                        if (e) {
                            root.selectedUnitIndex = e.u;
                            pendingKind = "expr";
                            pendingEU = e.u;
                            pendingEL = e.l;
                        } else {
                            pendingKind = "background";
                        }
                    }
                }
                onPositionChanged: mouse => {
                    const p = mapToItem(waveformCanvas, mouse.x, mouse.y);
                    lastPX = p.x;
                    lastPY = p.y;
                    if (!pressed) {
                        root.updateTimelineHover(p.x, p.y, mouse.modifiers);
                        return;
                    }
                    if (activeKind === "expr") {
                        if (!root.valueGesture)
                            return;
                        const v = root.previewUnitValueDrag(p.x, p.y, mouse.modifiers);
                        const vg = root.valueGesture;
                        if (vg && v !== undefined)
                            ToolTip.show(root.paramLabel(vg.key) + "  "
                                    + root.formatParamValue(vg.key, v));
                        return;
                    }
                    if (activeKind === "timing" || activeKind === "note") {
                        root.previewDrag(p.x, p.y, mouse.modifiers);
                        return;
                    }
                    if (activeKind === "frame" || pendingKind === "frame") {
                        activeKind = "frame";
                        dragCursor = Qt.SizeVerCursor;
                        frameLast = root.paintFrameTo(p.x, p.y, frameLast);
                        return;
                    }
                    if (!!root.gesture || !!root.valueGesture)
                        return;
                    if (Math.max(Math.abs(p.x - pressCX), Math.abs(p.y - pressCY)) < 3)
                        return;
                    pressMoved = true;
                    if (pendingKind === "expr") {
                        root.beginUnitValueDrag(pendingEU,
                                String(root.exprLanes[pendingEL].key), p.x, p.y);
                        activeKind = "expr";
                        dragCursor = Qt.SizeVerCursor;
                    } else if (pendingKind === "boundary") {
                        root.beginDrag(pendingPos, "start", p.x, p.y,
                                (mouse.modifiers & Qt.ShiftModifier) !== 0);
                        activeKind = "timing";
                        dragCursor = Qt.SizeHorCursor;
                    } else if (pendingKind === "note") {
                        const dx = Math.abs(p.x - pressCX);
                        const dy = Math.abs(p.y - pressCY);
                        const horizontal = dx >= dy;
                        if (pendingEdge && horizontal) {
                            root.beginDrag(pendingPos, "duration", p.x, p.y, false);
                            activeKind = "timing";
                            dragCursor = Qt.SizeHorCursor;
                        } else {
                            const firstNote = pendingPos === 0;
                            root.beginNoteDrag(pendingPos, p.x, p.y,
                                    (mouse.modifiers & Qt.ShiftModifier) !== 0,
                                    firstNote ? "pitch"
                                              : dy >= 2 * dx ? "pitch"
                                              : dx > 2 * dy ? "timing" : "both");
                            activeKind = "note";
                            dragCursor = firstNote ? Qt.SizeVerCursor : Qt.SizeAllCursor;
                        }
                    }
                    if (activeKind === "expr") {
                        const v = root.previewUnitValueDrag(p.x, p.y, mouse.modifiers);
                        const vg = root.valueGesture;
                        if (vg && v !== undefined)
                            ToolTip.show(root.paramLabel(vg.key) + "  "
                                    + root.formatParamValue(vg.key, v));
                    } else if (activeKind === "timing" || activeKind === "note")
                        root.previewDrag(p.x, p.y, mouse.modifiers);
                }
                onReleased: {
                    ToolTip.hide();
                    if (activeKind === "frame" || pendingKind === "frame") {
                        const trimmed = root.trimmedFrames(root.manualFrames);
                        if (JSON.stringify(trimmed) !== JSON.stringify(root.trimmedFrames(frameBackup)))
                            root.framesEdited(trimmed);
                        activeKind = "";
                        pendingKind = "";
                        dragCursor = 0;
                        pressMoved = false;
                        frameLast = -1;
                        root.hudText = "";
                        root.updateTimelineHover(lastPX, lastPY, 0);
                        waveformCanvas.requestPaint();
                        return;
                    }
                    if (activeKind === "expr")
                        root.finishUnitValueDrag(true);
                    else if (activeKind === "note")
                        root.finishNoteDrag(false);
                    else if (activeKind === "timing")
                        root.finishDrag(false);
                    activeKind = "";
                    dragCursor = 0;
                    if (!pressMoved && !doublePending) {
                        pendingSeekX = lastPX;
                        seekTimer.restart();
                    }
                    pressMoved = false;
                    root.updateTimelineHover(lastPX, lastPY, 0);
                }
                onCanceled: {
                    ToolTip.hide();
                    if (activeKind === "frame" || pendingKind === "frame") {
                        root.manualFrames = frameBackup.slice();
                        activeKind = "";
                        pendingKind = "";
                        dragCursor = 0;
                        pressMoved = false;
                        frameLast = -1;
                        root.hudText = "";
                        waveformCanvas.requestPaint();
                        return;
                    }
                    if (activeKind === "expr")
                        root.finishUnitValueDrag(false);
                    else if (activeKind === "note")
                        root.finishNoteDrag(true);
                    else if (activeKind === "timing")
                        root.finishDrag(true);
                    activeKind = "";
                    dragCursor = 0;
                    seekTimer.stop();
                    pressMoved = false;
                }
                onDoubleClicked: mouse => {
                    seekTimer.stop();
                    const p = mapToItem(waveformCanvas, mouse.x, mouse.y);
                    if (root.framePaintRequested(mouse.modifiers)) {
                        const target = root.frameAtTime(root.xToTime(p.x) - root.leadingMargin);
                        if (!root.frameHasPronunciation(target))
                            return;
                        let touched = false;
                        for (let index = target - 2; index <= target + 2; ++index) {
                            if (Math.abs(root.frameManualAt(index)) > 0.5) {
                                touched = true;
                                break;
                            }
                        }
                        if (touched) {
                            root.setFrameRange(target - 2, target + 2, 0);
                            root.framesEdited(root.trimmedFrames(root.manualFrames));
                        }
                        return;
                    }
                    const n = root.noteHit(p.x, p.y);
                    if (n) {
                        root.timingEditor.resetPitchAt(n.pos);
                        waveformCanvas.requestPaint();
                        return;
                    }
                    const e = root.exprHit(p.x, p.y);
                    if (e) {
                        root.unitValueEdited(e.u, e.key, root.paramRange(e.key).def);
                        return;
                    }
                    root.fitRequested();
                }
}

            Timer {
                id: seekTimer
                interval: 260
                repeat: false
                onTriggered: root.seekRequested(Math.max(0, root.xToTime(timelineMouse.pendingSeekX)))
            }

            Repeater {
                model: root.units
                delegate: Rectangle {
                    id: unitBand
                    required property var modelData
                    required property int index

                    readonly property bool editable: String(modelData.role) === "mora"
                    readonly property real start: root.unitNoteStart(modelData) + root.leadingMargin
                    readonly property real end: start + Math.max(1, root.unitDuration(modelData))
                    readonly property real centerY: unitBand.editable
                            ? root.noteY(Number(modelData.position)) : 29
                    x: root.timeToX(start)
                    y: unitBand.editable ? unitBand.centerY - 11 : 24
                    width: Math.max(2, root.timeToX(end) - x - 1)
                    height: editable ? 22 : 10
                    radius: 3
                    clip: true
                    color: Qt.rgba(root.accentColor.r, root.accentColor.g, root.accentColor.b,
                                   index === root.selectedUnitIndex ? 0.45
                                   : index === root.hoveredUnit ? 0.32 : 0.2)
                    border.color: index === root.selectedUnitIndex || index === root.hoveredUnit
                                  ? root.accentColor : root.axisColor
                    border.width: index === root.selectedUnitIndex || index === root.hoveredUnit ? 2 : 1
                    z: index === root.selectedUnitIndex ? 2 : 1

                    Text {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.margins: 2
                        text: root.displayName(unitBand.modelData)
                        color: root.labelColor
                        font.pixelSize: 9
                        elide: Text.ElideRight
                        visible: parent.width >= 28 && unitBand.editable
                    }

                    Rectangle {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.rightMargin: 3
                        width: 2
                        height: 12
                        color: root.accentColor
                        visible: unitBand.editable && parent.width > 14
                    }
                }
            }
            DragHud {
                x: Math.max(4, Math.min(parent.width - width - 4, root.hudX + 14))
                y: Math.max(4, Math.min(parent.height - height - 4, root.hudY - 30))
                hudText: root.hudText
                z: 10
            }
            }
            WheelHandler {
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: event => {
                    if ((event.modifiers & Qt.ControlModifier) !== 0) {
                        const anchor = timelineMouse.lastPX >= 0
                                ? timelineMouse.lastPX - timelineViewport.contentX
                                : timelineViewport.width / 2;
                        root.zoomAt(event.angleDelta.y > 0 ? root.msPerZoomStep
                                                           : 1 / root.msPerZoomStep,
                                Math.max(0, Math.min(timelineViewport.width, anchor)));
                    } else {
                        const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                        timelineViewport.contentX = Math.max(
                                    0, Math.min(timelineViewport.contentWidth - timelineViewport.width,
                                                timelineViewport.contentX - delta));
                    }
                    event.accepted = true;
                }
            }
        }

        Rectangle {
            visible: root.showDetails
            Layout.fillWidth: true
            Layout.preferredHeight: visible ? 1 : 0
            color: root.dividerColor
        }

        ScrollView {
            id: detailsPanel
            visible: root.showDetails
            Layout.fillWidth: true
            Layout.preferredHeight: root.showDetails ? 190 : 0
            clip: true
            background: Rectangle { color: "transparent" }
            ScrollBar.vertical.policy: ScrollBar.AsNeeded

            ColumnLayout {
                width: Math.max(0, parent.width - 12)
                spacing: 6

            GridLayout {
                Layout.fillWidth: true
                columns: 4
                columnSpacing: 8
                rowSpacing: 4

                Label {
                    text: root.translator.tr("main.pitch.position")
                }
                SpinBox {
                    id: positionSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 100000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                             && Number(root.selectedUnit.position) > 0
                    value: enabled
                           ? Math.round(root.moraStartAt(Number(root.selectedUnit.position),
                                                         root.selectedUnit.note_start_ms)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: {
                        if (root.selectedUnit)
                            root.moraStartEdited(Number(root.selectedUnit.position), value);
                    }
                }
                Label {
                    text: root.translator.tr("main.pitch.duration")
                }
                SpinBox {
                    id: durationSpin
                    Layout.preferredWidth: 108
                    from: 1
                    to: 100000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled
                           ? Math.round(root.moraDurationAt(Number(root.selectedUnit.position),
                                                              root.selectedUnit.duration_ms)) : 1
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: {
                        if (root.selectedUnit)
                            root.moraDurationEdited(Number(root.selectedUnit.position), value);
                    }
                }

                Label { text: root.translator.tr("main.pitch.offset") }
                SpinBox {
                    id: offsetSpin
                    Layout.preferredWidth: 108
                    from: -60000
                    to: 60000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "offset_ms", 0)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "offset_ms", value)
                }
                Label { text: root.translator.tr("main.pitch.cutoff") }
                SpinBox {
                    id: cutoffSpin
                    Layout.preferredWidth: 108
                    from: -60000
                    to: 60000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "cutoff_ms", 0)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "cutoff_ms", value)
                }

                Label { text: root.translator.tr("main.pitch.consonant") }
                SpinBox {
                    id: consonantSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 60000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "consonant_ms", 0)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "consonant_ms", value)
                }
                Label { text: root.translator.tr("main.pitch.preutterance") }
                SpinBox {
                    id: preutteranceSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 60000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "preutterance_ms", 0)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "preutterance_ms", value)
                }

                Label { text: root.translator.tr("main.pitch.overlap") }
                SpinBox {
                    id: overlapSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 60000
                    editable: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "overlap_ms", 0)) : 0
                    textFromValue: value => value + " ms"
                    valueFromText: text => parseInt(text)
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "overlap_ms", value)
                }
                Label { text: root.translator.tr("main.pitch.pitchFactor") }
                TextField {
                    id: pitchFactorField
                    Layout.preferredWidth: 108
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    text: enabled ? root.unitNumber(root.selectedUnitIndex, "pitch_factor", 1).toFixed(3) : ""
                    validator: DoubleValidator { bottom: 0.001; top: 100.0 }
                    onEditingFinished: root.commitNumber(root.selectedUnitIndex, "pitch_factor", text, 1)
                }
                Label { text: root.translator.tr("main.pitch.energyFactor") }
                TextField {
                    id: energyFactorField
                    Layout.preferredWidth: 108
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    text: enabled ? root.unitNumber(root.selectedUnitIndex, "energy_factor", 1).toFixed(3) : ""
                    validator: DoubleValidator { bottom: 0.001; top: 100.0 }
                    onEditingFinished: root.commitNumber(root.selectedUnitIndex, "energy_factor", text, 1)
                }

                Label { text: root.translator.tr("main.pitch.velocity") }
                SpinBox {
                    id: velocitySpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 200
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_velocity", 100)) : 100
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "resampler_velocity", value)
                }
                Label { text: root.translator.tr("main.pitch.volume") }
                SpinBox {
                    id: volumeSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 200
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_volume", 100)) : 100
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "resampler_volume", value)
                }

                Label { text: root.translator.tr("main.pitch.modulation") }
                SpinBox {
                    id: modulationSpin
                    Layout.preferredWidth: 108
                    from: 0
                    to: 100
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    value: enabled ? Math.round(root.unitNumber(root.selectedUnitIndex, "resampler_modulation", 0)) : 0
                    onValueModified: root.unitValueEdited(root.selectedUnitIndex, "resampler_modulation", value)
                }
                Label { text: root.translator.tr("main.pitch.tempo") }
                TextField {
                    id: tempoField
                    Layout.preferredWidth: 108
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    text: enabled ? root.unitNumber(root.selectedUnitIndex, "resampler_tempo", 120).toFixed(2) : ""
                    validator: DoubleValidator { bottom: 0.001; top: 1000.0 }
                    onEditingFinished: root.commitNumber(root.selectedUnitIndex, "resampler_tempo", text, 120)
                }

                Label { text: root.translator.tr("main.pitch.flags") }
                TextField {
                    id: flagsField
                    Layout.columnSpan: 3
                    Layout.fillWidth: true
                    enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    text: enabled ? String(root.unitValue(root.selectedUnitIndex, "resampler_flags") || "") : ""
                    onEditingFinished: root.unitValueEdited(root.selectedUnitIndex, "resampler_flags", text.trim())
                }

                Label { text: root.translator.tr("main.pitch.flagsPreset") }
                RowLayout {
                    Layout.columnSpan: 3
                    Layout.fillWidth: true
                    spacing: 6
                    Button {
                        text: "g-3"
                        enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                        onClicked: root.unitValueEdited(root.selectedUnitIndex, "resampler_flags", "g-3")
                    }
                    Button {
                        text: "Mt10"
                        enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                        onClicked: root.unitValueEdited(root.selectedUnitIndex, "resampler_flags", "Mt10")
                    }
                    Button {
                        text: root.translator.tr("main.pitch.flagsClear")
                        enabled: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                        onClicked: root.unitValueEdited(root.selectedUnitIndex, "resampler_flags", "")
                    }
                }

                Label {
                    Layout.columnSpan: 4
                    Layout.fillWidth: true
                    visible: root.selectedUnitIndex >= 0 && !!root.selectedUnit
                    text: root.selectedUnit
                          ? String(root.selectedUnit.source_name || root.selectedUnit.source || "")
                          : ""
                    color: root.mutedText
                    elide: Text.ElideMiddle
                    font.pixelSize: 10
                }
            }
        }
        }
    }

    onWaveformMinChanged: waveformCanvas.requestPaint()
    onWaveformMaxChanged: waveformCanvas.requestPaint()
    onLeadingMarginChanged: waveformCanvas.requestPaint()
    onWaveformDurationChanged: {
        root.gestureDuration = 0;
        waveformCanvas.requestPaint();
    }
    onMoraPositionsChanged: {
        waveformCanvas.requestPaint();
        
        scheduleSelectedFieldRefresh();
    }
    onMoraDurationsChanged: scheduleSelectedFieldRefresh()
    onSelectedUnitIndexChanged: {
        scheduleSelectedFieldRefresh();
    }
    onSelectedBoundaryChanged: {
        
        waveformCanvas.requestPaint();
    }
    onOverridesChanged: {
        scheduleSelectedFieldRefresh();
        waveformCanvas.requestPaint();
    }
    onPlaybackMsChanged: waveformCanvas.requestPaint()
    onAutoFramesChanged: waveformCanvas.requestPaint()
    onManualFramesChanged: waveformCanvas.requestPaint()
    onFrameMsChanged: waveformCanvas.requestPaint()
    onUnitsChanged: {
        if (!!root.gesture && !root.gestureModelMatches()) {
            root.restoreDrag();
            root.gesture = null;
            root.gesturePositions = [];
            root.gestureDurations = [];
            root.hudText = "";
        }
        if (!!root.valueGesture && root.units.length !== root.valueGesture.unitCount) {
            root.valueGesture = null;
            root.previewUnit = null;
            root.hudText = "";
        }
        ensureUnitSelection();
        waveformCanvas.requestPaint();
        scheduleSelectedFieldRefresh();
    }
    onWidthChanged: waveformCanvas.requestPaint()
}




