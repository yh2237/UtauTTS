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

    spacing: 10

    function formatTime(milliseconds) {
        const seconds = Math.max(0, Math.floor(milliseconds / 1000));
        const minutes = Math.floor(seconds / 60);
        return minutes + ":" + String(seconds % 60).padStart(2, "0");
    }

    RoundButton {
        id: playbackButton
        Layout.preferredWidth: 42
        Layout.preferredHeight: 42
        highlighted: true
        enabled: root.playing || root.hasAudio || (!root.busy && root.canGenerate)
        onClicked: root.primaryClicked()
        ToolTip.visible: hovered
        ToolTip.text: root.errorText.length ? root.errorText
                      : root.playing ? root.translator.tr("main.playback.paused")
                      : root.translator.tr("main.playback.generateAndPlay")

        contentItem: Text {
            anchors.centerIn: parent
            text: root.busy ? "\ue5d3" : root.playing ? "\ue034" : "\ue037"
            color: playbackButton.palette.buttonText
            font.family: iconFont.name
            font.pixelSize: 24
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }
    }

    Slider {
        Layout.fillWidth: true
        from: 0
        to: Math.max(1, root.duration)
        value: root.position
        enabled: root.hasAudio
        onMoved: root.seekRequested(value)
    }

    Label {
        text: root.formatTime(root.position) + " / " + root.formatTime(root.duration)
        color: root.mutedText
        font.pixelSize: 11
    }
}
