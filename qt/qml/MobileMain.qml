pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Media

// WebAssembly のスマホ向けシェル。デスクトップのメイン領域（発話カードのリスト）に
// 近い構成で、テキスト追加・合成・再生・保存と、発話ごとの設定/イントネーション、
// グローバル設定、共有ビュー（辞書/ライセンス/初回案内）を提供する。
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
    readonly property var licenseDocuments: injectedLegalDocuments
    property var translator: translatorInstance

    property int selectedIndex: 0
    property string language: "auto"
    property bool exportTextWithWav: false
    property bool exportLabWithWav: false
    property string exportTextEncoding: "utf-8"
    property bool playAfterSynthesize: false

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

    readonly property color accent: palette.highlight
    readonly property color borderColor: palette.mid
    readonly property color mutedText: palette.mid

    FontLoader {
        id: iconFont
        source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
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

    ListModel {
        id: utterances
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
            if (window.playAfterSynthesize) {
                window.playAfterSynthesize = false;
                player.play();
            }
        }
        function onLanguageChanged() {
            window.translator.load(window.appBackend.resolvedLanguage());
            window.language = window.appBackend.language;
        }
        function onMetadataChanged() {
            window.syncFromBackend();
        }
        function onExportSettingsChanged() {
            window.loadExportSettings();
        }
    }

    Component.onCompleted: {
        window.translator.load(window.appBackend.resolvedLanguage());
        window.loadExportSettings();
        window.syncFromBackend();
        if (!utterances.count)
            window.addUtterance();
    }

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            Item { Layout.fillWidth: true }
            ToolButton {
                text: "☰"
                font.pixelSize: 30
                Layout.preferredWidth: 64
                Layout.preferredHeight: 56
                onClicked: menuDrawer.open()
            }
        }
    }

    // --- 発話エディタ（デスクトップのメイン領域相当） ---

    Item {
        anchors.fill: parent

        ListView {
            id: utteranceList
            anchors.fill: parent
            anchors.topMargin: 12
            anchors.bottomMargin: 76
            model: utterances
            clip: true
            spacing: 4
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar { policy: ScrollBar.AlwaysOff }

            delegate: Item {
                id: card
                required property int index
                required property string content
                required property string voicebankId
                required property string imagePath

                width: Math.max(0, utteranceList.width)
                height: 46

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 12
                    anchors.rightMargin: 12
                    spacing: 6

                    Rectangle {
                        id: imageHandle
                        Layout.preferredWidth: 42
                        Layout.preferredHeight: 42
                        radius: 2
                        color: window.palette.alternateBase
                        border.color: card.index === window.selectedIndex ? window.accent : window.borderColor

                        Image {
                            anchors.fill: parent
                            anchors.margins: 2
                            source: window.voicebankImage(card.voicebankId)
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                        }
                        Label {
                            anchors.centerIn: parent
                            visible: !card.imagePath
                            text: window.translator.tr("main.card.icon")
                            color: window.mutedText
                            font.pixelSize: 9
                        }
                    }

                    TextField {
                        id: utteranceEditor
                        Layout.fillWidth: true
                        Layout.preferredHeight: 42
                        text: card.content
                        font.pixelSize: 16
                        placeholderText: window.translator.tr("main.textPlaceholder")
                        selectByMouse: true
                        onActiveFocusChanged: {
                            if (activeFocus)
                                window.selectedIndex = card.index;
                        }
                        onTextChanged: window.updateUtteranceText(card.index, text)
                    }

                    ToolButton {
                        contentItem: Text {
                            anchors.centerIn: parent
                            width: 22
                            height: 22
                            text: "\ue3c9"
                            color: parent.parent.palette.buttonText
                            font.family: iconFont.name
                            font.pixelSize: 20
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                        onClicked: {
                            window.selectedIndex = card.index;
                            window.openUtteranceSheet(card.index);
                        }
                    }
                }
            }
        }

        RoundButton {
            id: addButton
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.rightMargin: 24
            anchors.bottomMargin: 88
            width: 52
            height: 52
            highlighted: true
            z: 2
            contentItem: Text {
                anchors.centerIn: parent
                text: "\ue145"
                color: addButton.palette.buttonText
                font.family: iconFont.name
                font.pixelSize: 28
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
            onClicked: window.addUtterance()
        }

        RowLayout {
            id: bottomBar
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: 12
            spacing: 8

            PlaybackControls {
                Layout.fillWidth: true
                translator: window.translator
                mutedText: window.mutedText
                busy: window.appBackend.busy
                playing: player.playbackState === MediaPlayer.PlayingState
                hasAudio: window.appBackend.previewUrl.toString().length > 0
                canGenerate: window.currentUtterance() !== null
                             && String(window.currentUtterance().content).trim().length > 0
                position: player.position
                duration: player.duration
                errorText: window.appBackend.error
                onPrimaryClicked: window.togglePlayback()
                onSeekRequested: position => { player.position = position; }
            }
        }
    }

    BusyIndicator {
        anchors.centerIn: parent
        running: window.appBackend.busy
        visible: running
        z: 10
    }

    // --- ナビゲーション ---

    Drawer {
        id: menuDrawer
        edge: Qt.LeftEdge
        width: Math.min(window.width * 0.85, 360)
        modal: true
        background: Rectangle {
            color: window.palette.window
        }

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("menu.file.addVoicebank")
                enabled: !window.appBackend.busy
                onClicked: {
                    menuDrawer.close();
                    window.appBackend.beginAddVoicebanks();
                }
            }
            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("menu.settings")
                onClicked: {
                    menuDrawer.close();
                    settingsDialog.open();
                }
            }
            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("dictionary.title")
                onClicked: {
                    menuDrawer.close();
                    dictionaryContent.loadCurrent();
                    dictionaryDialog.open();
                }
            }
            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("menu.help.license")
                onClicked: {
                    menuDrawer.close();
                    licenseDialog.open();
                }
            }
            Button {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                text: window.translator.tr("onboarding.title")
                onClicked: {
                    menuDrawer.close();
                    onboardingDialog.open();
                }
            }
            Item { Layout.fillHeight: true }
        }
    }

    // --- 発話ごとの設定/イントネーション ---

    Dialog {
        id: utteranceSheet
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Overlay.overlay.width
        height: Overlay.overlay.height
        property int editIndex: -1
        header: WindowHeader {
            heading: window.translator.tr("menu.settings")
            onCloseClicked: utteranceSheet.close()
        }

        contentItem: ScrollView {
            contentWidth: availableWidth
            ColumnLayout {
                width: utteranceSheet.width
                spacing: 12

                Label { text: window.translator.tr("main.param.voicebank"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.voicebankOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.voicebankOptions(),
                                                   utteranceSheet.itemValue("voicebankId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "voicebankId", currentValue)
                }

                Label { text: window.translator.tr("settings.defaultModel"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.modelOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.modelOptions(),
                                                   utteranceSheet.itemValue("modelId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "modelId", currentValue)
                }

                Label { text: window.translator.tr("main.param.renderer"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.rendererOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.rendererOptions(),
                                                   utteranceSheet.itemValue("rendererId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "rendererId", currentValue)
                }

                Label { text: window.translator.tr("main.param.tone"); font.bold: true }
                TextField {
                    Layout.fillWidth: true
                    text: utteranceSheet.itemValue("tone", "")
                    onEditingFinished: window.setUtteranceSetting(utteranceSheet.editIndex, "tone", text)
                }

                Label { text: window.translator.tr("main.param.aliasPolicy"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: ["auto", "cvvc-enhanced", "vcv-prefer", "cvvc-prefer", "cv-only"]
                    currentIndex: Math.max(0, model.indexOf(utteranceSheet.itemValue("aliasPolicy", "auto")))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "aliasPolicy", currentText)
                }

                Label { text: window.translator.tr("main.param.intonation"); font.bold: true }
                RowLayout {
                    Layout.fillWidth: true
                    Slider {
                        id: intonationSlider
                        Layout.fillWidth: true
                        from: 0
                        to: 4.0
                        value: Number(utteranceSheet.itemValue("intonation", window.appBackend.defaultIntonationStrength))
                        onMoved: window.setUtteranceSetting(utteranceSheet.editIndex, "intonation", value)
                    }
                    Label { text: intonationSlider.value.toFixed(1) }
                }

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8
                    Button {
                        Layout.fillWidth: true
                        text: window.translator.tr("main.card.moveUp")
                        onClicked: window.moveUtterance(utteranceSheet.editIndex, -1)
                    }
                    Button {
                        Layout.fillWidth: true
                        text: window.translator.tr("main.card.moveDown")
                        onClicked: window.moveUtterance(utteranceSheet.editIndex, 1)
                    }
                }
                Button {
                    Layout.fillWidth: true
                    text: window.translator.tr("main.card.delete")
                    onClicked: {
                        window.removeUtterance(utteranceSheet.editIndex);
                        utteranceSheet.close();
                    }
                }
            }
        }
    }

    // --- グローバル設定 ---

    Dialog {
        id: settingsDialog
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Overlay.overlay.width
        height: Overlay.overlay.height
        header: WindowHeader {
            heading: window.translator.tr("menu.settings")
            onCloseClicked: settingsDialog.close()
        }

        contentItem: Item {
            ScrollView {
                id: settingsScroll
                anchors.fill: parent
                anchors.margins: 12
                contentWidth: availableWidth
                ColumnLayout {
                    width: settingsScroll.availableWidth
                    spacing: 12

                Label { text: window.translator.tr("settings.language"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.languageOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.languageOptions(), window.language)
                    onActivated: window.appBackend.setLanguage(currentValue)
                }

                Switch {
                    text: window.translator.tr("settings.theme.dark")
                    checked: window.darkMode
                    onToggled: window.appBackend.setDarkMode(checked)
                }

                CheckBox {
                    text: window.translator.tr("settings.exportTextWithWav")
                    checked: window.exportTextWithWav
                    onToggled: window.exportTextWithWav = checked
                }
                CheckBox {
                    text: window.translator.tr("settings.exportLabWithWav")
                    checked: window.exportLabWithWav
                    onToggled: window.exportLabWithWav = checked
                }
                ComboBox {
                    Layout.fillWidth: true
                    model: ["utf-8", "shift_jis"]
                    currentIndex: window.exportTextEncoding === "shift_jis" ? 1 : 0
                    onActivated: window.exportTextEncoding = currentText
                }
            }
        }
        }

        onClosed: window.applyExportSettings()
    }

    // --- 共有ビュー（辞書 / ライセンス / 初回案内） ---

    Dialog {
        id: dictionaryDialog
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Overlay.overlay.width
        height: Overlay.overlay.height
        header: WindowHeader {
            heading: window.translator.tr("dictionary.title")
            onCloseClicked: dictionaryDialog.close()
        }
        contentItem: Item {
            DictionaryContent {
                id: dictionaryContent
                anchors.fill: parent
                anchors.margins: 8
                backend: window.appBackend
                translator: window.translator
                hostWindow: window
                onCloseRequested: dictionaryDialog.close()
            }
        }
    }

    Dialog {
        id: licenseDialog
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Overlay.overlay.width
        height: Overlay.overlay.height
        header: WindowHeader {
            heading: window.translator.tr("menu.help.license")
            onCloseClicked: licenseDialog.close()
        }
        contentItem: Item {
            LicenseContent {
                anchors.fill: parent
                anchors.margins: 8
                documents: window.licenseDocuments
            }
        }
    }

    Dialog {
        id: onboardingDialog
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Overlay.overlay.width
        height: Overlay.overlay.height
        header: WindowHeader {
            heading: window.translator.tr("onboarding.title")
            onCloseClicked: onboardingDialog.close()
        }
        contentItem: Item {
            OnboardingContent {
                anchors.fill: parent
                anchors.margins: 12
                backend: window.appBackend
                translator: window.translator
            }
        }
    }

    // --- ヘルパー ---

    function reanalyzeAll() {
        // モバイルは発話単位の解析UIを持たないため何もしない。
    }

    function currentUtterance() {
        return window.selectedIndex >= 0 && window.selectedIndex < utterances.count
                ? utterances.get(window.selectedIndex) : null;
    }

    function voicebankImage(voicebankId) {
        const voice = core.voicebankById(voicebankId);
        return voice && voice.image_path ? window.appBackend.localFileUrl(voice.image_path) : "";
    }

    function voicebankOptions() {
        return window.appBackend.voicebanks.map(voice => ({id: voice.id, label: voice.name}));
    }

    function modelOptions() {
        return window.appBackend.models.map(model =>
            ({id: model.id, label: String(model.display_name || model.id)}));
    }

    function rendererOptions() {
        return window.appBackend.renderers.map(renderer =>
            ({id: renderer.id, label: String(renderer.display_name || renderer.id)}));
    }

    function languageOptions() {
        return window.appBackend.languageCodes().map(code => ({
            id: code,
            label: code === "auto" ? window.translator.tr("settings.language.auto")
                                   : window.appBackend.languageDisplayName(code)
        }));
    }

    function indexOfId(options, id) {
        for (let index = 0; index < options.length; ++index) {
            if (options[index].id === id)
                return index;
        }
        return 0;
    }

    function loadExportSettings() {
        window.exportTextWithWav = window.appBackend.exportTextWithWav;
        window.exportLabWithWav = window.appBackend.exportLabWithWav;
        window.exportTextEncoding = window.appBackend.exportTextEncoding;
    }

    function applyExportSettings() {
        window.appBackend.setExportSettings(window.exportTextWithWav,
                                            window.exportLabWithWav,
                                            window.exportTextEncoding);
    }

    function syncFromBackend() {
        if (!utterances.count)
            return;
        const selected = window.currentUtterance();
        if (!selected)
            return;
        if (!selected.voicebankId) {
            const voice = core.defaultVoicebank();
            utterances.setProperty(window.selectedIndex, "voicebankId", voice ? voice.id : "");
            utterances.setProperty(window.selectedIndex, "imagePath", voice ? (voice.image_path || "") : "");
        }
    }

    function selectedLanguage() {
        return window.language === "auto" ? "ja" : window.language;
    }

    function defaultUtterance() {
        const voice = core.defaultVoicebank();
        const language = window.selectedLanguage();
        return {
            content: "",
            voicebankId: voice ? voice.id : "",
            imagePath: voice ? (voice.image_path || "") : "",
            modelId: core.defaultModelIdForLanguage(language),
            rendererId: core.defaultRendererId(),
            tone: window.appBackend.defaultTone,
            aliasPolicy: core.normalizeAliasPolicy(window.appBackend.defaultAliasPolicy),
            intonation: window.appBackend.defaultIntonationStrength,
        };
    }

    function addUtterance() {
        utterances.append(window.defaultUtterance());
        window.selectedIndex = utterances.count - 1;
    }

    function updateUtteranceText(index, text) {
        if (index < 0 || index >= utterances.count)
            return;
        if (utterances.get(index).content === text)
            return;
        utterances.setProperty(index, "content", text);
    }

    function removeUtterance(index) {
        if (index < 0 || index >= utterances.count)
            return;
        utterances.remove(index);
        if (utterances.count === 0)
            window.addUtterance();
        window.selectedIndex = Math.max(0, Math.min(window.selectedIndex, utterances.count - 1));
    }

    function moveUtterance(index, delta) {
        const target = index + delta;
        if (index < 0 || index >= utterances.count || target < 0 || target >= utterances.count)
            return;
        utterances.move(index, target, 1);
        window.selectedIndex = target;
    }

    function openUtteranceSheet(index) {
        utteranceSheet.editIndex = index;
        utteranceSheet.open();
    }

    function itemValue(role, fallback) {
        const index = utteranceSheet.editIndex;
        if (index < 0 || index >= utterances.count)
            return fallback;
        const value = utterances.get(index)[role];
        return value === undefined || value === null ? fallback : value;
    }

    function setUtteranceSetting(index, role, value) {
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, role, value);
        if (role === "voicebankId") {
            const voice = core.voicebankById(value);
            utterances.setProperty(index, "imagePath", voice ? (voice.image_path || "") : "");
        }
    }

    function togglePlayback() {
        if (window.appBackend.busy)
            return;
        if (player.playbackState === MediaPlayer.PlayingState) {
            player.stop();
            return;
        }
        window.synthesizeSelected();
    }

    function synthesizeSelected() {
        const item = window.currentUtterance();
        if (!item || !String(item.content).trim().length)
            return;
        const language = window.selectedLanguage();
        window.playAfterSynthesize = true;
        window.appBackend.synthesize({
            text: item.content,
            language: language,
            phonemizer: core.resolvedPhonemizer(language, "auto", item.voicebankId),
            voicebank_id: item.voicebankId,
            model_id: item.modelId,
            renderer: item.rendererId,
            alias_policy: core.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone,
            color: "",
            mora_duration_ms: window.appBackend.defaultMoraDuration,
            pause_duration_ms: window.appBackend.defaultPauseDuration,
            intonation_strength: item.intonation,
            apply_pitch: true,
            dictionary: window.appBackend.dictionaryEntries,
        });
    }
}
