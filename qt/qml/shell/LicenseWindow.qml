pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Platform 1.0

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var translator
    property alias documents: content.documents
    signal closed()

    title: root.translator.tr("license.title")
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

    LicenseContent {
        id: content
        anchors.fill: parent
    }
}
