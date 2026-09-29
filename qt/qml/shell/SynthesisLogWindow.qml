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

    title: root.translator.tr("log.title")
    visible: false
    transientParent: hostWindow
    width: 720
    height: 420
    minimumWidth: 720
    maximumWidth: 720
    minimumHeight: 420
    maximumHeight: 420
    modality: Qt.ApplicationModal
    flags: Platform.isWeb ? Qt.Dialog | Qt.WindowTitleHint | Qt.WindowCloseButtonHint : Qt.Dialog
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    SynthesisLogContent {
        anchors.fill: parent
        anchors.margins: 12
        backend: root.backend
        translator: root.translator
        onCloseRequested: root.close()
    }
}
