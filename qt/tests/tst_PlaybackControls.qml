import QtQuick
import QtTest
import "../qml/controls"

TestCase {
    name: "PlaybackControls"
    when: windowShown
    visible: true
    width: 500
    height: 160

    QtObject {
        id: translations
        function tr(key) { return key; }
    }

    PlaybackControls {
        id: playback
        x: 20
        y: 40
        width: 460
        translator: translations
        hasAudio: true
        duration: 10000
        position: 5000
    }

    function init() {
        playback.busy = false;
        playback.playing = false;
        playback.hasAudio = true;
        playback.canGenerate = false;
        playback.position = 5000;
    }

    function test_iconColorDoesNotSwitchAfterPlayback() {
        const icon = findChild(playback, "previewPlaybackIcon");
        verify(icon !== null);
        playback.hasAudio = false;
        const before = icon.color;
        playback.hasAudio = true;
        playback.playing = true;
        compare(icon.color, before);
        playback.playing = false;
        compare(icon.color, before);
    }

    function test_waitingIndicator() {
        const indicator = findChild(playback, "previewWaitingIndicator");
        verify(indicator !== null);
        playback.hasAudio = false;
        playback.busy = true;
        verify(indicator.visible);
        playback.busy = false;
        verify(!indicator.visible);
    }

    function test_seekDoesNotJumpWhileDragging() {
        const slider = findChild(playback, "previewSeekSlider");
        verify(slider !== null);
        mousePress(slider, slider.width / 2, slider.height / 2);
        mouseMove(slider, slider.width * 0.75, slider.height / 2);
        const draggedValue = slider.value;
        verify(draggedValue > 5000);
        playback.position = 1000;
        wait(10);
        compare(slider.value, draggedValue);
        mouseRelease(slider, slider.width * 0.75, slider.height / 2);
        tryCompare(slider, "value", 1000);
    }
}
