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

    title: root.translator.tr("log.title")
    visible: false
    dialogWidth: 720
    dialogHeight: 420
    // 合成中に閉じても合成は続く。本体は操作できるままにする。
    applicationModal: false
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    SynthesisLogContent {
        anchors.fill: parent
        anchors.margins: Theme.spacingLarge
        backend: root.backend
        translator: root.translator
        onCloseRequested: root.close()
    }
}
