pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator

    function languageLabels() {
        const labels = [];
        const codes = root.backend.languageCodes();
        for (let index = 0; index < codes.length; ++index) {
            const code = codes[index];
            labels.push(code === "auto" ? root.translator.tr("settings.language.auto")
                                        : root.backend.languageDisplayName(code));
        }
        return labels;
    }

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
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 28
        spacing: 18

        Label {
            Layout.fillWidth: true
            Layout.bottomMargin: 16
            text: root.translator.tr("onboarding.title")
            font.pixelSize: 20
            font.bold: true
            horizontalAlignment: Text.AlignHCenter
        }

        RowLayout {
            Layout.fillWidth: true
            Label {
                Layout.fillWidth: true
                text: root.translator.tr("settings.language")
            }
            ComboBox {
                id: languageCombo
                Layout.preferredWidth: 200
                model: root.languageLabels()
                currentIndex: root.backend.languageCodes().indexOf(root.backend.language)
                onActivated: root.backend.setLanguage(root.backend.languageCodes()[currentIndex])
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Label {
                Layout.fillWidth: true
                text: root.translator.tr("settings.theme")
            }
            ComboBox {
                id: themeCombo
                Layout.preferredWidth: 200
                model: [root.translator.tr("settings.theme.light"),
                        root.translator.tr("settings.theme.dark")]
                currentIndex: root.backend.darkMode ? 1 : 0
                onActivated: root.backend.setDarkMode(currentIndex === 1)
            }
        }

        Label {
            Layout.fillWidth: true
            text: root.translator.tr("onboarding.translationNotice")
            color: root.palette.placeholderText
            font.pixelSize: 12
            wrapMode: Text.WordWrap
        }

        Item { Layout.fillHeight: true }

        Button {
            Layout.alignment: Qt.AlignRight
            text: root.translator.tr("onboarding.start")
            onClicked: root.backend.setOnboardingCompleted(true)
        }
    }
}
