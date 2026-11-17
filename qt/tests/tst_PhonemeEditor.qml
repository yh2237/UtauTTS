import QtQuick
import QtTest
import "../qml/editors"

TestCase {
    name: "PhonemeEditor"
    when: windowShown
    visible: true
    width: 600
    height: 420

    QtObject {
        id: translations
        function tr(key) { return key; }
    }

    PitchEditor {
        id: timing
        visible: false
    }

    PhonemeEditor {
        id: editor
        width: 560
        height: 400
        translator: translations
        timingEditor: timing
    }

    function init() {
        timing.morae = [{mora: "あ"}, {mora: "い"}, {mora: "う"}];
        timing.points = [100, 0, -100];
        timing.autoPoints = [500, 500, 500];
        timing.moraDurations = [100, 100, 100];
        timing.moraPositions = [0, 100, 200];
        editor.morae = timing.morae;
        editor.moraDurations = timing.moraDurations;
        editor.moraPositions = timing.moraPositions;
        editor.units = [
            {position: 0, role: "mora", note_start_ms: 0, duration_ms: 100, pitch_factor: 1},
            {position: 1, role: "mora", note_start_ms: 100, duration_ms: 100, pitch_factor: 1},
            {position: 2, role: "mora", note_start_ms: 200, duration_ms: 100, pitch_factor: 1}
        ];
        editor.autoFrames = Array(31).fill(50);
        editor.manualFrames = Array(31).fill(25);
        editor.overrides = [];
        editor.frameMs = 10;
        editor.leadingMargin = 0;
        editor.gesture = null;
        waitForRendering(editor);
    }

    function test_combinedCurveDoesNotCountAutomaticPitchTwice() {
        compare(editor.frameTotalAt(5), 175);
        compare(editor.frameTotalAt(15), 75);
        compare(editor.frameTotalAt(25), -25);
    }

    function test_penTargetsTheCombinedCurve() {
        editor.overrides = [{unit_index: 0, pitch_factor: 2}];
        const time = 50;
        const desired = 200;
        const y = editor.frameY(desired);
        const target = editor.paintFrameTo(editor.timeToX(time), y, -1);
        compare(target, 5);
        fuzzyCompare(editor.frameTotalAt(target), desired, 0.1);
        compare(editor.manualFrames[target], -1150);
    }

    function test_sparseFramesKeepTheirTimelinePosition() {
        editor.manualFrames = [0, 25, 0];
        compare(editor.frameManualAt(1), 25);
        compare(editor.frameManualAt(10), 0);
        compare(editor.frameManualAt(25), 0);
    }
}
