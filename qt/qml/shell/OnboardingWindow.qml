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

    title: root.translator.tr("onboarding.title")
    visible: false
    dialogWidth: 460
    dialogHeight: 350
    centerInHost: true
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    OnboardingContent {
        id: content
        anchors.fill: parent
        backend: root.backend
        translator: root.translator
    }
}
