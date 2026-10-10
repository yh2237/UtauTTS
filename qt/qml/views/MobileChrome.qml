pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    required property var window
    readonly property var backend: window.appBackend

    function openMenu() { menuDrawer.open(); }

    Connections {
        target: root.window
        function onMobileLayoutChanged() {
            if (!root.window.mobileLayout) {
                menuDrawer.close();
                settingsDialog.close();
                dictionaryDialog.close();
                licenseDialog.close();
                onboardingDialog.close();
            }
        }
    }

    Drawer {
        id: menuDrawer
        parent: Overlay.overlay
        edge: Qt.LeftEdge
        width: Math.min(root.window.width * 0.85, 360)
        height: parent ? parent.height : 0
        y: 0
        modal: root.window.mobileLayout
        interactive: root.window.mobileLayout
        leftPadding: 16
        rightPadding: 16
        topPadding: 16
        bottomPadding: 16
        exit: Transition {
            NumberAnimation {
                property: "position"
                to: 0
                duration: root.window.mobileLayout ? 150 : 0
            }
        }
        background: Rectangle { color: root.window.palette.window }

        contentItem: ScrollView {
            id: menuScroll
            implicitWidth: 0
            contentWidth: availableWidth
            // デスクトップのメニューバーと同じ順に並べる。スマホで使わない項目（終了・フォルダを開く・プラグイン・exo）と、
            // 同じ動作になる項目（Webでは「名前を付けて保存」は「保存」と同じ）は置かない。元に戻す・やり直すは見出しにある。
            ColumnLayout {
                width: menuScroll.availableWidth
                spacing: 6

                component SectionLabel: Label {
                    Layout.fillWidth: true
                    Layout.topMargin: 8
                    font.bold: true
                    font.pixelSize: 13
                    opacity: 0.6
                }
                component MenuButton: Button {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 44
                }

                SectionLabel { text: root.window.translator.tr("menu.file"); Layout.topMargin: 0 }
                MenuButton {
                    text: root.window.translator.tr("menu.file.open")
                    enabled: !root.backend.busy && !root.window.batchExportActive
                    onClicked: { menuDrawer.close(); root.window.openProject(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.file.save")
                    enabled: !root.backend.busy && !root.window.batchExportActive
                    onClicked: { menuDrawer.close(); root.window.saveCurrentProject(); }
                }

                SectionLabel { text: root.window.translator.tr("menu.file.export") }
                MenuButton {
                    text: root.window.translator.tr("menu.file.saveWav")
                    enabled: !root.backend.busy && !root.window.batchExportActive && root.window.utterancesModel.count > 0
                             && root.window.current().reading.length > 0
                    onClicked: { menuDrawer.close(); root.window.saveCurrentAudio(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.file.saveAllWav")
                    enabled: !root.backend.busy && !root.window.batchExportActive && root.window.hasExportableText()
                    onClicked: { menuDrawer.close(); root.window.openSaveAllDialog(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.file.exportUstx")
                    enabled: !root.backend.busy && !root.window.batchExportActive && root.window.utterancesModel.count > 0
                    onClicked: { menuDrawer.close(); root.window.openUstxExportDialog(); }
                }

                SectionLabel { text: root.window.translator.tr("menu.file.voicebanks") }
                MenuButton {
                    text: root.window.translator.tr("menu.file.addVoicebank")
                    enabled: !root.backend.busy && !root.window.batchExportActive
                    onClicked: { menuDrawer.close(); root.backend.beginAddVoicebanks(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.file.reloadVoicebanks")
                    enabled: !root.backend.busy
                    onClicked: { menuDrawer.close(); root.window.reloadVoicebanks(); }
                }

                SectionLabel { text: root.window.translator.tr("menu.playback") }
                MenuButton {
                    text: root.window.translator.tr("menu.playback.all")
                    enabled: !root.backend.busy && !root.window.batchExportActive && !root.window.playbackQueueActive
                             && root.window.hasPlayableTextFrom(0)
                    onClicked: { menuDrawer.close(); root.window.startPlaybackQueue(0); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.playback.fromSelected")
                    enabled: !root.backend.busy && !root.window.batchExportActive && !root.window.playbackQueueActive
                             && root.window.hasPlayableTextFrom(root.window.selectedIndex)
                    onClicked: { menuDrawer.close(); root.window.startPlaybackQueue(root.window.selectedIndex); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.playback.replay")
                    enabled: !root.backend.busy && !root.window.batchExportActive && !root.window.playbackQueueActive
                             && root.window.hasCachedAudio()
                    onClicked: { menuDrawer.close(); root.window.replayCachedAudio(); }
                }

                SectionLabel { text: root.window.translator.tr("menu.settings") }
                MenuButton {
                    text: root.window.translator.tr("menu.settings.settings")
                    enabled: !root.window.batchExportActive
                    onClicked: { menuDrawer.close(); settingsDialog.open(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.settings.dictionary")
                    enabled: !root.window.batchExportActive
                    onClicked: {
                        menuDrawer.close();
                        dictionaryContent.loadCurrent();
                        dictionaryDialog.open();
                    }
                }

                SectionLabel { text: root.window.translator.tr("menu.help") }
                MenuButton {
                    text: root.window.translator.tr("menu.help.about")
                    onClicked: {
                        menuDrawer.close();
                        if (!root.backend.showNativeAboutDialog())
                            root.window.menuDialogs.about.open();
                    }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.help.repository")
                    onClicked: { menuDrawer.close(); Qt.openUrlExternally(root.window.repositoryUrl); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.help.license")
                    onClicked: { menuDrawer.close(); licenseDialog.open(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.help.voicebankDetails")
                    enabled: root.backend.voicebanks.length > 0
                    onClicked: { menuDrawer.close(); root.window.showVoicebankDetails(); }
                }
                MenuButton {
                    text: root.window.translator.tr("menu.help.exportDiagnostics")
                    onClicked: {
                        menuDrawer.close();
                        root.window.exportDiagnosticsTo(root.backend.defaultSaveFile("utautts-diagnostics.json"));
                    }
                }
                MenuButton {
                    text: root.window.translator.tr("onboarding.title")
                    onClicked: { menuDrawer.close(); onboardingDialog.open(); }
                }
            }
        }
    }

    MobilePageDialog {
        id: settingsDialog
        title: root.window.translator.tr("menu.settings")
        property bool exportText: false
        property bool exportLab: false
        property string exportEncoding: "utf-8"
        onOpened: {
            exportText = root.backend.exportTextWithWav;
            exportLab = root.backend.exportLabWithWav;
            exportEncoding = root.backend.exportTextEncoding;
        }
        onClosed: root.backend.setExportSettings(exportText, exportLab, exportEncoding)
        contentItem: ScrollView {
            id: settingsScroll
            implicitWidth: 0
            contentWidth: availableWidth
            ColumnLayout {
                width: settingsScroll.availableWidth
                spacing: 12
                Label { text: root.window.translator.tr("settings.language"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: root.backend.languageCodes().map(code => ({
                        id: code, label: code === "auto" ? root.window.translator.tr("settings.language.auto")
                                                       : root.backend.languageDisplayName(code)
                    }))
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: root.backend.languageCodes().indexOf(root.backend.language)
                    onActivated: root.backend.setLanguage(currentValue)
                }
                Switch {
                    text: root.window.translator.tr("settings.theme.dark")
                    checked: root.backend.darkMode
                    onToggled: root.backend.setDarkMode(checked)
                }
                CheckBox {
                    text: root.window.translator.tr("settings.exportTextWithWav")
                    checked: settingsDialog.exportText
                    onToggled: settingsDialog.exportText = checked
                }
                CheckBox {
                    text: root.window.translator.tr("settings.exportLabWithWav")
                    checked: settingsDialog.exportLab
                    onToggled: settingsDialog.exportLab = checked
                }
                ComboBox {
                    Layout.fillWidth: true
                    model: ["utf-8", "shift_jis"]
                    currentIndex: settingsDialog.exportEncoding === "shift_jis" ? 1 : 0
                    onActivated: settingsDialog.exportEncoding = currentText
                }
            }
        }
    }

    MobilePageDialog {
        id: dictionaryDialog
        title: root.window.translator.tr("dictionary.title")
        contentItem: DictionaryContent {
            id: dictionaryContent
            backend: root.backend
            translator: root.window.translator
            hostWindow: root.window
            onCloseRequested: dictionaryDialog.close()
        }
    }

    MobilePageDialog {
        id: licenseDialog
        title: root.window.translator.tr("license.title")
        contentItem: LicenseContent { documents: root.window.licenseDocuments }
    }

    MobilePageDialog {
        id: onboardingDialog
        title: root.window.translator.tr("onboarding.title")
        contentItem: OnboardingContent {
            backend: root.backend
            translator: root.window.translator
        }
    }
}
