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

    VoicebankDetailsContent {
        id: content
        anchors.fill: parent
        backend: root.backend
        translator: root.translator
    }
}
