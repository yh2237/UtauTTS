pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Media

// WebAssembly のスマホ向け最小シェル。テキスト入力から合成・再生・保存と
// 音源ZIPの追加だけを提供する。デスクトップ/Web は Main.qml（AppWindow）を使う。
ApplicationWindow {
    id: window
    required property var injectedBackend
    required property var injectedLegalDocuments
    required property string injectedAppName
    required property url injectedRepositoryUrl
    required property bool injectedSelfTest
    required property bool injectedIntonationLab
    required property string injectedIntonationLabExamples

    readonly property var appBackend: injectedBackend
    readonly property bool darkMode: appBackend.darkMode
    property var translator: translatorInstance

    visible: !injectedSelfTest
    title: injectedAppName
    color: palette.window
    palette: Palette {
        window: window.darkMode ? "#202124" : "#f6f6f6"
        windowText: window.darkMode ? "#e8eaed" : "#000000"
        base: window.darkMode ? "#292a2d" : "#ffffff"
        alternateBase: window.darkMode ? "#303134" : "#f0f1f2"
        text: window.darkMode ? "#e8eaed" : "#000000"
        button: window.darkMode ? "#303134" : "#f0f1f2"
        buttonText: window.darkMode ? "#e8eaed" : "#000000"
        highlight: window.darkMode ? "#e8837d" : "#d35f6b"
        highlightedText: window.darkMode ? "#202124" : "#ffffff"
        placeholderText: window.darkMode ? "#9aa0a6" : "#6b7075"
        mid: window.darkMode ? "#5f6368" : "#aeb4ba"
    }

    Translator {
        id: translatorInstance
        backend: window.appBackend
    }

    AppCore {
        id: core
        backend: window.appBackend
        translator: window.translator
    }

    MediaDevices {
        id: mediaDevices
    }

    AudioOutput {
        id: previewAudioOutput
        volume: 1.0
    }

    MediaPlayer {
        id: player
        audioOutput: previewAudioOutput
    }

    Connections {
        target: window.appBackend
        function onPreviewReady() {
            player.stop();
            player.source = window.appBackend.previewUrl;
        }
    }

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            Label {
                Layout.fillWidth: true
                Layout.leftMargin: 12
                text: window.title
                font.bold: true
                elide: Text.ElideRight
            }
            ToolButton {
                text: "☰"
                onClicked: menuDrawer.open()
            }
        }
    }

    Drawer {
        id: menuDrawer
        edge: Qt.RightEdge
        width: Math.min(window.width * 0.8, 320)
        modal: true

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            Button {
                Layout.fillWidth: true
                text: window.translator.tr("menu.file.addVoicebank")
                enabled: !window.appBackend.busy
                onClicked: {
                    menuDrawer.close();
                    window.appBackend.beginAddVoicebanks();
                }
            }
            Button {
                Layout.fillWidth: true
                text: window.translator.tr("settings.theme")
                onClicked: window.appBackend.setDarkMode(!window.appBackend.darkMode)
            }
            Item { Layout.fillHeight: true }
        }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 12

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            TextArea {
                id: input
                placeholderText: window.translator.tr("main.textPlaceholder")
                wrapMode: TextEdit.Wrap
                font.pixelSize: 18
                selectByMouse: true
            }
        }

        Label {
            Layout.fillWidth: true
            visible: window.appBackend.error.length > 0
            text: window.appBackend.error
            color: palette.highlight
            wrapMode: Text.Wrap
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("menu.playback.current")
                enabled: !window.appBackend.busy && input.text.trim().length > 0
                onClicked: window.synthesize()
            }
            Button {
                Layout.preferredWidth: 64
                Layout.preferredHeight: 48
                enabled: window.appBackend.previewUrl.toString().length > 0
                text: player.playbackState === MediaPlayer.PlayingState ? "■" : "▶"
                onClicked: {
                    if (player.playbackState === MediaPlayer.PlayingState)
                        player.stop();
                    else
                        player.play();
                }
            }
            Button {
                Layout.preferredHeight: 48
                text: window.translator.tr("menu.file.saveWav")
                enabled: window.appBackend.previewUrl.toString().length > 0
                onClicked: window.appBackend.savePreview(
                                window.appBackend.defaultSaveFile("utautts-mobile.wav"));
            }
        }
    }

    BusyIndicator {
        anchors.centerIn: parent
        running: window.appBackend.busy
        visible: running
        z: 10
    }

    function synthesize() {
        const text = input.text.trim();
        if (!text.length || window.appBackend.busy)
            return;
        player.stop();
        const language = "ja";
        const voicebank = core.defaultVoicebank();
        window.appBackend.synthesize({
            text: text,
            language: language,
            phonemizer: core.defaultPhonemizer(language),
            voicebank_id: voicebank ? voicebank.id : "",
            model_id: core.defaultModelIdForLanguage(language),
            renderer: core.defaultRendererId(),
            alias_policy: core.normalizeAliasPolicy(window.appBackend.defaultAliasPolicy),
            tone: window.appBackend.defaultTone,
            color: "",
            mora_duration_ms: window.appBackend.defaultMoraDuration,
            pause_duration_ms: window.appBackend.defaultPauseDuration,
            intonation_strength: window.appBackend.defaultIntonationStrength,
            apply_pitch: true,
            dictionary: window.appBackend.dictionaryEntries,
        });
    }
}
