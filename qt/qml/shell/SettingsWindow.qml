pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import UtauTTS.Platform 1.0

AdaptiveDialogWindow {
    id: root
    required property var hostPalette
    required property var backend
    required property var translator
    property var audioOutputDevices: []
    signal applyRequested(bool closeAfter)
    signal closed()
    readonly property alias view: settingsView

    title: root.translator.tr("settings.title")
    visible: false
    dialogWidth: 720
    dialogHeight: 540
    centerInHost: true
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    function loadCurrent() {
        settingsView.loadCurrent();
    }

    SettingsContent {
        id: settingsView
        anchors.fill: parent
        hostWindow: root.hostWindow
        hostPalette: root.hostPalette
        backend: root.backend
        translator: root.translator
        audioOutputDevices: root.audioOutputDevices
        onApplyRequested: closeAfter => root.applyRequested(closeAfter)
        onCloseRequested: root.close()
    }
}
