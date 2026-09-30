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

    title: root.translator.tr("dictionary.title")
    visible: false
    dialogWidth: 760
    dialogHeight: 560
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
