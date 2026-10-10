pragma ComponentBehavior: Bound

import QtQuick
import "UnitOverrides.js" as UnitOverrides
import "MoraPositions.js" as MoraPositions
import "QmlCore.js" as QmlCore
import "UtteranceCore.js" as UtteranceCore
import "HistoryCore.js" as HistoryCore
import "IntonationLab.js" as IntonationLab
import "SelfTest.js" as SelfTest
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Platform 1.0
import QtQuick.Dialogs
import UtauTTS.Media

ApplicationWindow {
    id: window
    required property var injectedBackend
    required property var injectedLegalDocuments
    required property string injectedAppName
    required property url injectedRepositoryUrl
    required property bool injectedSelfTest
    required property bool injectedIntonationLab
    required property string injectedIntonationLabExamples
    width: 1240
    height: 850
    minimumWidth: Platform.isWeb ? 0 : 880
    minimumHeight: Platform.isWeb ? 0 : 600
    visible: !injectedSelfTest
    title: intonationLab ? "UtauTTS Intonation Lab" : injectedAppName
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

    property color accent: palette.highlight
    property color borderColor: palette.mid
    property color mutedText: palette.text
    readonly property url repositoryUrl: injectedRepositoryUrl
    readonly property var appBackend: injectedBackend
    readonly property bool darkMode: appBackend.darkMode
    readonly property bool intonationLab: injectedIntonationLab
    readonly property bool mobileLayout: Platform.isMobile && !window.intonationLab
    onWidthChanged: Platform.updateViewportWidth(width)
    onMobileLayoutChanged: {
        if (!mobileLayout && window.showOnboarding && onboardingWindowLoader)
            onboardingWindowLoader.active = true;
        if (mobileLayout && window.menuBar) {
            for (const menu of window.menuBar.menus)
                menu.close();
        }
    }
    readonly property bool showOnboarding: !window.injectedSelfTest && !window.intonationLab
                                             && !window.appBackend.onboardingCompleted
    readonly property var licenseDocuments: injectedLegalDocuments
    readonly property real defaultIntonationStrength: appBackend.defaultIntonationStrength
    readonly property real maxIntonationStrength: 8.0

    property var translator: translatorInstance
    property string updateAvailableVersion: ""
    property string updateReleaseNotes: ""
    property url updateReleaseUrl: ""
    property url updateDownloadUrl: ""
    property bool updateAvailablePreRelease: false
    property double updateDownloadReceived: 0
    property double updateDownloadTotal: 0
    property bool updateSuppressVersion: false
    property bool metadataReloadActive: false
    property string metadataReloadStage: "voicebanks"
    property var synthesisUnits: []
    property var synthesisWaveformMin: []
    property var synthesisWaveformMax: []
    property real synthesisDurationMs: 0
    property real synthesisLeadingMarginMs: 0
    property string synthesisViewUtteranceId: ""
    property int synthesisViewRevision: -1
    property bool synthesisViewStale: false
    property bool autoplayPreview: true
    property bool autoPreviewPending: false

    Timer {
        id: autoPreviewTimer
        interval: 260
        repeat: false
        onTriggered: window.refreshPreview()
    }

    Timer {
        id: intonationLabPrepareTimer
        interval: 100
        repeat: false
        onTriggered: {
            if (!window.intonationLab || window.intonationLabPendingIndex < 0)
                return;
            if (window.appBackend.busy || !window.metadataInitialized) {
                intonationLabPrepareTimer.restart();
                return;
            }
            const index = window.intonationLabPendingIndex;
            window.intonationLabPendingIndex = -1;
            window.prepareIntonationLabEntry(index);
        }
    }

    Timer {
        id: timingProsodyTimer
        interval: 120
        repeat: false
        property string utteranceId: ""
        onTriggered: {
            const index = window.utteranceIndex(utteranceId);
            if (index === window.selectedIndex)
                window.requestProsodyPreview(index);
        }
    }

    Timer {
        id: prosodyPreviewTimer
        interval: 90
        repeat: false
        onTriggered: window.flushProsodyPreviewRequest()
    }

    property alias utterancesModel: utterances
    property alias playerMedia: player
    readonly property var settingsWindowRef: settingsWindowLoader.item
    readonly property bool settingsWindowVisible: settingsWindowRef !== null && settingsWindowRef.visible

    Translator {
        id: translatorInstance
        backend: window.appBackend
    }

    AppCore {
        id: core
        backend: window.appBackend
        translator: window.translator
    }

    property int selectedIndex: 0
    property int nextUtteranceId: 1
    property int draggedUtteranceIndex: -1
    property string audioUtteranceId: ""
    property int audioRevision: -1
    property string pendingUtteranceId: ""
    property int pendingRevision: -1
    property string pendingProsodyRequestId: ""
    property bool pendingProsodyInFlight: false
    property string pendingProsodyUtteranceId: ""
    property int pendingProsodyRevision: -1
    property int pendingProsodyPreviewIndex: -1
    property bool saveRequestPending: false
    property bool playbackRequested: false
    property string playbackError: ""
    property bool batchExportActive: false
    property string batchAnalysisUtteranceId: ""
    property int batchExportIndex: -1
    property int batchExportOriginalIndex: 0
    property int batchExportCompleted: 0
    property string batchExportMode: ""
    property var batchExportQueue: []
    property url batchExportDirectory
    property var dragExportFiles: []
    property bool dragExportSelectedOnly: false
    property int dragExportFrameRate: 60
    property bool dragExportReady: false
    property bool playbackQueueActive: false
    property var playbackQueue: []
    property int playbackQueueIndex: -1
    property bool projectDirty: false
    property url projectFile
    property var undoStack: []
    property var redoStack: []
    property string historyMergeKey: ""
    property bool historyRestoring: false
    property string savedProjectFingerprint: ""
    readonly property bool canUndo: undoStack.length > 0
    readonly property bool canRedo: redoStack.length > 0
    property bool metadataInitialized: false
    property bool closeAfterProjectSave: false
    property bool closeBypass: false
    property bool intonationLabInitialized: false
    property string intonationLabStatus: ""
    property int intonationLabPendingIndex: -1

    function audioOutputDeviceKey(device) {
        if (!device)
            return "";
        let key = "";
        if (device.id !== undefined && device.id !== null)
            key = String(window.appBackend.audioOutputDeviceKey(device.id) || "");
        if (!key.length && device.description)
            key = "description:" + String(device.description);
        return key;
    }

    function applyAudioOutputDevice() {
        const devices = mediaDevices.audioOutputs || [];
        const requested = String(window.appBackend.audioOutputDeviceId || "");
        let selected = mediaDevices.defaultAudioOutput;
        if (requested.length) {
            for (let index = 0; index < devices.length; ++index) {
                if (window.audioOutputDeviceKey(devices[index]) === requested) {
                    selected = devices[index];
                    break;
                }
            }
        }
        if (selected !== undefined && selected !== null)
            previewAudioOutput.device = selected;
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.synthesizeShortcut)
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
                 && utterances.count > 0 && window.current().reading.length > 0
        onActivated: window.synthesizeCurrent()
    }



    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.saveProjectShortcut)
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
        onActivated: window.saveCurrentProject()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.reloadVoicebanksShortcut)
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
        onActivated: window.reloadVoicebanks()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.addUtteranceShortcut)
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.addUtterance()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.removeUtteranceShortcut)
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
                 && utterances.count > 0
        onActivated: window.removeUtterance()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.undoShortcut)
        context: Qt.ApplicationShortcut
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.undo()
        onActivatedAmbiguously: window.undo()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.redoShortcut)
        context: Qt.ApplicationShortcut
        enabled: !window.settingsWindowVisible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.redo()
        onActivatedAmbiguously: window.redo()
    }

    ListModel {
        id: utterances
    }

    Timer {
        id: analyzeTimer
        interval: 180
        onTriggered: {
            if (utterances.count && window.current().content.trim()) {
                if (window.appBackend.busy) {
                    analyzeTimer.restart();
                    return;
                }
                const item = window.current();
                window.analyzeUtterance(window.selectedIndex);
            }
        }
    }

    MediaDevices {
        id: mediaDevices
    }

    AudioOutput {
        id: previewAudioOutput
        volume: 1.0
        muted: false
    }

    MediaPlayer {
        id: player
        audioOutput: previewAudioOutput
        onMediaStatusChanged: {
            if (window.playbackRequested && (mediaStatus === MediaPlayer.LoadedMedia || mediaStatus === MediaPlayer.BufferedMedia)) {
                window.playbackRequested = false;
                play();
            } else if (window.playbackQueueActive && mediaStatus === MediaPlayer.EndOfMedia) {
                ++window.playbackQueueIndex;
                window.playNextPlaybackItem();
            }
        }
        onErrorOccurred: (error, errorString) => {
            window.playbackRequested = false;
            window.playbackError = errorString;
        }
    }

    MainDialogs { id: dialogs; host: window }
    Loader {
        id: dragTargetWindowLoader
        active: Platform.hasNativeFileDialog
        sourceComponent: DragSourceWindow {
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            files: window.dragExportFiles
            exportDirectory: window.batchExportDirectory
            ready: window.dragExportReady
            accent: window.accent
            mutedText: window.mutedText
            onDragError: window.showLogWindow()
        }
    }

    Loader {
        id: synthesisLogWindowLoader
        active: false
        sourceComponent: SynthesisLogWindow {
            hostWindow: window
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            onClosed: {
                if (Platform.isWeb)
                    synthesisLogWindowLoader.active = false;
            }
        }
        onLoaded: {
            if (item)
                window.showAuxiliaryWindow(item);
        }
    }

    Loader {
        id: settingsWindowLoader
        active: false
        sourceComponent: SettingsWindow {
            hostWindow: window
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            audioOutputDevices: mediaDevices.audioOutputs
            onApplyRequested: closeAfter => window.saveSettings(closeAfter)
            onClosed: window.settingsWindowClosed()
        }
        onLoaded: {
            if (item) {
                item.loadCurrent();
                window.showAuxiliaryWindow(item);
            }
        }
    }

    Loader {
        id: dictionaryWindowLoader
        active: false
        sourceComponent: DictionaryWindow {
            hostWindow: window
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            onClosed: {
                if (Platform.isWeb)
                    dictionaryWindowLoader.active = false;
            }
        }
        onLoaded: {
            if (item) {
                item.loadCurrent();
                window.showAuxiliaryWindow(item);
            }
        }
    }

    Loader {
        id: licenseWindowLoader
        active: false
        sourceComponent: LicenseWindow {
            hostWindow: window
            hostPalette: window.palette
            translator: window.translator
            documents: window.licenseDocuments
            onClosed: {
                if (Platform.isWeb)
                    licenseWindowLoader.active = false;
            }
        }
        onLoaded: {
            if (item)
                window.showAuxiliaryWindow(item);
        }
    }

    Loader {
        id: voicebankDetailsWindowLoader
        active: false
        sourceComponent: VoicebankDetailsWindow {
            hostWindow: window
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            onClosed: {
                if (Platform.isWeb)
                    voicebankDetailsWindowLoader.active = false;
            }
        }
        onLoaded: {
            if (item) {
                item.currentIndex = Math.max(0, Math.min(item.currentIndex,
                                                          window.appBackend.voicebanks.length - 1));
                window.showAuxiliaryWindow(item);
            }
        }
    }

    Loader {
        id: onboardingWindowLoader
        active: !Platform.isWeb
        sourceComponent: OnboardingWindow {
            hostWindow: window
            hostPalette: window.palette
            backend: window.appBackend
            translator: window.translator
            visible: window.showOnboarding && !window.mobileLayout
        }
    }

    Timer {
        id: historyMergeTimer
        interval: 700
        onTriggered: window.historyMergeKey = ""
    }

    Connections {
        target: window.appBackend

        function onBusyChanged() {
            if (!window.appBackend.busy) {
                // 完了通知を反映してから、保留中の編集を処理する。
                Qt.callLater(function() {
                    if (window.autoPreviewPending && !autoPreviewTimer.running)
                        window.refreshPreview();
                    if (!prosodyPreviewTimer.running)
                        window.flushProsodyPreviewRequest();
                });
            }
        }

        function onProjectPicked(project) {
            window.applyLoadedProject(project);
        }

        function onLanguageChanged() {
            window.translator.load(window.appBackend.resolvedLanguage());
        }

        function onUstxExportFinished(success, detail) {
            dialogs.ustxExportMessageDialog.text = window.translator.tr(
                    success ? "main.ustxExportSuccess" : "main.ustxExportFailed", detail);
            dialogs.ustxExportMessageDialog.open();
        }

        function onUpdateDownloadProgress(bytesReceived, bytesTotal) {
            window.updateDownloadReceived = bytesReceived;
            window.updateDownloadTotal = bytesTotal;
        }

        function onUpdateDownloadFinished(success, localZip) {
            dialogs.updateProgressDialog.close();
            if (success && localZip.length
                    && window.appBackend.installUpdate(localZip, window.updateAvailableVersion))
                Qt.quit();
        }

        function onMetadataReloadStarted() {
            window.metadataReloadActive = true;
            window.metadataReloadStage = "voicebanks";
            dialogs.metadataReloadDialog.open();
        }

        function onMetadataReloadStageChanged(stage) {
            window.metadataReloadStage = stage;
        }

        function onMetadataChanged() {
            if (window.metadataReloadActive) {
                window.metadataReloadActive = false;
                dialogs.metadataReloadDialog.close();
            }
            const suppressDirty = !window.metadataInitialized;
            window.assignDefaultVoicebank(suppressDirty);
            window.assignDefaultSynthesisSettings(suppressDirty);
            window.metadataInitialized = true;
            if (suppressDirty)
                window.resetHistory(window.projectDirty);
            if (window.intonationLab && !window.intonationLabInitialized)
                Qt.callLater(window.initializeIntonationLab);
        }

        function onAnalysisChanged() {
            const requestId = window.appBackend.analysisRequestId;
            const sourceText = window.appBackend.analysisSourceText;
            const index = window.utteranceIndex(requestId);
            if (index < 0 || utterances.get(index).content !== sourceText)
                return;
            const analysis = JSON.parse(window.appBackend.analysisJson);
            window.applyPronunciation(index, analysis.reading, window.copySequence(analysis.morae));
            if (window.batchExportActive && requestId === window.batchAnalysisUtteranceId) {
                window.batchAnalysisUtteranceId = "";
                if (!analysis.reading) {
                    window.playbackError = window.translator.tr("diagnostics.errorTitle");
                    window.finishBatchExport(false);
                    return;
                }
                Qt.callLater(window.synthesizeBatchItem);
                return;
            }
            if (!window.batchExportActive && index === window.selectedIndex)
                window.requestProsodyPreview(index);
        }

        function onProsodyChanged() {
            UtteranceCore.handleProsodyChanged(window.utteranceCoreContext);
        }

        function onPreviewReady() {
            UtteranceCore.handlePreviewReady(window.utteranceCoreContext);
        }

        function onErrorChanged() {
            if (window.appBackend.error.length) {
                window.pendingProsodyInFlight = false;
                window.pendingProsodyUtteranceId = "";
            }
            if (window.metadataReloadActive && window.appBackend.error.length) {
                window.metadataReloadActive = false;
                dialogs.metadataReloadDialog.close();
            }
            if (window.batchExportActive && !window.appBackend.busy
                    && (window.pendingUtteranceId.length || window.batchAnalysisUtteranceId.length)
                    && window.appBackend.error.length)
                window.finishBatchExport(false);
            else if (window.playbackQueueActive && window.pendingUtteranceId.length && window.appBackend.error.length)
                window.stopPlaybackQueue();
            else if (window.saveRequestPending && window.pendingUtteranceId.length && window.appBackend.error.length) {
                window.saveRequestPending = false;
                window.pendingUtteranceId = "";
                window.pendingRevision = -1;
            }
        }

        function onAudioOutputSettingsChanged() {
            window.applyAudioOutputDevice();
        }
    }

    Connections {
        target: mediaDevices
        function onAudioOutputsChanged() {
            window.applyAudioOutputDevice();
        }
    }

    Component.onCompleted: {
        Platform.updateViewportWidth(width);
        if (!window.mobileLayout && window.showOnboarding)
            onboardingWindowLoader.active = true;
        window.translator.load(window.appBackend.resolvedLanguage());
        window.applyAudioOutputDevice();
        if (window.intonationLab)
            Qt.callLater(window.initializeIntonationLab);
        else
            addUtterance(false);
        window.resetHistory(false);
        if (!window.injectedSelfTest && window.appBackend.updateCheckEnabled)
            window.checkForUpdates();
    }

    onClosing: close => {
        if (window.closeBypass) {
            window.closeBypass = false;
            return;
        }
        if (!window.projectDirty && !window.appBackend.busy && !window.batchExportActive)
            return;
        close.accepted = false;
        dialogs.closeWarningDialog.open();
    }

    menuBar: MainMenuBar {
        host: window
    }

    header: ToolBar {
        visible: window.intonationLab || window.mobileLayout
        height: window.mobileLayout ? 56 : window.intonationLab ? 44 : 0

        RowLayout {
            anchors.fill: parent
            visible: window.mobileLayout
            spacing: 0
            FontLoader {
                id: headerIconFont
                source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
            }
            Repeater {
                model: [
                    { icon: "", tip: "menu.edit.undo", redo: false },
                    { icon: "", tip: "menu.edit.redo", redo: true }
                ]
                delegate: ToolButton {
                    id: historyButton
                    required property var modelData
                    Layout.preferredWidth: 52
                    Layout.preferredHeight: 56
                    enabled: (historyButton.modelData.redo ? window.canRedo : window.canUndo)
                             && !window.appBackend.busy && !window.batchExportActive && !window.playbackQueueActive
                    contentItem: Text {
                        text: historyButton.modelData.icon
                        font.family: headerIconFont.name
                        font.pixelSize: 26
                        color: historyButton.palette.buttonText
                        opacity: historyButton.enabled ? 1 : 0.35
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    Accessible.name: window.translator.tr(historyButton.modelData.tip)
                    onClicked: historyButton.modelData.redo ? window.redo() : window.undo()
                }
            }
            Item { Layout.fillWidth: true }
            ToolButton {
                text: "☰"
                font.pixelSize: 30
                Layout.preferredWidth: 64
                Layout.preferredHeight: 56
                onClicked: mobileChrome.openMenu()
            }
        }

        RowLayout {
            visible: window.intonationLab
            anchors.fill: parent
            anchors.leftMargin: 14
            anchors.rightMargin: 10
            spacing: 10

            Label {
                text: "イントネーション調整"
                font.bold: true
            }
            Label {
                text: {
                    let completed = 0;
                    for (let index = 0; index < window.utterancesModel.count; ++index) {
                        if (window.utterancesModel.get(index).trainingAccepted)
                            ++completed;
                    }
                    return completed + " / " + window.utterancesModel.count;
                }
                color: window.mutedText
            }
            Item { Layout.fillWidth: true }
            Label {
                Layout.maximumWidth: 360
                visible: text.length > 0
                text: window.intonationLabStatus
                color: window.appBackend.error.length ? "#b42318" : window.mutedText
                elide: Text.ElideRight
            }
            Button {
                text: "完了して次へ"
                enabled: !window.appBackend.busy && window.utterancesModel.count > 0
                         && window.current().reading.length > 0
                onClicked: window.completeIntonationLabEntry()
            }
        }
    }

    EditorContent {
        id: editorContent
        window: window
        anchors.fill: parent
    }

    MobileChrome {
        id: mobileChrome
        anchors.fill: parent
        window: window
        visible: window.mobileLayout
    }

    function current() {
        return QmlCore.currentUtterance(utterances, selectedIndex);
    }

    function rendererSettingContext(rendererId) {
        const context = {};
        const renderers = window.appBackend.renderers;
        for (let index = 0; index < renderers.length; ++index) {
            const renderer = renderers[index];
            if (String(renderer.id) !== String(rendererId || ""))
                continue;
            const settings = renderer.settings || [];
            for (let settingIndex = 0; settingIndex < settings.length; ++settingIndex) {
                const setting = settings[settingIndex];
                context[String(setting.id)] = window.appBackend.rendererSetting(
                        renderer.id, setting.id, setting.default);
            }
            break;
        }
        return context;
    }

    function addRendererSettings(request, rendererId) {
        const settings = window.rendererSettingContext(rendererId);
        const keys = Object.keys(settings);
        for (let index = 0; index < keys.length; ++index) {
            if (Object.prototype.hasOwnProperty.call(request, keys[index]))
                delete settings[keys[index]];
        }
        if (Object.keys(settings).length)
            request.renderer_settings = settings;
        return request;
    }

    function diagnosticContext() {
        if (!utterances.count)
            return {};
        const item = window.current();
        const context = {
            voicebank_id: item.voicebankId || "",
            model_id: item.modelId || "",
            renderer: item.renderer || "",
            alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone || "C4",
            color: item.color || "",
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            leading_preutterance_ms: item.leadingPreutterance,
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch,
            resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson),
            unit_overrides: window.decodeSequence(item.phonemeOverridesJson)
        };
        window.addRendererSettings(context, item.renderer);
        return context;
    }

    function qtShortcutSequence(sequence) {
        const parts = String(sequence || "").split("+");
        if (parts.length && parts[parts.length - 1] === "Enter")
            parts[parts.length - 1] = "Return";
        return parts.join("+");
    }

    function recentProjectLabel(path) {
        const normalized = String(path || "").replace(/\\/g, "/");
        const slash = normalized.lastIndexOf("/");
        return slash >= 0 ? normalized.slice(slash + 1) : normalized;
    }

    function loadRecentProject(path) {
        const normalized = String(path || "").replace(/\\/g, "/");
        if (!normalized.length)
            return;
        const encoded = normalized.split("/").map(segment => encodeURIComponent(segment)).join("/");
        const url = normalized.startsWith("/") ? "file://" + encoded : "file:///" + encoded;
        window.loadProjectFrom(url);
    }

    function showVoicebankDetails() {
        if (!window.appBackend.voicebanks.length)
            return;
        if (voicebankDetailsWindowLoader.item) {
            const details = voicebankDetailsWindowLoader.item;
            details.currentIndex = Math.max(0, Math.min(details.currentIndex,
                                                        window.appBackend.voicebanks.length - 1));
            window.showAuxiliaryWindow(details);
        } else {
            voicebankDetailsWindowLoader.active = true;
        }
    }

    function openLicense() {
        if (licenseWindowLoader.item)
            window.showAuxiliaryWindow(licenseWindowLoader.item);
        else
            licenseWindowLoader.active = true;
    }

    function addVoicebanks() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        if (Platform.hasNativeFileDialog)
            dialogs.voicebankAddDialog.open();
        else
            window.appBackend.beginAddVoicebanks();
    }

    function saveSettings(closeAfter) {
        const shortcuts = [settingsWindowRef.view.pendingSynthesizeShortcut,
                           settingsWindowRef.view.pendingSaveProjectShortcut,
                           settingsWindowRef.view.pendingReloadVoicebanksShortcut,
                           settingsWindowRef.view.pendingAddUtteranceShortcut,
                           settingsWindowRef.view.pendingRemoveUtteranceShortcut,
                           settingsWindowRef.view.pendingUndoShortcut,
                           settingsWindowRef.view.pendingRedoShortcut];
        const usedShortcuts = [];
        for (let index = 0; index < shortcuts.length; ++index) {
            const shortcut = String(shortcuts[index] || "").trim();
            if (!shortcut.length)
                continue;
            const normalized = window.qtShortcutSequence(shortcut).toLowerCase();
            if (usedShortcuts.indexOf(normalized) >= 0) {
                dialogs.shortcutConflictDialog.open();
                return;
            }
            usedShortcuts.push(normalized);
        }
        window.appBackend.setSynthesisDefaults(settingsWindowRef.view.pendingDefaultModelId,
                                               settingsWindowRef.view.pendingDefaultRendererId,
                                               settingsWindowRef.view.pendingDefaultTone,
                                               settingsWindowRef.view.pendingDefaultAliasPolicy);
        window.appBackend.setDarkMode(settingsWindowRef.view.pendingDarkMode);
        window.appBackend.setLanguage(settingsWindowRef.view.pendingLanguage);
        window.appBackend.setFfmpegPath(settingsWindowRef.view.pendingFfmpegPath);
        window.appBackend.setAudioOutputDeviceId(settingsWindowRef.view.pendingAudioOutputDeviceId);
        window.appBackend.setCloseLogOnSuccess(settingsWindowRef.view.pendingCloseLogOnSuccess);
        window.appBackend.setUpdateCheckEnabled(settingsWindowRef.view.pendingUpdateCheckEnabled);
        window.appBackend.setPreReleaseUpdateCheckEnabled(
                    settingsWindowRef.view.pendingPreReleaseUpdateCheckEnabled);
        window.appBackend.setPreviewCacheFileCount(settingsWindowRef.view.pendingPreviewCacheFileCount);
        window.appBackend.setAutoPreviewEnabled(settingsWindowRef.view.pendingAutoPreviewEnabled);
        window.appBackend.setExtendedDetailsVisible(settingsWindowRef.view.pendingExtendedDetailsVisible);
        window.appBackend.setDefaultVoicebank(settingsWindowRef.view.pendingDefaultVoicebankId);
        window.appBackend.setExportSettings(settingsWindowRef.view.pendingExportTextWithWav,
                                            settingsWindowRef.view.pendingExportLabWithWav,
                                            settingsWindowRef.view.pendingExportTextEncoding);
        window.appBackend.setShortcutSequences(settingsWindowRef.view.pendingSynthesizeShortcut,
                                               settingsWindowRef.view.pendingSaveProjectShortcut,
                                               settingsWindowRef.view.pendingReloadVoicebanksShortcut,
                                               settingsWindowRef.view.pendingAddUtteranceShortcut,
                                               settingsWindowRef.view.pendingRemoveUtteranceShortcut,
                                               settingsWindowRef.view.pendingUndoShortcut,
                                               settingsWindowRef.view.pendingRedoShortcut);
        if (closeAfter) {
            const settingsWindow = window.settingsWindowRef;
            if (settingsWindow) {
                settingsWindow.close();
                settingsWindow.visible = false;
            }
        }
    }

    function showAuxiliaryWindow(auxiliaryWindow) {
        if (auxiliaryWindow.open !== undefined) {
            auxiliaryWindow.open();
            return;
        }
        auxiliaryWindow.visible = true;
        auxiliaryWindow.raise();
        auxiliaryWindow.requestActivate();
    }

    function showLogWindow() {
        if (window.mobileLayout)
            return;
        if (synthesisLogWindowLoader.item)
            window.showAuxiliaryWindow(synthesisLogWindowLoader.item);
        else
            synthesisLogWindowLoader.active = true;
    }

    function closeLogWindow() {
        if (synthesisLogWindowLoader.item)
            synthesisLogWindowLoader.item.close();
    }

    // 4番目は正式版1・ベータ版0、5番目はベータ番号。同じ版ではベータ版を正式版より前に並べる。
    function versionParts(version) {
        const match = /(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-beta\.(\d+))?/.exec(String(version));
        if (!match)
            return null;
        const parts = [];
        for (let index = 1; index <= 3; ++index)
            parts.push(match[index] ? parseInt(match[index], 10) : 0);
        parts.push(match[4] ? 0 : 1);
        parts.push(match[4] ? parseInt(match[4], 10) : 0);
        return parts;
    }

    function compareVersions(a, b) {
        const partsA = window.versionParts(a);
        const partsB = window.versionParts(b);
        if (!partsA || !partsB)
            return 0;
        const length = Math.max(partsA.length, partsB.length);
        for (let index = 0; index < length; ++index) {
            const valueA = partsA[index] || 0;
            const valueB = partsB[index] || 0;
            if (valueA !== valueB)
                return valueA < valueB ? -1 : 1;
        }
        return 0;
    }

    function releaseAssetURL(release) {
        const packageName = Qt.platform.os === "linux"
                ? "UtauTTS-linux-x64.zip"
                : Qt.platform.os === "osx"
                ? "UtauTTS-mac-arm64.zip"
                : "UtauTTS-win-x64.zip";
        const assets = release && Array.isArray(release.assets) ? release.assets : [];
        for (const asset of assets) {
            if (asset && asset.name === packageName && asset.browser_download_url)
                return String(asset.browser_download_url);
        }
        return "";
    }

    function isNewerRelease(left, right) {
        const versionOrder = window.compareVersions(left.tag_name, right.tag_name);
        if (versionOrder !== 0)
            return versionOrder > 0;
        if (!!left.prerelease !== !!right.prerelease)
            return !left.prerelease;
        const leftDate = Date.parse(String(left.published_at || left.created_at || ""));
        const rightDate = Date.parse(String(right.published_at || right.created_at || ""));
        return Number.isFinite(leftDate) && leftDate > rightDate;
    }

    function selectUpdateRelease(data, allowPreRelease) {
        const releases = Array.isArray(data) ? data : [data];
        let selected = null;
        for (const release of releases) {
            if (!release || release.draft || (!allowPreRelease && release.prerelease))
                continue;
            if (!release.tag_name || window.compareVersions(release.tag_name, Qt.application.version) <= 0)
                continue;
            if (!window.releaseAssetURL(release))
                continue;
            if (!selected || window.isNewerRelease(release, selected))
                selected = release;
        }
        return selected;
    }

    function checkForUpdates() {
        const request = new XMLHttpRequest();
        request.timeout = 10000;
        const allowPreRelease = window.appBackend.preReleaseUpdateCheckEnabled;
        const endpoint = allowPreRelease
                ? "https://api.github.com/repos/yh2237/UtauTTS/releases?per_page=100"
                : "https://api.github.com/repos/yh2237/UtauTTS/releases/latest";
        request.open("GET", endpoint);
        request.onreadystatechange = function() {
            if (request.readyState !== XMLHttpRequest.DONE)
                return;
            if (request.status !== 200)
                return;
            let data;
            try {
                data = JSON.parse(request.responseText);
            } catch (error) {
                return;
            }
            const release = window.selectUpdateRelease(data, allowPreRelease);
            if (!release)
                return;
            const latest = String(release.tag_name);
            const suppressed = window.appBackend.suppressedUpdateVersion();
            if (suppressed && window.compareVersions(latest, suppressed) <= 0)
                return;
            window.updateSuppressVersion = false;
            window.updateAvailableVersion = latest;
            window.updateReleaseNotes = release.body ? String(release.body) : "";
            window.updateReleaseUrl = release.html_url ? String(release.html_url) : "";
            window.updateDownloadUrl = window.releaseAssetURL(release);
            window.updateAvailablePreRelease = !!release.prerelease;
            dialogs.updateDialog.open();
        };
        request.send();
    }

    function performUpdate() {
        if (!window.updateDownloadUrl) {
            Qt.openUrlExternally(window.updateReleaseUrl);
            return;
        }
        window.updateDownloadReceived = 0;
        window.updateDownloadTotal = 0;
        if (window.appBackend.startUpdateDownload(window.updateDownloadUrl, window.updateAvailableVersion)) {
            dialogs.updateDialog.close();
            dialogs.updateProgressDialog.open();
        } else {
            dialogs.updateDialog.close();
            Qt.openUrlExternally(window.updateReleaseUrl);
        }
    }

    function openSettings() {
        if (window.settingsWindowRef) {
            window.settingsWindowRef.loadCurrent();
            window.showAuxiliaryWindow(window.settingsWindowRef);
        } else {
            settingsWindowLoader.active = true;
        }
    }

    function settingsWindowClosed() {
        if (Platform.isWeb)
            settingsWindowLoader.active = false;
    }

    function openDictionarySettings() {
        if (dictionaryWindowLoader.item) {
            dictionaryWindowLoader.item.loadCurrent();
            window.showAuxiliaryWindow(dictionaryWindowLoader.item);
        } else {
            dictionaryWindowLoader.active = true;
        }
    }

    function voicebankById(id) { return core.voicebankById(id); }

    function reloadVoicebanks() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        window.clearPlayback();
        window.appBackend.reloadVoicebanks();
    }

    function voicebankTypeOptions(id, selectedColor) { return core.voicebankTypeOptions(id, selectedColor); }

    function voicebankTypeOptionAt(id, index, selectedColor) { return core.voicebankTypeOptionAt(id, index, selectedColor); }

    function voicebankHasColor(id, color) { return core.voicebankHasColor(id, color); }

    function typeIdForColor(id, color) { return core.typeIdForColor(id, color); }

    function defaultVoicebank() { return core.defaultVoicebank(); }

    function modelById(id) { return core.modelById(id); }

    function rendererById(id) { return core.rendererById(id); }

    function defaultModelId() { return core.defaultModelId(); }

    function defaultModelIdForLanguage(language) { return core.defaultModelIdForLanguage(language); }

    function preferredRendererForModel(model) { return core.preferredRendererForModel(model); }

    function defaultRendererId() { return core.defaultRendererId(); }

    function normalizeRendererId(id) { return core.normalizeRendererId(id); }

    function normalizeAliasPolicy(value) { return core.normalizeAliasPolicy(value); }

    function utteranceIndex(id) {
        return QmlCore.utteranceIndex(utterances, id);
    }

    function voicebankName(id) { return core.voicebankName(id); }

    function fileNamePart(value, fallback) { return core.fileNamePart(value, fallback); }

    function audioFileName(item) { return core.audioFileName(item); }

    function dragAudioFileName(item, index) { return core.dragAudioFileName(item, index); }

    function saveCurrentAudio() {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive || !window.current().reading.length)
            return;
        const item = window.current();
        window.clearPlayback();
        window.saveRequestPending = true;
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.clearLogs();
        window.showLogWindow();
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function openSaveAllDialog() {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive)
            return;
        if (Platform.hasNativeFileDialog) {
            dialogs.saveAllDialog.open();
        } else {
            window.startBatchExport(window.appBackend.defaultSaveFile("utautts.wav"));
        }
    }

    function openDragExportDialog(selectedOnly) {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive)
            return;
        if (selectedOnly && !window.current().reading.length)
            return;
        window.dragExportSelectedOnly = selectedOnly;
        dialogs.frameRateDialog.open();
    }

    function projectNumber(value, fallback, minimum, maximum, integer) {
        const parsed = Number(value);
        if (!Number.isFinite(parsed))
            return fallback;
        const normalized = integer ? Math.round(parsed) : parsed;
        return Math.max(minimum, Math.min(maximum, normalized));
    }

    function historySnapshot() {
        return HistoryCore.historySnapshot(window.historyCoreContext);
    }

    function editableFingerprint() {
        return HistoryCore.editableFingerprint(window.historyCoreContext);
    }

    function beginHistoryChange(key, merge) {
        HistoryCore.beginHistoryChange(window.historyCoreContext, key, merge);
    }

    function endHistoryGesture() {
        HistoryCore.endHistoryGesture(window.historyCoreContext);
    }

    function resetHistory(markDirty) {
        HistoryCore.resetHistory(window.historyCoreContext, markDirty);
    }

    function clearEditHistory() {
        HistoryCore.clearEditHistory(window.historyCoreContext);
    }

    function undo() {
        HistoryCore.undo(window.historyCoreContext);
    }

    function redo() {
        HistoryCore.redo(window.historyCoreContext);
    }
    function runInterfaceSelfTest() {
        return SelfTest.run(window.selfTestContext);
    }
    function intonationLabDefaultFile() {
        return IntonationLab.defaultFile(window.intonationLabContext);
    }

    function intonationLabBaseModelPath() {
        return IntonationLab.baseModelPath();
    }

    function intonationLabFirstIncomplete() {
        return IntonationLab.firstIncomplete(window.intonationLabContext);
    }

    function prepareIntonationLabEntry(index) {
        IntonationLab.prepareEntry(window.intonationLabContext, index);
    }

    function initializeIntonationLab() {
        IntonationLab.initialize(window.intonationLabContext);
    }

    function completeIntonationLabEntry() {
        IntonationLab.completeEntry(window.intonationLabContext);
    }
    function projectData() {
        const savedUtterances = [];
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            const saved = {
                text: item.content || "",
                language: item.language || "ja",
                phonemizer: item.phonemizer || window.defaultPhonemizer(item.language || "ja"),
                voicebank_id: item.voicebankId || "",
                model_id: item.modelId || "",
                renderer_id: item.renderer || "",
                alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
                tone: item.tone || "C4",
                color: item.color || "",
                mora_duration_ms: item.moraDuration,
                pause_duration_ms: item.pauseDuration,
                leading_preutterance_ms: item.leadingPreutterance,
                intonation: item.intonation,
                apply_pitch: !!item.applyPitch,
                pitch_points: window.decodeSequence(item.pointsJson),
                pitch_frames: window.decodeSequence(item.pitchFramesJson),
                mora_durations_ms: window.decodeSequence(item.moraDurationsJson),
                mora_positions_ms: window.decodeSequence(item.moraPositionsJson),
                automatic_pitch_points: window.automaticSequence(item, "autoPointsJson"),
                automatic_frame_pitch: window.automaticSequence(item, "autoPitchFramesJson"),
                automatic_frame_ms: Number(item.autoFrameMs) || 10,
                automatic_mora_durations_ms: window.automaticSequence(item, "autoMoraDurationsJson"),
                automatic_mora_positions_ms: window.automaticSequence(item, "autoMoraPositionsJson"),
                manual_pitch_edited: window.hasManualPitch(item),
                manual_mora_duration_edited: window.hasManualMoraDurations(item),
                resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson),
                phoneme_overrides: window.decodeSequence(item.phonemeOverridesJson),
                analysis_cache: {
                    reading: item.reading || "",
                    morae: window.decodeSequence(item.moraeJson)
                }
            };
            if (window.intonationLab) {
                saved.training_accepted = !!item.trainingAccepted;
                saved.lab_entry_id = item.labEntryId || "";
            }
            savedUtterances.push(saved);
        }
        const project = {
            format: "utautts-project",
            format_version: 8,
            app_version: Qt.application.version,
            utterances: savedUtterances,
            selected_index: utterances.count ? selectedIndex : 0
        };
        if (window.intonationLab) {
            project.intonation_lab = {
                version: 2,
                corpus: "japanese-v1",
                base_model_path: window.intonationLabBaseModelPath(),
                completed: window.intonationLabFirstIncomplete() < 0
            };
        }
        return project;
    }

    function reanalyzeAll() {
        window.clearPlayback();
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            utterances.setProperty(index, "reading", "");
            utterances.setProperty(index, "moraeJson", "[]");
            utterances.setProperty(index, "phonemeOverridesJson", "[]");
            if (item.content.trim())
                window.analyzeUtterance(index);
        }
        if (utterances.count)
            window.selectUtterance(window.selectedIndex);
    }

    function openProjectSaveDialog() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        const destination = window.projectFile.toString().length
                ? window.projectFile : window.appBackend.defaultSaveFile("untitled.utautts");
        if (Platform.hasNativeFileDialog) {
            dialogs.projectSaveDialog.currentFile = destination;
            dialogs.projectSaveDialog.open();
        } else {
            window.saveProjectTo(destination);
        }
    }

    function saveCurrentProject() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        if (!window.projectFile.toString().length) {
            window.openProjectSaveDialog();
            return;
        }
        window.saveProjectTo(window.projectFile);
    }

    function openUstxExportDialog() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        const destination = window.appBackend.defaultSaveFile("untitled.ustx");
        if (Platform.hasNativeFileDialog) {
            dialogs.ustxExportFileDialog.currentFile = destination;
            dialogs.ustxExportFileDialog.open();
        } else {
            window.exportUstxTo(destination);
        }
    }

    function exportUstxTo(destination) {
        if (!destination || !destination.toString().length)
            return;
        window.appBackend.exportUstx(destination, window.projectData());
    }

    function exportDiagnosticsTo(destination) {
        if (!destination || !destination.toString().length)
            return;
        const success = window.appBackend.exportDiagnosticReport(
                destination, window.diagnosticContext());
        dialogs.diagnosticResultDialog.title = window.translator.tr(
                success ? "diagnostics.successTitle" : "diagnostics.errorTitle");
        dialogs.diagnosticResultDialog.text = success
                ? window.translator.tr("diagnostics.success")
                : window.appBackend.error;
        dialogs.diagnosticResultDialog.open();
    }

    function saveProjectTo(destination) {
        if (!destination || !destination.toString().length)
            return;
        const saved = window.appBackend.saveProject(destination, window.projectData());
        if (!saved) {
            window.closeAfterProjectSave = false;
            return;
        }
        window.projectFile = destination;
        window.appBackend.rememberRecentProject(destination);
        window.endHistoryGesture();
        window.savedProjectFingerprint = window.editableFingerprint();
        window.projectDirty = false;
        if (window.closeAfterProjectSave) {
            window.closeAfterProjectSave = false;
            window.quitWithoutWarning();
        }
    }

    function loadProjectFrom(source) {
        if (!source || !source.toString().length || window.appBackend.busy || window.batchExportActive)
            return;
        const project = window.appBackend.loadProject(source);
        if (!project || project._error !== undefined) {
            dialogs.projectLoadErrorDialog.text = project && project._error !== undefined
                    ? String(project._error) : window.translator.tr("main.projectLoadError");
            dialogs.projectLoadErrorDialog.open();
            return;
        }
        window.applyLoadedProject(project, source);
    }

    function openProject() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        if (Platform.hasNativeFileDialog)
            dialogs.projectOpenDialog.open();
        else
            window.appBackend.beginOpenProject();
    }

    function applyLoadedProject(project, source) {
        if (!project || project.utterances === undefined || project.utterances === null) {
            dialogs.projectLoadErrorDialog.text = window.translator.tr("main.projectNoUtterances");
            dialogs.projectLoadErrorDialog.open();
            return;
        }
        const loadedUtterances = window.copySequence(project.utterances);

        window.projectDirty = false;
        window.clearPlayback();
        utterances.clear();
        window.nextUtteranceId = 1;
        let migratedRenderer = false;
        const projectFormatVersion = Number(project.format_version) || 1;
        for (let index = 0; index < loadedUtterances.length; ++index) {
            const saved = loadedUtterances[index] || {};
            const voicebankId = String(saved.voicebank_id || "");
            const voice = window.voicebankById(voicebankId);
            const points = window.copySequence(saved.pitch_points);
            const pitchFrames = window.copySequence(saved.pitch_frames);
            const content = String(saved.text || "");
            let rendererId = window.normalizeRendererId(saved.renderer_id);
            if (String(saved.renderer_id || "") !== rendererId)
                migratedRenderer = true;
            const manualDurations = window.copySequence(saved.mora_durations_ms);
            const automaticDurations = window.copySequence(saved.automatic_mora_durations_ms);
            let manualPositions = window.copySequence(saved.mora_positions_ms);
            let automaticPositions = window.copySequence(saved.automatic_mora_positions_ms);
            if (projectFormatVersion < 2) {
                manualPositions = window.moraStartsFromCenters(manualPositions, manualDurations);
                automaticPositions = window.moraStartsFromCenters(automaticPositions, automaticDurations);
            }
            utterances.append({
                utteranceId: "utterance-" + window.nextUtteranceId++,
                content: content,
                language: String(saved.language || "ja"),
                phonemizer: String(saved.phonemizer || window.defaultPhonemizer(saved.language || "ja")),
                reading: "",
                moraeJson: "[]",
                pointsJson: JSON.stringify(points),
                pitchFramesJson: JSON.stringify(pitchFrames),
                moraDurationsJson: JSON.stringify(manualDurations),
                moraPositionsJson: JSON.stringify(manualPositions),
                autoPointsJson: JSON.stringify(window.copySequence(saved.automatic_pitch_points)),
                autoPitchFramesJson: JSON.stringify(window.copySequence(saved.automatic_frame_pitch)),
                autoFrameMs: Number(saved.automatic_frame_ms) || 10,
                autoMoraDurationsJson: JSON.stringify(automaticDurations),
                autoMoraPositionsJson: JSON.stringify(automaticPositions),
                resamplerExpressionsJson: JSON.stringify(window.copySequence(saved.resampler_expressions)),
                phonemeOverridesJson: JSON.stringify(window.copySequence(saved.phoneme_overrides)),
                manualPitchEdited: saved.manual_pitch_edited === undefined
                        ? points.some(value => Math.abs(Number(value)) > .1)
                          || pitchFrames.some(value => Math.abs(Number(value)) > .1)
                        : !!saved.manual_pitch_edited,
                manualMoraDurationEdited: saved.manual_mora_duration_edited === undefined
                        ? window.copySequence(saved.mora_durations_ms).some(value => Number(value) > 0)
                        : !!saved.manual_mora_duration_edited,
                trainingAccepted: !!saved.training_accepted,
                labEntryId: String(saved.lab_entry_id || ""),
                voicebankId: voicebankId,
                imagePath: voice ? voice.image_path || "" : "",
                modelId: String(saved.model_id || ""),
                renderer: rendererId,
                aliasPolicy: saved.alias_policy === undefined
                        ? window.appBackend.defaultAliasPolicy : window.normalizeAliasPolicy(saved.alias_policy),
                tone: String(saved.tone || window.appBackend.defaultTone),
                color: String(saved.color || ""),
                moraDuration: window.projectNumber(saved.mora_duration_ms, window.appBackend.defaultMoraDuration, 20, 1000, true),
                pauseDuration: window.projectNumber(saved.pause_duration_ms, window.appBackend.defaultPauseDuration, 0, 3000, true),
                leadingPreutterance: window.projectNumber(saved.leading_preutterance_ms, 0, 0, 300, true),
                intonation: window.projectNumber(saved.intonation, window.defaultIntonationStrength, 0, window.maxIntonationStrength, false),
                applyPitch: saved.apply_pitch === undefined ? true : !!saved.apply_pitch,
                revision: 0
            });
        }

        window.projectDirty = migratedRenderer;
        window.projectFile = source || "";
        if (source && source.toString().length)
            window.appBackend.rememberRecentProject(source);

        if (!utterances.count) {
            selectedIndex = 0;
            editorContent.pitchEditor.points = [];
            editorContent.pitchEditor.morae = [];
            editorContent.pitchEditor.moraDurations = [];
            editorContent.pitchEditor.moraPositions = [];
            editorContent.phonemeEditor.manualFrames = [];
            editorContent.phonemeEditor.autoFrames = [];
            window.resetHistory(migratedRenderer);
            return;
        }
        selectedIndex = Math.max(0, Math.min(Number(project.selected_index) || 0, utterances.count - 1));
        window.selectUtterance(selectedIndex);
        if (window.intonationLab) {
            const next = window.intonationLabFirstIncomplete();
            if (next >= 0) {
                window.selectedIndex = next;
                window.prepareIntonationLabEntry(next);
            } else {
                window.intonationLabStatus = "全ての例文を書き出しました。";
            }
        } else {
            for (let index = 0; index < utterances.count; ++index) {
                const item = utterances.get(index);
                if (item.content.trim())
                    window.analyzeUtterance(index);
            }
        }
        editorContent.utteranceList.positionViewAtIndex(selectedIndex, ListView.Contain);
        window.resetHistory(migratedRenderer);
    }

    function localImageUrl(path) {
        return path ? window.appBackend.localFileUrl(path) : "";
    }

    function defaultPhonemizer(language) { return core.defaultPhonemizer(language); }

    function phonemizerOptions(language) { return core.phonemizerOptions(language); }

    function resolvedPhonemizer(language, phonemizer, voicebankId) { return core.resolvedPhonemizer(language, phonemizer, voicebankId); }

    function analyzeUtterance(index) {
        if (index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (!item.content.trim())
            return;
        window.appBackend.analyzeSpeech(item.content, item.utteranceId,
                                        item.language || "ja",
                                        window.resolvedPhonemizer(item.language, item.phonemizer,
                                                                 item.voicebankId),
                                        item.voicebankId || "");
    }

    function updateSpeechLanguage(language, phonemizer) {
        if (!utterances.count)
            return;
        const item = current();
        if (item.language === language && item.phonemizer === phonemizer)
            return;
        utterances.setProperty(selectedIndex, "language", language);
        utterances.setProperty(selectedIndex, "phonemizer", phonemizer);
        utterances.setProperty(selectedIndex, "modelId", window.defaultModelIdForLanguage(language));
        utterances.setProperty(selectedIndex, "reading", "");
        utterances.setProperty(selectedIndex, "moraeJson", "[]");
        utterances.setProperty(selectedIndex, "phonemeOverridesJson", "[]");
        clearAutomaticProsody(selectedIndex);
        markUtteranceDirty(selectedIndex);
        selectCombo(editorContent.modelCombo, current().modelId);
        window.analyzeUtterance(selectedIndex);
    }

    function updateSetting(name, value) {
        if (!utterances.count)
            return;
        const item = current();
        if (item[name] === value)
            return;
        utterances.setProperty(selectedIndex, name, value);
        if (["voicebankId", "modelId", "renderer", "aliasPolicy", "phonemizer", "tone", "color", "moraDuration", "pauseDuration",
             "intonation", "applyPitch"].indexOf(name) >= 0)
            clearAutomaticProsody(selectedIndex);
        if (name === "moraDuration")
            editorContent.pitchEditor.defaultMoraDuration = value;
        else if (name === "pauseDuration")
            editorContent.pitchEditor.defaultPauseDuration = value;
        markUtteranceDirty(selectedIndex);
        if (name === "phonemizer") {
            utterances.setProperty(selectedIndex, "reading", "");
            utterances.setProperty(selectedIndex, "moraeJson", "[]");
            utterances.setProperty(selectedIndex, "phonemeOverridesJson", "[]");
            window.analyzeUtterance(selectedIndex);
        }
        if (name === "voicebankId") {
            utterances.setProperty(selectedIndex, "reading", "");
            utterances.setProperty(selectedIndex, "moraeJson", "[]");
            utterances.setProperty(selectedIndex, "phonemeOverridesJson", "[]");
            window.analyzeUtterance(selectedIndex);
        }
        if (["voicebankId", "modelId", "renderer", "aliasPolicy", "phonemizer"].indexOf(name) >= 0)
            utterances.setProperty(selectedIndex, "phonemeOverridesJson", "[]");
        if (name === "voicebankId") {
            const voice = window.voicebankById(value);
            if (Platform.hasDiffsinger && voice && String(voice.kind || "") === "diffsinger") {
                utterances.setProperty(selectedIndex, "renderer", "diffsinger");
                selectCombo(editorContent.rendererCombo, "diffsinger");
            } else if (item.renderer === "diffsinger") {
                const renderer = window.defaultRendererId();
                utterances.setProperty(selectedIndex, "renderer", renderer);
                selectCombo(editorContent.rendererCombo, renderer);
            }
        }
        if (["voicebankId", "aliasPolicy", "modelId", "renderer", "tone", "color", "moraDuration",
             "pauseDuration", "intonation", "applyPitch"].indexOf(name) >= 0) {
            window.requestMissingProsodyPreview(selectedIndex);
        }
    }

    function updateUtteranceText(index, text) {
        if (index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (item.content === text)
            return;
        window.clearEditHistory();
        if (index === window.selectedIndex)
            window.clearSynthesisView();
        utterances.setProperty(index, "content", text);
        utterances.setProperty(index, "reading", "");
        utterances.setProperty(index, "moraeJson", "[]");
        utterances.setProperty(index, "pointsJson", "[]");
        utterances.setProperty(index, "pitchFramesJson", "[]");
        utterances.setProperty(index, "moraDurationsJson", "[]");
        utterances.setProperty(index, "moraPositionsJson", "[]");
        utterances.setProperty(index, "autoPointsJson", "[]");
        utterances.setProperty(index, "autoMoraDurationsJson", "[]");
        utterances.setProperty(index, "autoMoraPositionsJson", "[]");
        utterances.setProperty(index, "autoPitchFramesJson", "[]");
        utterances.setProperty(index, "phonemeOverridesJson", "[]");
        utterances.setProperty(index, "manualPitchEdited", false);
        utterances.setProperty(index, "manualMoraDurationEdited", false);
        markUtteranceDirty(index);
        selectUtterance(index);
        window.requestMissingProsodyPreview(index);
    }

    function updatePitchPoints(points) {
        if (!utterances.count)
            return;
        const pointsJson = JSON.stringify(points);
        if (current().pointsJson === pointsJson && current().manualPitchEdited)
            return;
        window.beginHistoryChange("pitch:" + current().utteranceId, false);
        utterances.setProperty(selectedIndex, "pointsJson", pointsJson);
        utterances.setProperty(selectedIndex, "manualPitchEdited", true);
        if (!current().applyPitch) {
            utterances.setProperty(selectedIndex, "applyPitch", true);
        }
        markUtteranceDirty(selectedIndex);
        window.scheduleAutoPreview();
    }

    function hasManualPitchFrames(item) {
        return window.decodeSequence(item ? item.pitchFramesJson : "")
                .some(value => Math.abs(Number(value)) > .1);
    }

    function updatePitchFrames(frames) {
        if (!utterances.count)
            return;
        const framesJson = JSON.stringify(frames);
        if (current().pitchFramesJson === framesJson && current().manualPitchEdited)
            return;
        window.beginHistoryChange("pitchframes:" + current().utteranceId, false);
        utterances.setProperty(selectedIndex, "pitchFramesJson", framesJson);
        utterances.setProperty(selectedIndex, "manualPitchEdited", true);
        if (!current().applyPitch) {
            utterances.setProperty(selectedIndex, "applyPitch", true);
        }
        markUtteranceDirty(selectedIndex);
        window.scheduleAutoPreview();
    }

    function applyAutomaticFramePitch(index, automaticFrames, frameMs) {
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "autoPitchFramesJson", JSON.stringify(automaticFrames));
        utterances.setProperty(index, "autoFrameMs", Number(frameMs) > 0 ? Number(frameMs) : 10);
        if (index === window.selectedIndex && editorContent.phonemeEditor)
            editorContent.phonemeEditor.autoFrames = automaticFrames.slice();
    }

    function updateMoraTiming(durations, positions) {
        if (!utterances.count)
            return;
        const item = current();
        const durationsJson = JSON.stringify(durations);
        const positionsJson = JSON.stringify(window.normalizedMoraPositions(positions));
        if (item.moraDurationsJson === durationsJson
                && item.moraPositionsJson === positionsJson
                && item.manualMoraDurationEdited)
            return;
        window.beginHistoryChange("timing:" + item.utteranceId, true);
        utterances.setProperty(selectedIndex, "moraDurationsJson", durationsJson);
        utterances.setProperty(selectedIndex, "moraPositionsJson", positionsJson);
        utterances.setProperty(selectedIndex, "manualMoraDurationEdited", true);
        markUtteranceDirty(selectedIndex);
        window.scheduleTimingProsodyPreview(selectedIndex);
        window.scheduleAutoPreview();
    }

    function updateTimingAndPitch(durations, positions, points) {
        if (!utterances.count)
            return;
        const item = current();
        const durationsJson = JSON.stringify(durations);
        const positionsJson = JSON.stringify(window.normalizedMoraPositions(positions));
        const pointsJson = JSON.stringify(points);
        const timingChanged = item.moraDurationsJson !== durationsJson
                || item.moraPositionsJson !== positionsJson;
        const pitchChanged = item.pointsJson !== pointsJson;
        if (!timingChanged && !pitchChanged)
            return;
        window.beginHistoryChange("note:" + item.utteranceId, false);
        if (timingChanged) {
            utterances.setProperty(selectedIndex, "moraDurationsJson", durationsJson);
            utterances.setProperty(selectedIndex, "moraPositionsJson", positionsJson);
            utterances.setProperty(selectedIndex, "manualMoraDurationEdited", true);
        }
        if (pitchChanged) {
            utterances.setProperty(selectedIndex, "pointsJson", pointsJson);
            utterances.setProperty(selectedIndex, "manualPitchEdited", true);
            if (!item.applyPitch)
                utterances.setProperty(selectedIndex, "applyPitch", true);
        }
        markUtteranceDirty(selectedIndex);
        if (timingChanged)
            window.scheduleTimingProsodyPreview(selectedIndex);
        window.scheduleAutoPreview();
    }

    function updateMoraStart(position, startMs) {
        if (!utterances.count || position < 0 || position >= editorContent.pitchEditor.morae.length)
            return;
        if (position === 0)
            return;
        if (!Number.isFinite(Number(startMs)))
            return;
        editorContent.pitchEditor.setPositionAtMS(position, Number(startMs), false);
    }

    function updateMoraDuration(position, durationMs) {
        if (!utterances.count || position < 0 || position >= editorContent.pitchEditor.morae.length)
            return;
        if (!Number.isFinite(Number(durationMs)))
            return;
        editorContent.pitchEditor.setDurationAtMS(position, Number(durationMs));
    }

    readonly property var menuDialogs: ({about: dialogs.aboutDialog, rendererPackages: dialogs.rendererPackagesDialog})

    readonly property var selfTestContext: ({window: window, utterances: utterances, editorContent: editorContent, analyzeTimer: analyzeTimer})

    readonly property var intonationLabContext: ({window: window, utterances: utterances, prepareTimer: intonationLabPrepareTimer, qt: Qt})

    readonly property var historyCoreContext: ({window: window, utterances: utterances, mergeTimer: historyMergeTimer})

    readonly property var utteranceCoreContext: ({
        window: window,
        utterances: utterances,
        player: player,
        overrides: UnitOverrides,
        qt: Qt,
        platform: Platform,
        saveDialog: dialogs.saveDialog
    })

    function updateUnitOverride(unitIndex, key, value) {
        UtteranceCore.updateUnitOverride(window.utteranceCoreContext, unitIndex, key, value);
    }

    function clearUnitOverride(unitIndex) {
        UtteranceCore.clearUnitOverride(window.utteranceCoreContext, unitIndex);
    }

    function clearSynthesisView() {
        window.synthesisUnits = [];
        window.synthesisWaveformMin = [];
        window.synthesisWaveformMax = [];
        window.synthesisDurationMs = 0;
        window.synthesisLeadingMarginMs = 0;
        window.synthesisViewUtteranceId = "";
        window.synthesisViewRevision = -1;
        window.synthesisViewStale = false;
    }

    function updateSynthesisViewFromBackend(utteranceId, revision) {
        let result;
        try {
            result = JSON.parse(window.appBackend.synthesisJson || "{}");
        } catch (error) {
            window.clearSynthesisView();
            return;
        }
        if (!result || !Array.isArray(result.units)) {
            window.clearSynthesisView();
            return;
        }
        window.synthesisUnits = window.copySequence(result.units);
        window.synthesisWaveformMin = window.copySequence(result.waveform_min);
        window.synthesisWaveformMax = window.copySequence(result.waveform_max);
        window.synthesisDurationMs = Number(result.duration_ms) || 0;
        window.synthesisLeadingMarginMs = Number(result.leading_margin_ms) || 0;
        window.synthesisViewUtteranceId = String(utteranceId || "");
        window.synthesisViewRevision = Number(revision);
        window.synthesisViewStale = false;
    }

    function hasCurrentSynthesisView() {
        if (!utterances.count || !window.synthesisUnits.length || window.synthesisViewStale)
            return false;
        const item = window.current();
        return !!item
                && window.synthesisViewUtteranceId === item.utteranceId
                && window.synthesisViewRevision === item.revision;
    }

    function hasCurrentSynthesisLayout() {
        if (!utterances.count || window.synthesisViewRevision < 0)
            return false;
        const item = window.current();
        return !!item && window.synthesisViewUtteranceId === item.utteranceId;
    }

    function extendedEditorUnits(morae, durations, positions,
                                 defaultMoraDuration, defaultPauseDuration) {
        if (window.hasCurrentSynthesisView())
            return window.synthesisUnits;

        const source = window.copySequence(morae);
        const durationValues = window.copySequence(durations);
        const positionValues = window.copySequence(positions);
        const hasPositions = positionValues.length >= source.length
                && source.every((value, index) =>
                                    Number.isFinite(Number(positionValues[index])));
        const units = [];
        let fallbackStart = 0;
        for (let index = 0; index < source.length; ++index) {
            const mora = source[index] || {};
            const pause = !!mora.pause;
            const defaultDuration = Math.max(20, Number(pause
                    ? defaultPauseDuration : defaultMoraDuration) || 120);
            const start = hasPositions
                    ? Math.max(0, Number(positionValues[index]))
                    : fallbackStart;
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
                silent: pause
            });
            fallbackStart = Math.max(fallbackStart, start + duration);
        }
        return units;
    }

    function markUtteranceDirty(index, markProject) {
        if (index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        utterances.setProperty(index, "revision", item.revision + 1);
        if (markProject !== false)
            window.projectDirty = true;
        if (window.synthesisViewUtteranceId === item.utteranceId
                && window.synthesisViewRevision >= 0)
            window.synthesisViewStale = true;
        if (window.audioUtteranceId === item.utteranceId)
            clearPlayback();
    }

    function hasCurrentAudio() {
        if (!utterances.count || !window.audioUtteranceId || !player.source.toString().length)
            return false;
        const item = current();
        return item.utteranceId === window.audioUtteranceId && item.revision === window.audioRevision;
    }

    function hasCachedAudio() {
        if (!window.audioUtteranceId || !player.source.toString().length)
            return false;
        const index = window.utteranceIndex(window.audioUtteranceId);
        return index >= 0 && utterances.get(index).revision === window.audioRevision;
    }

    function stopPlaybackQueue() {
        window.playbackQueueActive = false;
        window.playbackQueue = [];
        window.playbackQueueIndex = -1;
        window.pendingUtteranceId = "";
        window.pendingRevision = -1;
    }

    function replayCachedAudio() {
        if (!window.hasCachedAudio())
            return;
        window.stopPlaybackQueue();
        window.playbackRequested = false;
        window.playbackError = "";
        player.stop();
        if (player.duration > 0)
            player.position = 0;
        player.play();
    }

    function clearPlayback(stopQueue) {
        if (stopQueue !== false)
            window.stopPlaybackQueue();
        window.playbackRequested = false;
        window.playbackError = "";
        player.stop();
        player.source = "";
        window.audioUtteranceId = "";
        window.audioRevision = -1;
    }

    function assignDefaultVoicebank(suppressDirty) {
        const voice = window.defaultVoicebank();
        if (!utterances.count || !voice)
            return;
        for (let i = 0; i < utterances.count; ++i) {
            const item = utterances.get(i);
            const resolved = window.voicebankById(item.voicebankId);
            if (resolved && resolved.id !== item.voicebankId) {
                utterances.setProperty(i, "voicebankId", resolved.id);
                utterances.setProperty(i, "imagePath", resolved.image_path || "");
                markUtteranceDirty(i, suppressDirty !== true);
            } else if (!item.voicebankId) {
                utterances.setProperty(i, "voicebankId", voice.id);
                utterances.setProperty(i, "imagePath", voice.image_path || "");
                if (!String(item.content || "").trim().length && voice.suggested_language) {
                    const language = String(voice.suggested_language);
                    utterances.setProperty(i, "language", language);
                    utterances.setProperty(i, "phonemizer", "auto");
                    utterances.setProperty(i, "modelId", window.defaultModelIdForLanguage(language));
                }
                markUtteranceDirty(i, suppressDirty !== true);
            }
        }
        selectUtterance(selectedIndex);
    }

    function assignDefaultSynthesisSettings(suppressDirty) {
        if (!utterances.count || !window.appBackend.renderers.length)
            return;
        const rendererId = window.defaultRendererId();
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            let changed = false;
            const modelId = String(item.modelId || "");
            if (modelId !== "none" && !window.modelById(modelId)) {
                utterances.setProperty(index, "modelId",
                                       window.defaultModelIdForLanguage(item.language || "ja"));
                changed = true;
            }
            if (!window.rendererById(String(item.renderer || ""))) {
                utterances.setProperty(index, "renderer", rendererId);
                changed = true;
            }
            if (!item.aliasPolicy) {
                utterances.setProperty(index, "aliasPolicy", "auto");
                changed = true;
            }
            if (changed)
                markUtteranceDirty(index, suppressDirty !== true);
        }
        selectUtterance(selectedIndex);
    }

    function selectCombo(combo, value) {
        for (let i = 0; i < combo.count; ++i) {
            if (combo.valueAt(i) === value) {
                combo.currentIndex = i;
                return true;
            }
        }
        combo.currentIndex = -1;
        return false;
    }

    function selectUtterance(index, preservePlaybackQueue) {
        if (index < 0 || index >= utterances.count)
            return;
        const changed = index !== selectedIndex;
        if (changed) {
            if (preservePlaybackQueue === true)
                clearPlayback(false);
            else
                clearPlayback();
            window.clearSynthesisView();
        }
        selectedIndex = index;
        const item = current();
        editorContent.toneField.text = item.tone;
        editorContent.moraSlider.value = item.moraDuration;
        editorContent.pauseSlider.value = item.pauseDuration;
        editorContent.leadingPreutteranceSlider.value = item.leadingPreutterance;
        editorContent.intonationSlider.value = item.intonation;
        editorContent.pitchEditor.points = window.decodeSequence(item.pointsJson);
        editorContent.pitchEditor.autoPoints = window.automaticSequence(item, "autoPointsJson");
        editorContent.pitchEditor.morae = window.decodeSequence(item.moraeJson);
        editorContent.pitchEditor.defaultMoraDuration = item.moraDuration;
        editorContent.pitchEditor.defaultPauseDuration = item.pauseDuration;
        editorContent.pitchEditor.moraDurations = window.displayedMoraDurations(item);
        editorContent.pitchEditor.moraPositions = window.displayedMoraPositions(item);
        editorContent.phonemeEditor.manualFrames = window.decodeSequence(item.pitchFramesJson);
        editorContent.phonemeEditor.autoFrames = window.automaticSequence(item, "autoPitchFramesJson");
        editorContent.phonemeEditor.frameMs = Number(item.autoFrameMs) > 0 ? Number(item.autoFrameMs) : 10;
        selectCombo(editorContent.voiceCombo, item.voicebankId);
        selectCombo(editorContent.speechLanguageCombo, item.language || "ja");
        selectCombo(editorContent.phonemizerCombo,
                    item.phonemizer || window.defaultPhonemizer(item.language || "ja"));
        Qt.callLater(function() {
            if (window.selectedIndex !== index || !utterances.count)
                return;
            const selected = window.current();
            window.selectCombo(editorContent.colorCombo,
                    window.typeIdForColor(selected.voicebankId, selected.color || ""));
        });
        selectCombo(editorContent.aliasPolicyCombo, window.normalizeAliasPolicy(item.aliasPolicy));
        selectCombo(editorContent.modelCombo, item.modelId);
        selectCombo(editorContent.rendererCombo, item.renderer);
        window.requestMissingProsodyPreview(index);
    }

    function copySequence(sequence) {
        return QmlCore.copySequence(sequence);
    }

    function decodeSequence(json) {
        return QmlCore.decodeSequence(json);
    }

    function hasManualPitch(item) {
        if (item && item.manualPitchEdited)
            return true;
        if (decodeSequence(item ? item.pointsJson : "").some(value => Math.abs(Number(value)) > .1))
            return true;
        return window.hasManualPitchFrames(item);
    }

    function hasManualMoraDurations(item) {
        if (item && item.manualMoraDurationEdited)
            return true;
        return decodeSequence(item ? item.moraDurationsJson : "").some(value => Number(value) > 0);
    }

    function automaticSequence(item, name) {
        return decodeSequence(item ? item[name] : "[]");
    }

    function automaticProsodyReady(item) {
        const moraCount = decodeSequence(item ? item.moraeJson : "[]").length;
        return moraCount > 0
                && automaticSequence(item, "autoPointsJson").length === moraCount
                && automaticSequence(item, "autoMoraDurationsJson").length === moraCount
                && automaticSequence(item, "autoMoraPositionsJson").length === moraCount;
    }

    function requestMissingProsodyPreview(index) {
        if (index < 0 || index >= utterances.count)
            return;
        window.pendingProsodyPreviewIndex = index;
        prosodyPreviewTimer.restart();
    }

    function flushProsodyPreviewRequest() {
        const index = window.pendingProsodyPreviewIndex;
        if (window.batchExportActive || index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (!item.content.trim() || window.automaticProsodyReady(item))
            return;
        if (window.pendingProsodyInFlight && window.pendingProsodyUtteranceId === item.utteranceId
                && window.pendingProsodyRevision === item.revision)
            return;
        const utteranceId = item.utteranceId;
        const revision = item.revision;
        Qt.callLater(function() {
            const currentIndex = window.utteranceIndex(utteranceId);
            if (currentIndex !== window.selectedIndex || currentIndex < 0)
                return;
            const selected = utterances.get(currentIndex);
            if (selected.revision !== revision || window.automaticProsodyReady(selected))
                return;
            if (window.pendingProsodyInFlight && window.pendingProsodyUtteranceId === utteranceId
                    && window.pendingProsodyRevision === revision)
                return;
            window.requestProsodyPreview(currentIndex);
        });
    }

    function displayedMoraDurations(item) {
        if (hasManualMoraDurations(item))
            return decodeSequence(item.moraDurationsJson);
        return automaticSequence(item, "autoMoraDurationsJson");
    }

    function displayedMoraPositions(item) {
        if (hasManualMoraDurations(item))
            return window.normalizedMoraPositions(decodeSequence(item.moraPositionsJson));
        return window.normalizedMoraPositions(automaticSequence(item, "autoMoraPositionsJson"));
    }

    function normalizedMoraPositions(positions) {
        return MoraPositions.normalizedMoraPositions(positions);
    }

    function moraStartsFromCenters(centers, durations) {
        return MoraPositions.moraStartsFromCenters(centers, durations);
    }

    function applyPronunciation(index, reading, morae) {
        const old = utterances.get(index);
        const oldPoints = window.decodeSequence(old.pointsJson);
        const oldDurations = window.decodeSequence(old.moraDurationsJson);
        const oldPositions = window.decodeSequence(old.moraPositionsJson);
        if (index === window.selectedIndex)
            window.clearSynthesisView();
        const values = [];
        const durations = [];
        const positions = oldPositions.length === morae.length ? oldPositions.slice() : [];
        for (let i = 0; i < morae.length; ++i)
            values.push(i < oldPoints.length ? oldPoints[i] : 0);
        for (let i = 0; i < morae.length; ++i)
            durations.push(i < oldDurations.length ? oldDurations[i] : 0);
        utterances.setProperty(index, "reading", reading);
        utterances.setProperty(index, "moraeJson", JSON.stringify(morae));
        utterances.setProperty(index, "pointsJson", JSON.stringify(values));
        utterances.setProperty(index, "moraDurationsJson", JSON.stringify(durations));
        utterances.setProperty(index, "moraPositionsJson", JSON.stringify(positions));
        window.clearAutomaticArrays(index);
        if (index === window.selectedIndex) {
            editorContent.pitchEditor.points = values.slice();
            editorContent.pitchEditor.autoPoints = [];
            editorContent.pitchEditor.morae = morae.slice();
            editorContent.pitchEditor.moraDurations = durations.slice();
            editorContent.pitchEditor.moraPositions = positions.slice();
        }
    }

    function clearAutomaticArrays(index) {
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "autoPointsJson", "[]");
        utterances.setProperty(index, "autoMoraDurationsJson", "[]");
        utterances.setProperty(index, "autoMoraPositionsJson", "[]");
        utterances.setProperty(index, "autoPitchFramesJson", "[]");
    }

    function applyAutomaticProsody(index, automaticPoints, automaticDurations, automaticPositions) {
        if (index < 0 || index >= utterances.count)
            return;
        const automaticStarts = window.moraStartsFromCenters(automaticPositions, automaticDurations);
        utterances.setProperty(index, "autoPointsJson", JSON.stringify(automaticPoints));
        utterances.setProperty(index, "autoMoraDurationsJson", JSON.stringify(automaticDurations));
        utterances.setProperty(index, "autoMoraPositionsJson", JSON.stringify(automaticStarts));
        if (index === window.selectedIndex) {
            const item = utterances.get(index);
            editorContent.pitchEditor.autoPoints = automaticPoints.slice();
            editorContent.pitchEditor.moraDurations = hasManualMoraDurations(item)
                    ? decodeSequence(item.moraDurationsJson) : automaticDurations.slice();
            editorContent.pitchEditor.moraPositions = window.displayedMoraPositions(item);
        }
    }

    function clearAutomaticProsody(index) {
        window.clearAutomaticArrays(index);
        if (index === window.selectedIndex) {
            const item = utterances.get(index);
            editorContent.pitchEditor.autoPoints = [];
            editorContent.pitchEditor.moraDurations = hasManualMoraDurations(item)
                    ? decodeSequence(item.moraDurationsJson) : [];
            editorContent.pitchEditor.moraPositions = window.displayedMoraPositions(item);
        }
    }

    function resetMoraDuration() {
        editorContent.moraSlider.value = 120;
        window.updateSetting("moraDuration", 120);
    }

    function resetIntonation() {
        editorContent.intonationSlider.value = window.defaultIntonationStrength;
        window.updateSetting("intonation", window.defaultIntonationStrength);
    }

    function resetPauseDuration() {
        editorContent.pauseSlider.value = 180;
        window.updateSetting("pauseDuration", 180);
    }

    function resetLeadingPreutterance() {
        editorContent.leadingPreutteranceSlider.value = 0;
        window.updateSetting("leadingPreutterance", 0);
    }

    function addUtterance(markDirty) {
        const voice = window.defaultVoicebank();
        const language = voice && voice.suggested_language
                ? String(voice.suggested_language) : "ja";
        const phonemizer = "auto";
        utterances.append({
            utteranceId: "utterance-" + nextUtteranceId++,
            content: "",
            language: language,
            phonemizer: phonemizer,
            reading: "",
            moraeJson: "[]",
            pointsJson: "[]",
            pitchFramesJson: "[]",
            moraDurationsJson: "[]",
            moraPositionsJson: "[]",
            autoPointsJson: "[]",
            autoPitchFramesJson: "[]",
            autoFrameMs: 10,
            autoMoraDurationsJson: "[]",
            autoMoraPositionsJson: "[]",
            resamplerExpressionsJson: "[]",
            phonemeOverridesJson: "[]",
            manualPitchEdited: false,
            manualMoraDurationEdited: false,
            trainingAccepted: false,
            labEntryId: "",
            voicebankId: voice ? voice.id : "",
            imagePath: voice ? voice.image_path || "" : "",
            modelId: window.metadataInitialized ? window.defaultModelIdForLanguage(language) : "",
            renderer: Platform.hasDiffsinger && voice && String(voice.kind || "") === "diffsinger"
                    ? "diffsinger" : (window.appBackend.renderers.length ? window.defaultRendererId() : ""),
            aliasPolicy: window.appBackend.defaultAliasPolicy,
            tone: window.appBackend.defaultTone,
            color: "",
            moraDuration: window.appBackend.defaultMoraDuration,
            pauseDuration: window.appBackend.defaultPauseDuration,
            leadingPreutterance: window.appBackend.defaultLeadingPreutterance,
            intonation: window.defaultIntonationStrength,
            applyPitch: true,
            revision: 0
        });
        if (markDirty !== false)
            window.projectDirty = true;
        const newIndex = utterances.count - 1;
        selectUtterance(newIndex);
        editorContent.utteranceList.positionViewAtEnd();
        Qt.callLater(() => {
            const newCard = editorContent.utteranceList.itemAtIndex(newIndex);
            if (!newCard || !newCard.textEditor)
                return;
            newCard.textEditor.forceActiveFocus();
            newCard.textEditor.selectAll();
        });
    }

    function removeUtterance() {
        if (!utterances.count)
            return;
        window.clearEditHistory();
        clearPlayback();
        utterances.remove(selectedIndex);
        window.projectDirty = true;
        if (!utterances.count) {
            selectedIndex = 0;
            editorContent.pitchEditor.points = [];
            editorContent.pitchEditor.morae = [];
            editorContent.phonemeEditor.manualFrames = [];
            editorContent.phonemeEditor.autoFrames = [];
            return;
        }
        selectedIndex = Math.min(selectedIndex, utterances.count - 1);
        selectUtterance(selectedIndex);
    }

    function moveUtterance(delta) {
        const target = selectedIndex + delta;
        if (target < 0 || target >= utterances.count)
            return;
        window.clearPlayback();
        utterances.move(selectedIndex, target, 1);
        window.projectDirty = true;
        selectedIndex = target;
        editorContent.utteranceList.positionViewAtIndex(target, ListView.Contain);
    }

    function hasPlayableTextFrom(startIndex) {
        const first = Math.max(0, Number(startIndex) || 0);
        for (let index = first; index < utterances.count; ++index) {
            if (utterances.get(index).reading.length)
                return true;
        }
        return false;
    }

    function hasExportableText() {
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            if (item.reading.length || item.content.trim().length)
                return true;
        }
        return false;
    }

    function startPlaybackQueue(startIndex) {
        if (window.appBackend.busy || window.batchExportActive || window.playbackQueueActive)
            return;
        const first = Math.max(0, Number(startIndex) || 0);
        const queue = [];
        for (let index = first; index < utterances.count; ++index) {
            if (utterances.get(index).reading.length)
                queue.push(index);
        }
        if (!queue.length)
            return;

        window.stopPlaybackQueue();
        window.clearPlayback(false);
        window.playbackQueue = queue;
        window.playbackQueueIndex = 0;
        window.playbackQueueActive = true;
        window.appBackend.clearLogs();
        window.showLogWindow();
        window.playNextPlaybackItem();
    }

    function playNextPlaybackItem() {
        if (!window.playbackQueueActive)
            return;
        if (window.playbackQueueIndex >= window.playbackQueue.length) {
            window.finishPlaybackQueue();
            return;
        }

        const index = Number(window.playbackQueue[window.playbackQueueIndex]);
        if (index < 0 || index >= utterances.count || !utterances.get(index).reading.length) {
            ++window.playbackQueueIndex;
            Qt.callLater(window.playNextPlaybackItem);
            return;
        }

        window.selectUtterance(index, true);
        window.clearPlayback(false);
        const item = utterances.get(index);
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function finishPlaybackQueue() {
        const closeLog = window.appBackend.closeLogOnSuccess;
        window.stopPlaybackQueue();
        if (closeLog)
            window.closeLogWindow();
    }

    function synthesizeCurrent() {
        const item = current();
        if (!item || !item.reading)
            return;
        autoPreviewTimer.stop();
        window.autoPreviewPending = false;
        if (window.appBackend.busy) {
            // 生成中のプレビューを使い、二重合成を避ける。
            if (window.pendingUtteranceId === item.utteranceId
                    && window.pendingRevision === item.revision)
                window.autoplayPreview = true;
            return;
        }
        clearPlayback();
        window.autoplayPreview = true;
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.clearLogs();
        window.showLogWindow();
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function scheduleAutoPreview() {
        if (!window.appBackend.autoPreviewEnabled || window.batchExportActive
                || window.saveRequestPending || window.playbackQueueActive || !utterances.count)
            return;
        const item = current();
        if (!item || !item.reading)
            return;
        window.autoPreviewPending = true;
        autoPreviewTimer.restart();
    }

    function scheduleExtendedEditorWaveform() {
        if (!editorContent.extendedPitchEditorVisible || window.hasCurrentSynthesisView())
            return;
        window.scheduleAutoPreview();
    }

    function scheduleTimingProsodyPreview(index) {
        if (window.batchExportActive || index < 0 || index >= utterances.count)
            return;
        timingProsodyTimer.utteranceId = utterances.get(index).utteranceId;
        timingProsodyTimer.restart();
    }

    function refreshPreview() {
        if (!window.appBackend.autoPreviewEnabled || window.batchExportActive
                || window.saveRequestPending || window.playbackQueueActive || !utterances.count) {
            window.autoPreviewPending = false;
            return;
        }
        if (window.appBackend.busy)
            return;
        window.autoPreviewPending = false;
        const item = current();
        if (!item || !item.reading || window.hasCurrentAudio())
            return;
        window.autoplayPreview = false;
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function seekPreview(positionMs) {
        if (!window.hasCurrentAudio())
            return;
        const duration = window.playerMedia.duration;
        const clamped = Math.max(0, Math.min(duration > 0 ? duration : positionMs, positionMs));
        window.playerMedia.position = clamped;
    }

    function quitWithoutWarning() {
        window.closeBypass = true;
        Qt.quit();
    }

    function buildSynthesisRequest(item) {
        const points = window.decodeSequence(item.pointsJson);
        const morae = window.decodeSequence(item.moraeJson);
        const manualPitch = window.hasManualPitch(item);
        const manualDurations = window.hasManualMoraDurations(item)
                ? window.decodeSequence(item.moraDurationsJson) : [];
        const request = {
            text: item.content,
            reading: item.reading || "",
            language: item.language || "ja",
            phonemizer: window.resolvedPhonemizer(item.language, item.phonemizer, item.voicebankId),
            dictionary: window.appBackend.dictionaryEntries,
            voicebank_id: item.voicebankId || editorContent.voiceCombo.currentValue,
            model_id: item.modelId,
            renderer: item.renderer,
            alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone,
            color: item.color || "",
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            leading_preutterance_ms: item.leadingPreutterance,
            mora_durations_ms: manualDurations,
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch,
            resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson),
            unit_overrides: window.decodeSequence(item.phonemeOverridesJson)
        };
        window.addRendererSettings(request, item.renderer);
        const frameOffsets = window.decodeSequence(item.pitchFramesJson);
        const hasFrameOffsets = frameOffsets.some(value => Math.abs(Number(value)) > .1);
        if (item.applyPitch && item.reading && manualPitch && hasFrameOffsets) {
            request.manual_pitch = {
                version: 1,
                reading: item.reading,
                mode: "frames",
                frames: frameOffsets.map(value => Math.max(-1200, Math.min(1200, Number(value) || 0)))
            };
        } else if (item.applyPitch && item.reading && manualPitch && points.some(value => Math.abs(Number(value)) > .1)) {
            const manualPoints = [];
            for (let index = 0; index < points.length; ++index) {
                const mora = index < morae.length ? morae[index] : null;
                if (mora && mora.pause)
                    continue;
                manualPoints.push({
                    position: index,
                    mora: mora ? mora.mora || "" : "",
                    cents: points[index]
                });
            }
            request.manual_pitch = {
                version: 1,
                reading: item.reading,
                mode: "offset",
                points: manualPoints
            };
        }
        if (window.intonationLab)
            request.model_path = window.intonationLabBaseModelPath();
        return request;
    }

    function buildProsodyRequest(item, requestId) {
        const request = {
            request_id: requestId,
            text: item.content,
            reading: item.reading || "",
            language: item.language || "ja",
            phonemizer: window.resolvedPhonemizer(item.language, item.phonemizer, item.voicebankId),
            dictionary: window.appBackend.dictionaryEntries,
            model_id: item.modelId,
            model_path: window.intonationLab ? window.intonationLabBaseModelPath() : "",
            renderer: item.renderer,
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            mora_durations_ms: window.hasManualMoraDurations(item)
                    ? window.decodeSequence(item.moraDurationsJson) : [],
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch
        };
        window.addRendererSettings(request, item.renderer);
        return request;
    }

    function requestProsodyPreview(index) {
        UtteranceCore.requestProsodyPreview(window.utteranceCoreContext, index);
    }

    function buildExportQueue(selectedOnly) {
        const queue = [];
        if (selectedOnly) {
            if (utterances.count && window.current().reading.length)
                queue.push(window.selectedIndex);
            return queue;
        }
        for (let index = 0; index < utterances.count; ++index) {
            if (utterances.get(index).reading.length)
                queue.push(index);
        }
        return queue;
    }

    function beginBatchExport(directory, mode, queue) {
        if (!directory || !directory.toString().length || !queue.length || window.appBackend.busy)
            return;
        window.batchExportDirectory = directory;
        window.batchExportMode = mode;
        window.batchExportQueue = queue;
        window.batchExportOriginalIndex = window.selectedIndex;
        window.batchExportIndex = 0;
        window.batchExportCompleted = 0;
        window.dragExportFiles = [];
        window.batchExportActive = true;
        window.batchAnalysisUtteranceId = "";
        window.clearPlayback();
        window.pendingUtteranceId = "";
        window.pendingRevision = -1;
        window.pendingProsodyRequestId = "";
        window.pendingProsodyInFlight = false;
        window.pendingProsodyUtteranceId = "";
        window.pendingProsodyRevision = -1;
        window.appBackend.clearLogs();
        window.showLogWindow();
        Qt.callLater(function() { window.synthesizeBatchItem(); });
    }

    function startBatchExport(directory) {
        const queue = [];
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            if (item.reading.length || item.content.trim().length)
                queue.push(index);
        }
        window.beginBatchExport(directory, "save", queue);
    }

    function startDragExport(directory) {
        if (!directory || !directory.toString().length)
            return;
        const queue = window.buildExportQueue(window.dragExportSelectedOnly);
        if (!queue.length)
            return;
        window.batchExportDirectory = directory;
        window.dragExportReady = false;
        window.beginBatchExport(window.batchExportDirectory, "drag", queue);
    }

    function synthesizeBatchItem() {
        if (!window.batchExportActive)
            return;
        if (window.batchAnalysisUtteranceId.length)
            return;
        if (window.appBackend.busy) {
            Qt.callLater(function() { window.synthesizeBatchItem(); });
            return;
        }
        while (window.batchExportIndex < window.batchExportQueue.length) {
            const index = window.batchExportQueue[window.batchExportIndex];
            const item = utterances.get(index);
            if (!item.reading.length) {
                window.batchAnalysisUtteranceId = item.utteranceId;
                window.analyzeUtterance(index);
                return;
            }
            ++window.batchExportIndex;
            window.selectUtterance(index);
            const requestItem = utterances.get(index);
            window.pendingUtteranceId = requestItem.utteranceId;
            window.pendingRevision = requestItem.revision;
            window.appBackend.synthesize(window.buildSynthesisRequest(requestItem));
            return;
        }
        window.finishBatchExport(true);
    }

    function finishBatchExport(success) {
        if (!window.batchExportActive)
            return;
        const wasDragExport = window.batchExportMode === "drag";
        const dragExportSucceeded = success && wasDragExport;
        const files = window.dragExportFiles.slice();
        window.batchExportActive = false;
        window.batchAnalysisUtteranceId = "";
        window.batchExportMode = "";
        window.batchExportQueue = [];
        window.pendingUtteranceId = "";
        window.pendingRevision = -1;
        if (utterances.count)
            window.selectUtterance(Math.min(window.batchExportOriginalIndex, utterances.count - 1));
        if (success && (window.appBackend.closeLogOnSuccess || wasDragExport))
            window.closeLogWindow();
        if (dragExportSucceeded && files.length) {
            window.dragExportFiles = window.dragFilesWithExo(files);
            window.dragExportReady = true;
            if (dragTargetWindow)
                window.showAuxiliaryWindow(dragTargetWindow);
        } else if (!success && wasDragExport) {
            window.dragExportReady = false;
        }
    }

    function dragFilesWithExo(files) {
        const exo = window.appBackend.writeDragExo(window.batchExportDirectory, files, window.dragExportFrameRate);
        return exo && exo.toString().length ? [exo] : files;
    }
}