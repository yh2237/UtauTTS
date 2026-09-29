pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import UtauTTS.Platform 1.0

// 設定ウィンドウのホスト。中身は views/SettingsContent.qml が担当する。
ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
    property var audioOutputDevices: []
    signal applyRequested(bool closeAfter)
    signal closed()
    readonly property alias view: settingsView

    title: root.translator.tr("settings.title")
    visible: false
    width: 720
    height: 540
    minimumWidth: 720
    maximumWidth: 720
    minimumHeight: 540
    maximumHeight: 540
    x: root.hostWindow.x + (root.hostWindow.width - width) / 2
    y: root.hostWindow.y + (root.hostWindow.height - height) / 2
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    header: WindowHeader {
        heading: root.title
        visible: Platform.isWeb
        height: Platform.isWeb ? implicitHeight : 0
        onCloseClicked: root.close()
    }

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
