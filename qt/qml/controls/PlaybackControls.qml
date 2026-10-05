pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

RowLayout {
    id: root
    required property var translator

    FontLoader {
        id: iconFont
        source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
    }
    property color mutedText
    property bool busy: false
    property bool playing: false
    property bool hasAudio: false
    property bool canGenerate: false
    property real position: 0
    property real duration: 0
    property string errorText: ""
    signal primaryClicked()
    signal seekRequested(real position)

    spacing: 8

    function formatTime(milliseconds) {
        const seconds = Math.max(0, Math.floor(milliseconds / 1000));
        const minutes = Math.floor(seconds / 60);
        return minutes + ":" + String(seconds % 60).padStart(2, "0");
    }

    readonly property bool waiting: root.busy && !root.playing && !root.hasAudio

    RoundButton {
        id: playbackButton
        Layout.preferredWidth: 40
        Layout.preferredHeight: 40
        highlighted: true
        enabled: root.playing || root.hasAudio || root.canGenerate
        onClicked: root.primaryClicked()
        ToolTip.visible: hovered
        ToolTip.text: root.errorText.length ? root.errorText
                      : root.playing ? root.translator.tr("main.playback.paused")
                      : root.translator.tr("main.playback.generateAndPlay")

        contentItem: Text {
            objectName: "previewPlaybackIcon"
            anchors.centerIn: parent
            visible: !root.waiting
            text: root.playing ? "\ue034" : "\ue037"
            color: playbackButton.palette.buttonText
            font.family: iconFont.name
            font.pixelSize: 22
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }

        Item {
            id: waitingIndicator
            objectName: "previewWaitingIndicator"
            anchors.centerIn: parent
            width: 22
            height: 22
            visible: root.waiting

            Canvas {
                id: waitingArc
                anchors.fill: parent
                property color strokeColor: playbackButton.palette.buttonText
                onStrokeColorChanged: requestPaint()
                onPaint: {
                    const context = getContext("2d");
                    context.clearRect(0, 0, width, height);
                    context.strokeStyle = strokeColor;
                    context.lineWidth = 2;
                    context.lineCap = "round";
                    context.beginPath();
                    context.arc(width / 2, height / 2, width / 2 - 2,
                                -Math.PI / 2, Math.PI);
                    context.stroke();
                }
            }

            RotationAnimator on rotation {
                from: 0
                to: 360
                duration: 2000
                loops: Animation.Infinite
                running: root.waiting
            }
        }
    }

    Slider {
        id: seekSlider
        objectName: "previewSeekSlider"
        Layout.fillWidth: true
        from: 0
        to: Math.max(1, root.duration)
        enabled: root.hasAudio
        onMoved: root.seekRequested(value)

        // ドラッグ中は再生位置の更新を反映しない。
        Binding {
            target: seekSlider
            property: "value"
            value: root.position
            when: !seekSlider.pressed
            restoreMode: Binding.RestoreNone
        }
    }

    Label {
        text: root.formatTime(root.position) + " / " + root.formatTime(root.duration)
        color: root.mutedText
        font.pixelSize: 11
    }
}
