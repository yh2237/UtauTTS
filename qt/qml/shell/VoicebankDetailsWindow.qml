pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Platform 1.0

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
    signal closed()
    property alias currentIndex: content.currentIndex

    title: root.translator.tr("voicebankDetails.title")
    visible: false
    width: 860
    height: 620
    minimumWidth: 860
    maximumWidth: 860
    minimumHeight: 620
    maximumHeight: 620
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Platform.isWeb ? Qt.Dialog | Qt.WindowTitleHint | Qt.WindowCloseButtonHint : Qt.Dialog
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
