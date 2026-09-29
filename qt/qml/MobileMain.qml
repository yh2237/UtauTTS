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
    onSelectedIndexChanged: window.refreshEditorInputs()
    property string language: "auto"
    property bool exportTextWithWav: false
    property bool exportLabWithWav: false
    property string exportTextEncoding: "utf-8"
    property bool playAfterSynthesize: false
    property string lastRequestKey: ""
    property var synthesisUnits: []
    property int editorRevision: 0
    property string pendingProsodyRequestId: ""
    property int pendingProsodyIndex: -1

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

    Timer {
        id: analysisTimer
        interval: 500
        onTriggered: window.ensureSelectionAnalyzed()
    }

    Connections {
        target: window.appBackend
        function onPreviewReady() {
            player.stop();
            player.source = window.appBackend.previewUrl;
            try {
                const result = JSON.parse(window.appBackend.synthesisJson);
                window.synthesisUnits = result.units || [];
            } catch (error) {
                window.synthesisUnits = [];
            }
            window.refreshEditorInputs();
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
        function onProsodyChanged() {
            window.applyProsodyResult();
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

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            ListView {
                id: utteranceList
                anchors.fill: parent
                anchors.topMargin: 12
                anchors.bottomMargin: 8
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
                            if (activeFocus) {
                                window.selectedIndex = card.index;
                                window.ensureSelectionAnalyzed();
                            }
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
                            window.ensureSelectionAnalyzed();
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

        }

        // --- イントネーションエディタ（デスクトップのピッチペイン相当） ---

        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 280

            Component.onCompleted: window.refreshEditorInputs()

            ButtonGroup {
                id: mPitchTabMode
                exclusive: true
            }
            ButtonGroup {
                id: mPitchToolMode
                exclusive: true
            }

            Item {
                anchors.fill: parent
                anchors.leftMargin: 12
                anchors.rightMargin: 12
                anchors.bottomMargin: 8

                Item {
                    id: mPitchModeTabs
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.topMargin: 1
                    width: mBasicPitchTab.width + mExtendedPitchTab.width
                    height: 35
                    z: 3
                    readonly property int currentIndex: mExtendedPitchTab.checked ? 1 : 0

                    Row {
                        anchors.fill: parent
                        spacing: 0
                        ToolButton {
                            id: mBasicPitchTab
                            width: Math.max(96, mBasicPitchLabel.implicitWidth + 24)
                            height: parent.height
                            ButtonGroup.group: mPitchTabMode
                            checkable: true
                            checked: true
                            text: window.translator.tr("main.pitch.basic")
                            background: Rectangle {
                                color: mBasicPitchTab.checked ? window.palette.base
                                       : mBasicPitchTab.hovered ? Qt.rgba(window.palette.alternateBase.r, window.palette.alternateBase.g, window.palette.alternateBase.b, 0.42)
                                                                : "transparent"
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    height: 1
                                    color: mBasicPitchTab.checked ? window.palette.base : "transparent"
                                }
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 1
                                    height: 18
                                    color: window.borderColor
                                }
                            }
                            contentItem: Text {
                                id: mBasicPitchLabel
                                anchors.centerIn: parent
                                text: mBasicPitchTab.text
                                color: mBasicPitchTab.checked ? window.palette.text : window.palette.placeholderText
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                        ToolButton {
                            id: mExtendedPitchTab
                            width: Math.max(96, mExtendedPitchLabel.implicitWidth + 24)
                            height: parent.height
                            ButtonGroup.group: mPitchTabMode
                            checkable: true
                            text: window.translator.tr("main.pitch.extended")
                            background: Rectangle {
                                color: mExtendedPitchTab.checked ? window.palette.base
                                       : mExtendedPitchTab.hovered ? Qt.rgba(window.palette.alternateBase.r, window.palette.alternateBase.g, window.palette.alternateBase.b, 0.42)
                                                                   : "transparent"
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    height: 1
                                    color: mExtendedPitchTab.checked ? window.palette.base : "transparent"
                                }
                            }
                            contentItem: Text {
                                id: mExtendedPitchLabel
                                anchors.centerIn: parent
                                text: mExtendedPitchTab.text
                                color: mExtendedPitchTab.checked ? window.palette.text : window.palette.placeholderText
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                    }
                }

                Rectangle {
                    id: mPitchToolGroupFrame
                    visible: mPitchModeTabs.currentIndex === 1
                    anchors.left: mPitchModeTabs.right
                    anchors.top: parent.top
                    anchors.leftMargin: 12
                    anchors.topMargin: 1
                    width: visible ? 65 : 0
                    height: 35
                    z: 3
                    color: "transparent"

                    RowLayout {
                        anchors.fill: parent
                        spacing: 0
                        ToolButton {
                            id: mHandTool
                            ButtonGroup.group: mPitchToolMode
                            checkable: true
                            checked: true
                            Layout.preferredWidth: 32
                            Layout.fillHeight: true
                            background: Rectangle {
                                radius: 3
                                color: mHandTool.checked ? Qt.rgba(window.accent.r, window.accent.g, window.accent.b, 0.16)
                                       : mHandTool.hovered ? Qt.rgba(window.palette.mid.r, window.palette.mid.g, window.palette.mid.b, 0.18)
                                                           : "transparent"
                            }
                            contentItem: Text {
                                anchors.centerIn: parent
                                text: "\ue925"
                                color: mHandTool.checked ? window.accent : mHandTool.palette.buttonText
                                font.family: iconFont.name
                                font.pixelSize: 16
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                        Rectangle {
                            Layout.preferredWidth: 1
                            Layout.preferredHeight: 14
                            Layout.alignment: Qt.AlignVCenter
                            color: window.borderColor
                        }
                        ToolButton {
                            id: mPenTool
                            ButtonGroup.group: mPitchToolMode
                            checkable: true
                            Layout.preferredWidth: 32
                            Layout.fillHeight: true
                            background: Rectangle {
                                radius: 3
                                color: mPenTool.checked ? Qt.rgba(window.accent.r, window.accent.g, window.accent.b, 0.16)
                                       : mPenTool.hovered ? Qt.rgba(window.palette.mid.r, window.palette.mid.g, window.palette.mid.b, 0.18)
                                                          : "transparent"
                            }
                            contentItem: Text {
                                anchors.centerIn: parent
                                text: "\ue3c9"
                                color: mPenTool.checked ? window.accent : mPenTool.palette.buttonText
                                font.family: iconFont.name
                                font.pixelSize: 16
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                    }
                }

                IntonationEditorSurface {
                    id: mPitchEditorSurface
                    anchors.fill: parent
                    surfaceColor: window.palette.base
                    borderColor: window.borderColor
                    contentMargin: 8
                    topContentMargin: 44
                    showSideBorders: false

                    StackLayout {
                        id: mPitchModeStack
                        anchors.fill: parent
                        currentIndex: mPitchModeTabs.currentIndex

                        ColumnLayout {
                            spacing: 0
                            PitchEditor {
                                id: mPitchEditor
                                Layout.fillWidth: true
                                Layout.fillHeight: true
                                translator: window.translator
                                accentColor: window.accent
                                axisColor: window.palette.mid
                                gridColor: window.palette.alternateBase
                                labelColor: window.palette.text
                                defaultMoraDuration: window.appBackend.defaultMoraDuration
                                defaultPauseDuration: window.appBackend.defaultPauseDuration
                                points: []
                                morae: []
                                moraDurations: []
                                moraPositions: []
                                autoPoints: []
                                onPointsEdited: points => window.updateIntonationPoints(points)
                                onTimingEdited: (durations, positions) => window.updateIntonationTiming(durations, positions)
                            }
                            Item {
                                visible: mBasicPitchScrollBar.visible
                                Layout.fillWidth: true
                                Layout.preferredHeight: visible ? 18 : 0
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    height: 1
                                    color: window.borderColor
                                }
                                PitchHorizontalScrollBar {
                                    id: mBasicPitchScrollBar
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    anchors.bottomMargin: 2
                                    editor: mPitchEditor
                                    trackColor: window.palette.mid
                                    thumbColor: window.accent
                                }
                            }
                        }

                        PhonemeEditor {
                            id: mPhonemeEditor
                            translator: window.translator
                            accentColor: window.accent
                            axisColor: window.palette.mid
                            gridColor: window.palette.alternateBase
                            labelColor: window.palette.text
                            mutedText: window.mutedText
                            dividerColor: window.borderColor
                            showTimelineFrame: false
                            timingEditor: mPitchEditor
                            units: []
                            waveformMin: []
                            waveformMax: []
                            waveformDuration: 0
                            leadingMargin: 0
                            morae: mPitchEditor.morae
                            moraDurations: mPitchEditor.moraDurations
                            moraPositions: mPitchEditor.moraPositions
                            overrides: []
                            manualFrames: []
                            autoFrames: []
                            frameMs: 10
                            playbackMs: player.playbackState === MediaPlayer.PlayingState ? player.position : -1
                            showDetails: window.appBackend.extendedDetailsVisible
                            framePaintMode: mPenTool.checked && mPitchModeTabs.currentIndex === 1
                            onUnitValueEdited: (unitIndex, key, value) => window.updateIntonationUnitOverride(unitIndex, key, value)
                            onMoraStartEdited: (position, startMs) => window.updateIntonationMoraStart(position, startMs)
                            onMoraDurationEdited: (position, durationMs) => window.updateIntonationMoraDuration(position, durationMs)
                            onNoteGestureEdited: (durations, positions, points) => window.updateIntonationGesture(durations, positions, points)
                            onResetUnitRequested: unitIndex => window.clearIntonationUnitOverride(unitIndex)
                            onSeekRequested: positionMs => { player.position = positionMs; }
                            onFramesEdited: frames => window.updateIntonationFrames(frames)
                        }
                    }
                }
            }
        }

        PlaybackControls {
            Layout.fillWidth: true
            Layout.margins: 12
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

        contentItem: Item {
            ScrollView {
                id: utteranceScroll
                anchors.fill: parent
                anchors.margins: 12
                contentWidth: availableWidth
                ColumnLayout {
                    width: utteranceScroll.availableWidth
                    spacing: 12

                Label { text: window.translator.tr("main.param.voicebank"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.voicebankOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.voicebankOptions(),
                                                   window.itemValue("voicebankId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "voicebankId", currentValue)
                }

                Label { text: window.translator.tr("settings.defaultModel"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.modelOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.modelOptions(),
                                                   window.itemValue("modelId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "modelId", currentValue)
                }

                Label { text: window.translator.tr("main.param.renderer"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: window.rendererOptions()
                    textRole: "label"
                    valueRole: "id"
                    currentIndex: window.indexOfId(window.rendererOptions(),
                                                   window.itemValue("rendererId", ""))
                    onActivated: window.setUtteranceSetting(utteranceSheet.editIndex, "rendererId", currentValue)
                }

                Label { text: window.translator.tr("main.param.tone"); font.bold: true }
                TextField {
                    Layout.fillWidth: true
                    text: window.itemValue("tone", "")
                    onEditingFinished: window.setUtteranceSetting(utteranceSheet.editIndex, "tone", text)
                }

                Label { text: window.translator.tr("main.param.aliasPolicy"); font.bold: true }
                ComboBox {
                    Layout.fillWidth: true
                    model: ["auto", "cvvc-enhanced", "vcv-prefer", "cvvc-prefer", "cv-only"]
                    currentIndex: Math.max(0, model.indexOf(window.itemValue("aliasPolicy", "auto")))
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
                        value: Number(window.itemValue("intonation", window.appBackend.defaultIntonationStrength))
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
                anchors.margins: 12
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
                anchors.margins: 12
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

    function selectedRole(role) {
        const item = window.currentUtterance();
        if (!item)
            return [];
        const value = item[role];
        return value === undefined || value === null ? [] : value;
    }

    function editorUnits() {
        if (window.synthesisUnits.length)
            return window.synthesisUnits;
        return window.intonationUnits(
            window.decode(window.selectedRole("moraeJson")),
            window.decode(window.selectedRole("durationsJson")),
            window.decode(window.selectedRole("positionsJson")));
    }

    function refreshEditorInputs() {
        if (!mPitchEditor || !mPhonemeEditor)
            return;
        mPitchEditor.points = window.decode(window.selectedRole("pointsJson"));
        mPitchEditor.morae = window.decode(window.selectedRole("moraeJson"));
        mPitchEditor.moraDurations = window.decode(window.selectedRole("durationsJson"));
        mPitchEditor.moraPositions = window.decode(window.selectedRole("positionsJson"));
        mPitchEditor.autoPoints = window.decode(window.selectedRole("autoPointsJson"));
        mPhonemeEditor.overrides = window.decode(window.selectedRole("phonemeOverridesJson"));
        mPhonemeEditor.units = window.editorUnits();
        mPhonemeEditor.manualFrames = window.decode(window.selectedRole("framePitchJson"));
        mPhonemeEditor.autoFrames = window.decode(window.selectedRole("autoFramePitchJson"));
        mPhonemeEditor.frameMs = Number(window.selectedRole("frameMs")) || 10;
        mPitchEditor.refresh();
        // wasm ではシグナル処理中の Canvas 再描画が反映されないことがあるため、
        // 次のイベントループでもう一度描画する。
        Qt.callLater(function() {
            mPitchEditor.refresh();
        });
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
        const language = window.selectedLanguage();
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            let changed = false;
            if (!item.voicebankId) {
                const voice = core.defaultVoicebank();
                utterances.setProperty(index, "voicebankId", voice ? voice.id : "");
                utterances.setProperty(index, "imagePath", voice ? (voice.image_path || "") : "");
                changed = true;
            }
            if (!item.modelId || item.modelId === "none") {
                const model = core.defaultModelIdForLanguage(language);
                if (model && model !== "none") {
                    utterances.setProperty(index, "modelId", model);
                    changed = true;
                }
            }
            if (!item.rendererId)
                utterances.setProperty(index, "rendererId", core.defaultRendererId());
            // 解析結果は音源/モデルに依存するため、既定値が変わったらやり直す。
            if (changed)
                window.resetAnalysis(index);
        }
        window.refreshEditorInputs();
        window.ensureSelectionAnalyzed();
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
            reading: "",
            moraeJson: "[]",
            pointsJson: "[]",
            durationsJson: "[]",
            positionsJson: "[]",
            autoPointsJson: "[]",
            autoDurationsJson: "[]",
            autoPositionsJson: "[]",
            framePitchJson: "[]",
            autoFramePitchJson: "[]",
            phonemeOverridesJson: "[]",
            frameMs: 10,
            manualPitchEdited: false,
            manualTimingEdited: false,
        };
    }

    function addUtterance() {
        utterances.append(window.defaultUtterance());
        window.selectedIndex = utterances.count - 1;
        window.editorRevision += 1;
    }

    function resetAnalysis(index) {
        if (index < 0 || index >= utterances.count)
            return;
        const fields = ["reading", "moraeJson", "pointsJson", "durationsJson", "positionsJson",
                        "autoPointsJson", "autoDurationsJson", "autoPositionsJson",
                        "autoFramePitchJson", "framePitchJson", "phonemeOverridesJson"];
        for (let i = 0; i < fields.length; ++i)
            utterances.setProperty(index, fields[i], fields[i] === "reading" ? "" : "[]");
        utterances.setProperty(index, "manualPitchEdited", false);
        utterances.setProperty(index, "manualTimingEdited", false);
    }

    function updateUtteranceText(index, text) {
        if (index < 0 || index >= utterances.count)
            return;
        if (utterances.get(index).content === text)
            return;
        utterances.setProperty(index, "content", text);
        window.resetAnalysis(index);
        window.refreshEditorInputs();
        window.invalidateAudio();
        analysisTimer.restart();
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
        if (role === "voicebankId" || role === "modelId" || role === "aliasPolicy")
            window.resetAnalysis(index);
        window.refreshEditorInputs();
        window.invalidateAudio();
    }

    function togglePlayback() {
        if (window.appBackend.busy)
            return;
        if (player.playbackState === MediaPlayer.PlayingState) {
            player.stop();
            return;
        }
        const item = window.currentUtterance();
        if (!item || !String(item.content).trim().length)
            return;
        // 音声キャッシュは Backend 側がリクエスト単位で判定する。
        // ここでは常に合成を要求する（内容が同じなら Backend がキャッシュを返す）。
        window.ensureSelectionAnalyzed();
        window.playAfterSynthesize = true;
        window.appBackend.synthesize(window.buildRequest(item));
    }

    function decode(value) {
        if (Array.isArray(value))
            return value;
        if (!value)
            return [];
        try {
            return JSON.parse(value) || [];
        } catch (error) {
            return [];
        }
    }

    function encode(value) {
        return JSON.stringify(value || []);
    }

    function invalidateAudio() {
        window.lastRequestKey = "";
        window.synthesisUnits = [];
        window.editorRevision += 1;
    }

    function buildRequest(item) {
        const language = window.selectedLanguage();
        const morae = window.decode(item.moraeJson);
        const points = window.decode(item.pointsJson);
        const durations = window.decode(item.durationsJson);
        const request = {
            text: item.content,
            reading: item.reading || "",
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
        };
        if (item.manualTimingEdited && durations.length)
            request.mora_durations_ms = durations;
        const overrides = window.decode(item.phonemeOverridesJson);
        if (overrides.length && overrides.some(value => value && Object.keys(value).length))
            request.unit_overrides = overrides;
        const frames = window.decode(item.framePitchJson);
        if (item.manualPitchEdited && frames.some(value => Math.abs(Number(value)) > 0.1)) {
            request.manual_pitch = {
                version: 1,
                reading: item.reading || "",
                mode: "frames",
                frames: frames.map(value => Math.max(-1200, Math.min(1200, Number(value) || 0))),
            };
        } else if (item.manualPitchEdited && points.some(value => Math.abs(Number(value)) > 0.1)) {
            const manualPoints = [];
            for (let index = 0; index < points.length; ++index) {
                const mora = index < morae.length ? morae[index] : null;
                if (mora && mora.pause)
                    continue;
                manualPoints.push({
                    position: index,
                    mora: mora ? (mora.mora || "") : "",
                    cents: Number(points[index]) || 0,
                });
            }
            request.manual_pitch = {
                version: 1,
                reading: item.reading || "",
                mode: "offset",
                points: manualPoints,
            };
        }
        return request;
    }

    function buildProsodyRequest(item, requestId) {
        const request = {
            request_id: requestId,
            text: item.content,
            reading: item.reading || "",
            language: window.selectedLanguage(),
            phonemizer: core.resolvedPhonemizer(window.selectedLanguage(), "auto", item.voicebankId),
            dictionary: window.appBackend.dictionaryEntries,
            model_id: item.modelId,
            renderer: item.rendererId,
            mora_duration_ms: window.appBackend.defaultMoraDuration,
            pause_duration_ms: window.appBackend.defaultPauseDuration,
            intonation_strength: item.intonation,
            apply_pitch: true,
        };
        return request;
    }

    function ensureSelectionAnalyzed() {
        const index = window.selectedIndex;
        const item = index >= 0 && index < utterances.count ? utterances.get(index) : null;
        if (index < 0 || index >= utterances.count)
            return;
        if (!String(item.content).trim().length)
            return;
        if (window.decode(item.moraeJson).length)
            return;
        const requestId = "mobile-prosody-" + index + "-" + Date.now();
        window.pendingProsodyRequestId = requestId;
        window.pendingProsodyIndex = index;
        window.appBackend.predictProsody(window.buildProsodyRequest(item, requestId));
    }

    function applyProsodyResult() {
        if (window.appBackend.prosodyRequestId !== window.pendingProsodyRequestId)
            return;
        const index = window.pendingProsodyIndex;
        window.pendingProsodyIndex = -1;
        if (index < 0 || index >= utterances.count)
            return;
        let result;
        try {
            result = JSON.parse(window.appBackend.prosodyJson);
        } catch (error) {
            return;
        }
        const morae = result.morae || [];
        const autoPoints = result.pitch_points || [];
        const durations = result.mora_durations_ms || [];
        const starts = core.moraStartsFromCenters(result.mora_positions_ms || [], durations);
        const manualPoints = [];
        for (let i = 0; i < morae.length; ++i)
            manualPoints.push(0);
        utterances.setProperty(index, "reading", String(result.reading || ""));
        utterances.setProperty(index, "moraeJson", window.encode(morae));
        utterances.setProperty(index, "pointsJson", window.encode(manualPoints));
        utterances.setProperty(index, "durationsJson", window.encode(durations));
        utterances.setProperty(index, "positionsJson", window.encode(starts));
        utterances.setProperty(index, "autoPointsJson", window.encode(autoPoints));
        utterances.setProperty(index, "autoDurationsJson", window.encode(durations));
        utterances.setProperty(index, "autoPositionsJson", window.encode(starts));
        utterances.setProperty(index, "autoFramePitchJson", window.encode(result.frame_pitch_cents || []));
        utterances.setProperty(index, "framePitchJson", "[]");
        utterances.setProperty(index, "frameMs", Number(result.frame_ms) || 10);
        utterances.setProperty(index, "manualPitchEdited", false);
        utterances.setProperty(index, "manualTimingEdited", false);
        window.refreshEditorInputs();
        window.invalidateAudio();
    }

    function updateIntonationPoints(points) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "pointsJson", window.encode(points));
        utterances.setProperty(index, "manualPitchEdited", true);
        window.invalidateAudio();
    }

    function updateIntonationTiming(durations, positions) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "durationsJson", window.encode(durations));
        utterances.setProperty(index, "positionsJson", window.encode(positions));
        utterances.setProperty(index, "manualTimingEdited", true);
        window.invalidateAudio();
    }

    function intonationUnits(morae, durations, positions) {
        const defaultMoraDuration = window.appBackend.defaultMoraDuration;
        const defaultPauseDuration = window.appBackend.defaultPauseDuration;
        const source = morae || [];
        const durationValues = durations || [];
        const positionValues = positions || [];
        const hasPositions = positionValues.length >= source.length
                && source.every((value, index) => Number.isFinite(Number(positionValues[index])));
        const units = [];
        let fallbackStart = 0;
        for (let index = 0; index < source.length; ++index) {
            const mora = source[index] || {};
            const pause = !!mora.pause;
            const defaultDuration = Math.max(20, Number(pause
                    ? defaultPauseDuration : defaultMoraDuration) || 120);
            const start = hasPositions ? Math.max(0, Number(positionValues[index])) : fallbackStart;
            let duration = defaultDuration;
            if (hasPositions && index + 1 < source.length)
                duration = Math.max(20, Number(positionValues[index + 1]) - start);
            else if (!hasPositions && Number.isFinite(Number(durationValues[index]))
                     && Number(durationValues[index]) > 0)
                duration = Math.max(20, Number(durationValues[index]));
            const text = String(mora.mora || "");
            units.push({
                position: index,
                role: pause ? "pause" : "mora",
                mora: text,
                alias: text,
                note_start_ms: start,
                duration_ms: duration,
                silent: pause,
            });
            fallbackStart = Math.max(fallbackStart, start + duration);
        }
        return units;
    }

    function updateIntonationUnitOverride(unitIndex, key, value) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        const overrides = window.decode(utterances.get(index).phonemeOverridesJson);
        overrides[unitIndex] = overrides[unitIndex] || {};
        overrides[unitIndex][key] = value;
        utterances.setProperty(index, "phonemeOverridesJson", window.encode(overrides));
        window.invalidateAudio();
        // バー表示へ即時反映する（overrides 代入で再描画される）。
        if (mPhonemeEditor)
            mPhonemeEditor.overrides = window.decode(utterances.get(index).phonemeOverridesJson);
    }

    function clearIntonationUnitOverride(unitIndex) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        const overrides = window.decode(utterances.get(index).phonemeOverridesJson);
        delete overrides[unitIndex];
        utterances.setProperty(index, "phonemeOverridesJson", window.encode(overrides));
        window.invalidateAudio();
    }

    function updateIntonationMoraStart(position, startMs) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        const positions = window.decode(utterances.get(index).positionsJson);
        const durations = window.decode(utterances.get(index).durationsJson);
        positions[position] = Math.max(0, startMs);
        if (position + 1 < positions.length)
            durations[position] = Math.max(20, Number(positions[position + 1]) - Number(positions[position]));
        utterances.setProperty(index, "positionsJson", window.encode(positions));
        utterances.setProperty(index, "durationsJson", window.encode(durations));
        utterances.setProperty(index, "manualTimingEdited", true);
        window.invalidateAudio();
    }

    function updateIntonationMoraDuration(position, durationMs) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        const positions = window.decode(utterances.get(index).positionsJson);
        const durations = window.decode(utterances.get(index).durationsJson);
        durations[position] = Math.max(20, durationMs);
        if (position + 1 < positions.length)
            positions[position + 1] = Number(positions[position] || 0) + Number(durations[position]);
        utterances.setProperty(index, "positionsJson", window.encode(positions));
        utterances.setProperty(index, "durationsJson", window.encode(durations));
        utterances.setProperty(index, "manualTimingEdited", true);
        window.invalidateAudio();
    }

    function updateIntonationGesture(durations, positions, points) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "durationsJson", window.encode(durations));
        utterances.setProperty(index, "positionsJson", window.encode(positions));
        utterances.setProperty(index, "pointsJson", window.encode(points));
        utterances.setProperty(index, "manualTimingEdited", true);
        utterances.setProperty(index, "manualPitchEdited", true);
        window.invalidateAudio();
    }

    function updateIntonationFrames(frames) {
        const index = window.selectedIndex;
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "framePitchJson", window.encode(frames));
        utterances.setProperty(index, "manualPitchEdited", true);
        window.invalidateAudio();
    }

    function synthesizeSelected() {
        const item = window.currentUtterance();
        if (!item || !String(item.content).trim().length)
            return;
        const request = window.buildRequest(item);
        window.playAfterSynthesize = true;
        window.lastRequestKey = JSON.stringify(request);
        window.appBackend.synthesize(request);
    }
}
