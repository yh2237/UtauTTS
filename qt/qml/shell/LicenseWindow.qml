pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Platform 1.0

AdaptiveDialogWindow {
    id: root
    required property var hostPalette
    required property var translator
    property alias documents: content.documents
    signal closed()

    title: root.translator.tr("license.title")
    visible: false
    dialogWidth: 860
    dialogHeight: 620
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    LicenseContent {
        id: content
        anchors.fill: parent
    }
}
