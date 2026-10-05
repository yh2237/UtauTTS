import QtQuick
import QtTest
import "../qml/controls"

TestCase {
    name: "ParameterSlider"
    when: windowShown
    visible: true
    width: 320
    height: 160

    ParameterSlider {
        id: slider
        x: 20
        y: 40
        width: 240
        from: 0
        to: 100
        stepSize: 5
        onResetRequested: value = 50
    }

    SignalSpy {
        id: moves
        target: slider
        signalName: "moved"
    }

    SignalSpy {
        id: resets
        target: slider
        signalName: "resetRequested"
    }

    function init() {
        slider.value = 50;
        slider.previousTapTime = 0;
        moves.clear();
        resets.clear();
    }

    function test_keyboard() {
        slider.forceActiveFocus();
        keyClick(Qt.Key_Right);
        compare(slider.value, 55);
        compare(moves.count, 1);
        keyClick(Qt.Key_Left);
        compare(slider.value, 50);
    }

    function test_dragAndFocus() {
        mouseDrag(slider, slider.width / 2, slider.height / 2, 70, 0);
        verify(slider.value > 50);
        compare(slider.value % 5, 0);
        verify(moves.count > 0);
        verify(slider.activeFocus);
    }

    function test_doubleClickReset() {
        slider.value = 80;
        mouseDoubleClickSequence(slider, slider.width * 0.8, slider.height / 2);
        tryCompare(resets, "count", 1);
        compare(slider.value, 50);
    }

    function test_dragIsNotDoubleClick() {
        mouseClick(slider, slider.width / 2, slider.height / 2);
        mouseDrag(slider, slider.width / 2, slider.height / 2, 60, 0);
        compare(resets.count, 0);
        verify(slider.value > 50);
    }

    function test_doubleClickOnTrack() {
        slider.value = 10;
        mouseDoubleClickSequence(slider, slider.width * 0.8, slider.height / 2);
        tryCompare(resets, "count", 1);
        compare(slider.value, 50);
    }
}
