import QtQuick
import QtTest
import "../qml/editors"

TestCase {
    name: "PitchEditor"
    when: windowShown
    visible: true
    width: 600
    height: 400

    PitchEditor {
        id: editor
        width: 560
        height: 320
    }

    SignalSpy {
        id: edits
        target: editor
        signalName: "pointsEdited"
    }

    SignalSpy {
        id: timingEdits
        target: editor
        signalName: "timingEdited"
    }

    function init() {
        editor.morae = [{mora: "か", pause: false}, {mora: "さ", pause: false},
                        {mora: "た", pause: false}];
        editor.points = [0, 0, 0];
        editor.autoPoints = [0, 0, 0];
        editor.moraDurations = [120, 120, 120];
        editor.moraPositions = [0, 120, 240];
        editor.horizontalOffset = 0;
        edits.clear();
        timingEdits.clear();
        waitForRendering(editor);
    }

    function test_dragPitchPointVertically() {
        mouseDrag(editor, editor.pointX(1), editor.pointY(1), 0, -30);
        verify(editor.points[1] > 0, "pitch point did not move vertically");
        compare(editor.points[0], 0);
        compare(edits.count, 1);
        compare(editor.moraPositions[1], 120);
    }

    function test_firstLabelIsNotClipped() {
        verify(editor.pointX(0) - editor.moraWidth / 2 >= 0,
               "the first mora label extends outside the viewport");
    }

    function test_timingBoundaryStillDragsHorizontally() {
        mouseDrag(editor, editor.pointX(1), 20, 15, 0);
        verify(editor.moraPositions[1] > 120);
        compare(editor.points[1], 0);
        compare(timingEdits.count, 1);
        compare(edits.count, 0);
    }

    function test_backgroundStillPans() {
        const morae = [];
        const points = [];
        const durations = [];
        const positions = [];
        for (let i = 0; i < 20; ++i) {
            morae.push({mora: "か", pause: false});
            points.push(0);
            durations.push(120);
            positions.push(i * 120);
        }
        editor.morae = morae;
        editor.points = points;
        editor.autoPoints = points;
        editor.moraDurations = durations;
        editor.moraPositions = positions;
        waitForRendering(editor);
        mouseDrag(editor, 250, 20, -70, 0);
        verify(editor.horizontalOffset > 0);
        compare(edits.count, 0);
        compare(timingEdits.count, 0);
    }
}
