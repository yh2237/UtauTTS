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

    title: root.translator.tr("dictionary.title")
    visible: false
    width: 760
    height: 560
    minimumWidth: 760
    maximumWidth: 760
    minimumHeight: 560
    maximumHeight: 560
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Platform.isWeb ? Qt.Dialog | Qt.WindowTitleHint | Qt.WindowCloseButtonHint : Qt.Dialog
    palette: hostPalette
    color: palette.window

    onClosing: root.closed()

    function loadCurrent() {
        content.loadCurrent();
    }

    DictionaryContent {
        id: content
        anchors.fill: parent
        backend: root.backend
        translator: root.translator
        hostWindow: root.hostWindow
        onCloseRequested: {
            root.close();
            root.visible = false;
        }
    }
}
