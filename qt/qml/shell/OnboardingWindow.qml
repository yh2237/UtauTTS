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

    title: root.translator.tr("onboarding.title")
    visible: false
    width: 460
    height: 350
    minimumWidth: 460
    maximumWidth: 460
    minimumHeight: 350
    maximumHeight: 350
    x: root.hostWindow.x + (root.hostWindow.width - width) / 2
    y: root.hostWindow.y + (root.hostWindow.height - height) / 2
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Platform.isWeb ? Qt.Dialog | Qt.WindowTitleHint | Qt.WindowCloseButtonHint : Qt.Dialog
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
