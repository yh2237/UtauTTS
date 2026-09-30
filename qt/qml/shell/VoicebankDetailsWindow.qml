pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Platform 1.0

AdaptiveDialogWindow {
    id: root
    required property var hostPalette
    required property var backend
    required property var translator
    signal closed()
    property alias currentIndex: content.currentIndex

    title: root.translator.tr("voicebankDetails.title")
    visible: false
    dialogWidth: 860
    dialogHeight: 620
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    VoicebankDetailsContent {
        id: content
        anchors.fill: parent
        backend: root.backend
        translator: root.translator
    }
}
