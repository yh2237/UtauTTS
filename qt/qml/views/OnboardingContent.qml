pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// 初回起動オンボーディングの共有ビュー。ウィンドウ装飾は shell 側が担当する。
ColumnLayout {
    id: content
    required property var backend
    required property var translator

    function languageLabels() {
        const labels = [];
        const codes = content.backend.languageCodes();
        for (let index = 0; index < codes.length; ++index) {
            const code = codes[index];
            labels.push(code === "auto" ? content.translator.tr("settings.language.auto")
                                        : content.backend.languageDisplayName(code));
        }
        return labels;
    }

    anchors.margins: 28
    spacing: 18

    Label {
        Layout.fillWidth: true
        Layout.bottomMargin: 16
        text: content.translator.tr("onboarding.title")
        font.pixelSize: 20
        font.bold: true
        horizontalAlignment: Text.AlignHCenter
    }

    RowLayout {
        Layout.fillWidth: true
        Label {
            Layout.fillWidth: true
            text: content.translator.tr("settings.language")
        }
        ComboBox {
            id: languageCombo
            Layout.preferredWidth: 200
            model: content.languageLabels()
            currentIndex: content.backend.languageCodes().indexOf(content.backend.language)
            onActivated: content.backend.setLanguage(content.backend.languageCodes()[currentIndex])
        }
    }

    RowLayout {
        Layout.fillWidth: true
        Label {
            Layout.fillWidth: true
            text: content.translator.tr("settings.theme")
        }
        ComboBox {
            id: themeCombo
            Layout.preferredWidth: 200
            model: [content.translator.tr("settings.theme.light"),
                    content.translator.tr("settings.theme.dark")]
            currentIndex: content.backend.darkMode ? 1 : 0
            onActivated: content.backend.setDarkMode(currentIndex === 1)
        }
    }

    Label {
        Layout.fillWidth: true
        text: content.translator.tr("onboarding.translationNotice")
        font.pixelSize: 12
        wrapMode: Text.WordWrap
    }

    Item { Layout.fillHeight: true }

    Button {
        Layout.alignment: Qt.AlignRight
        text: content.translator.tr("onboarding.start")
        onClicked: content.backend.setOnboardingCompleted(true)
    }
}
