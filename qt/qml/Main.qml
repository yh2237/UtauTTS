pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import QtMultimedia

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
    minimumWidth: 880
    minimumHeight: 600
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
    readonly property var licenseDocuments: injectedLegalDocuments
    readonly property real defaultIntonationStrength: appBackend.defaultIntonationStrength
    readonly property real maxIntonationStrength: 4.0

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

    Timer {
        id: autoPreviewTimer
        interval: 350
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

    property alias utterancesModel: utterances
    property alias playerMedia: player
    property alias settingsWindowRef: settingsWindow

    Translator {
        id: translatorInstance
        backend: window.appBackend
    }

    property int selectedIndex: 0
    property int nextUtteranceId: 1
    property int draggedUtteranceIndex: -1
    property string audioUtteranceId: ""
    property int audioRevision: -1
    property string pendingUtteranceId: ""
    property int pendingRevision: -1
    property string pendingProsodyRequestId: ""
    property string pendingProsodyUtteranceId: ""
    property int pendingProsodyRevision: -1
    property bool saveRequestPending: false
    property bool playbackRequested: false
    property string playbackError: ""
    property bool batchExportActive: false
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
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
                 && utterances.count > 0 && window.current().reading.length > 0
        onActivated: window.synthesizeCurrent()
    }



    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.saveProjectShortcut)
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
        onActivated: window.saveCurrentProject()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.reloadVoicebanksShortcut)
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
        onActivated: window.reloadVoicebanks()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.addUtteranceShortcut)
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.addUtterance()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.removeUtteranceShortcut)
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
                 && utterances.count > 0
        onActivated: window.removeUtterance()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.undoShortcut)
        context: Qt.ApplicationShortcut
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.undo()
        onActivatedAmbiguously: window.undo()
    }

    Shortcut {
        sequence: window.qtShortcutSequence(window.appBackend.redoShortcut)
        context: Qt.ApplicationShortcut
        enabled: !settingsWindow.visible && !window.appBackend.busy && !window.batchExportActive
                 && !window.playbackQueueActive
        onActivated: window.redo()
        onActivatedAmbiguously: window.redo()
    }

    ListModel {
        id: utterances
    }

    Timer {
        id: analyzeTimer
        interval: 250
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

    FileDialog {
        id: saveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [window.translator.tr("main.wavFilter")]
        defaultSuffix: "wav"
        onAccepted: window.appBackend.savePreview(selectedFile)
    }

    FolderDialog {
        id: saveAllDialog
        onAccepted: window.startBatchExport(selectedFolder)
    }

    FolderDialog {
        id: dragSaveDialog
        onAccepted: window.startDragExport(selectedFolder)
    }

    Dialog {
        id: frameRateDialog
        title: window.translator.tr("main.exoFrameRateTitle")
        modal: true
        width: Math.min(window.width - 40, 400)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.CloseOnEscape
        standardButtons: Dialog.Ok | Dialog.Cancel
        onAccepted: {
            window.dragExportFrameRate = frameRateSpin.value;
            dragSaveDialog.open();
        }

        contentItem: ColumnLayout {
            spacing: 12

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Label {
                    text: window.translator.tr("main.frameRate")
                }
                SpinBox {
                    id: frameRateSpin
                    Layout.preferredWidth: 120
                    from: 1
                    to: 240
                    stepSize: 1
                    value: 60
                    editable: true
                }
                Label {
                    text: window.translator.tr("main.fps")
                    color: window.mutedText
                }
                Item {
                    Layout.fillWidth: true
                }
            }
        }
    }

    DragSourceWindow {
        id: dragTargetWindow
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
        files: window.dragExportFiles
        exportDirectory: window.batchExportDirectory
        ready: window.dragExportReady
        accent: window.accent
        mutedText: window.mutedText
        onDragError: window.showAuxiliaryWindow(synthesisLogWindow)
    }

    SynthesisLogWindow {
        id: synthesisLogWindow
        hostWindow: window
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
    }

    SettingsWindow {
        id: settingsWindow
        hostWindow: window
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
        audioOutputDevices: mediaDevices.audioOutputs
        onApplyRequested: closeAfter => window.saveSettings(closeAfter)
    }

    DictionaryWindow {
        id: dictionaryWindow
        hostWindow: window
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
    }

    LicenseWindow {
        id: licenseWindow
        hostWindow: window
        hostPalette: window.palette
        translator: window.translator
        documents: window.licenseDocuments
    }

    VoicebankDetailsWindow {
        id: voicebankDetailsWindow
        hostWindow: window
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
    }

    Timer {
        id: historyMergeTimer
        interval: 700
        onTriggered: window.historyMergeKey = ""
    }

    FileDialog {
        id: projectSaveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [window.translator.tr("main.projectFilter")]
        defaultSuffix: "utautts"
        onAccepted: window.saveProjectTo(selectedFile)
        onRejected: window.closeAfterProjectSave = false
    }

    FileDialog {
        id: ustxExportFileDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [window.translator.tr("main.ustxFilter")]
        defaultSuffix: "ustx"
        onAccepted: window.exportUstxTo(selectedFile)
    }

    Dialog {
        id: rendererPackagesDialog
        title: window.translator.tr("plugins.title")
        anchors.centerIn: parent
        width: Math.min(620, window.width - 40)
        height: Math.min(540, window.height - 40)
        modal: true
        standardButtons: Dialog.Close
        contentItem: ColumnLayout {
            ScrollView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Column {
                    width: parent.width
                    spacing: 10
                    Repeater {
                        model: window.appBackend.renderers
                        delegate: Label {
                            required property var modelData
                            width: parent.width
                            wrapMode: Text.Wrap
                            text: modelData.display_name + "  " + (modelData.version || "")
                        }
                    }
                    Label {
                        width: parent.width
                        visible: window.appBackend.pluginProblems.length > 0
                        wrapMode: Text.Wrap
                        text: window.translator.tr("plugins.disabled") + "\n" + window.appBackend.pluginProblems.join("\n\n")
                    }
                }
            }
            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: window.appBackend.error
                visible: text.length > 0
            }
        }
    }

    FileDialog {
        id: projectOpenDialog
        fileMode: FileDialog.OpenFile
        nameFilters: [window.translator.tr("main.projectFilter")]
        onAccepted: window.loadProjectFrom(selectedFile)
    }

    FileDialog {
        id: diagnosticSaveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [window.translator.tr("diagnostics.filter")]
        defaultSuffix: "json"
        onAccepted: {
            const success = window.appBackend.exportDiagnosticReport(
                    selectedFile, window.diagnosticContext());
            diagnosticResultDialog.title = window.translator.tr(
                    success ? "diagnostics.successTitle" : "diagnostics.errorTitle");
            diagnosticResultDialog.text = success
                    ? window.translator.tr("diagnostics.success")
                    : window.appBackend.error;
            diagnosticResultDialog.open();
        }
    }

    MessageDialog {
        id: diagnosticResultDialog
        buttons: MessageDialog.Ok
    }

    MessageDialog {
        id: ustxExportMessageDialog
        title: window.translator.tr("main.ustxExportTitle")
        buttons: MessageDialog.Ok
    }

    Dialog {
        id: closeWarningDialog
        title: window.translator.tr("main.closeConfirmTitle")
        modal: true
        width: Math.min(window.width - 40, 460)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: window.appBackend.busy || window.batchExportActive
                      ? window.translator.tr("main.closeWhileBusy")
                      : window.translator.tr("main.closeUnsaved")
                wrapMode: Text.WordWrap
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Item {
                    Layout.fillWidth: true
                }

                Button {
                    text: window.translator.tr("main.cancel")
                    onClicked: closeWarningDialog.close()
                }

                Button {
                    text: window.translator.tr("main.saveAndQuit")
                    enabled: window.projectDirty && !window.appBackend.busy && !window.batchExportActive
                    onClicked: {
                        closeWarningDialog.close();
                        window.closeAfterProjectSave = true;
                        window.saveCurrentProject();
                    }
                }

                Button {
                    text: window.translator.tr("main.quitWithoutSaving")
                    onClicked: {
                        closeWarningDialog.close();
                        window.quitWithoutWarning();
                    }
                }
            }
        }
    }

    MessageDialog {
        id: shortcutConflictDialog
        title: window.translator.tr("main.shortcutConflictTitle")
        text: window.translator.tr("main.shortcutConflictText")
        buttons: MessageDialog.Ok
    }

    MessageDialog {
        id: projectLoadErrorDialog
        title: window.translator.tr("main.projectOpenErrorTitle")
        buttons: MessageDialog.Ok
    }

    MessageDialog {
        id: aboutDialog
        title: window.translator.tr("main.aboutTitle")
        text: window.translator.tr("main.aboutText", Qt.application.version)
        informativeText: window.translator.tr("main.aboutInformative")
        buttons: MessageDialog.Ok
    }

    Dialog {
        id: updateDialog
        title: window.translator.tr("update.title")
        modal: true
        width: Math.min(window.width - 40, 480)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.CloseOnEscape
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: window.translator.tr("update.message", window.updateAvailableVersion)
                wrapMode: Text.WordWrap
            }

            Label {
                Layout.fillWidth: true
                visible: window.updateAvailablePreRelease
                text: window.translator.tr("update.preReleaseNotice")
                wrapMode: Text.WordWrap
                color: window.mutedText
                font.pixelSize: 11
            }

            ScrollView {
                Layout.fillWidth: true
                Layout.preferredHeight: 150
                clip: true
                TextArea {
                    readOnly: true
                    wrapMode: Text.WordWrap
                    text: window.updateReleaseNotes + (window.updateReleaseNotes.length ? "\n\n" : "") + window.updateReleaseUrl
                }
            }

            Label {
                Layout.fillWidth: true
                text: window.translator.tr("update.preserveNote")
                wrapMode: Text.WordWrap
                color: window.mutedText
                font.pixelSize: 11
            }

            CheckBox {
                id: suppressUpdateVersionCheckBox
                Layout.fillWidth: true
                text: window.translator.tr("update.suppressVersion")
                checked: window.updateSuppressVersion
                onToggled: {
                    window.updateSuppressVersion = checked;
                    window.appBackend.setSuppressedUpdateVersion(checked ? window.updateAvailableVersion : "");
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Button {
                    text: window.translator.tr("update.button")
                    highlighted: true
                    onClicked: window.performUpdate()
                }
                Button {
                    text: window.translator.tr("update.openRelease")
                    onClicked: Qt.openUrlExternally(window.updateReleaseUrl)
                }
                Button {
                    text: window.translator.tr("update.later")
                    onClicked: updateDialog.close()
                }
            }
        }
    }

    Dialog {
        id: updateProgressDialog
        title: window.translator.tr("update.title")
        modal: true
        width: Math.min(window.width - 40, 440)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: window.translator.tr("update.downloading")
                wrapMode: Text.WordWrap
            }

            ProgressBar {
                Layout.fillWidth: true
                from: 0
                to: 1
                value: window.updateDownloadTotal > 0 ? window.updateDownloadReceived / window.updateDownloadTotal : 0
                indeterminate: window.updateDownloadTotal <= 0
            }

            Label {
                Layout.fillWidth: true
                text: window.updateDownloadTotal > 0
                    ? Math.floor(window.updateDownloadReceived / 1048576) + " / "
                      + Math.floor(window.updateDownloadTotal / 1048576) + " MB"
                    : ""
                color: window.mutedText
                font.pixelSize: 11
                horizontalAlignment: Text.AlignHCenter
            }

            Button {
                Layout.alignment: Qt.AlignHCenter
                text: window.translator.tr("common.cancel")
                onClicked: {
                    window.appBackend.cancelUpdateDownload();
                    updateProgressDialog.close();
                }
            }
        }
    }

    Dialog {
        id: metadataReloadDialog
        title: window.translator.tr("metadata.loading." + window.metadataReloadStage)
        modal: true
        width: Math.min(window.width - 40, 440)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            ProgressBar {
                Layout.fillWidth: true
                indeterminate: true
            }
        }
    }

    Connections {
        target: window.appBackend

        function onLanguageChanged() {
            window.translator.load(window.appBackend.resolvedLanguage());
        }

        function onUstxExportFinished(success, detail) {
            ustxExportMessageDialog.text = window.translator.tr(
                    success ? "main.ustxExportSuccess" : "main.ustxExportFailed", detail);
            ustxExportMessageDialog.open();
        }

        function onUpdateDownloadProgress(bytesReceived, bytesTotal) {
            window.updateDownloadReceived = bytesReceived;
            window.updateDownloadTotal = bytesTotal;
        }

        function onUpdateDownloadFinished(success, localZip) {
            updateProgressDialog.close();
            if (success && localZip.length
                    && window.appBackend.installUpdate(localZip, window.updateAvailableVersion))
                Qt.quit();
        }

        function onMetadataReloadStarted() {
            window.metadataReloadActive = true;
            window.metadataReloadStage = "voicebanks";
            metadataReloadDialog.open();
        }

        function onMetadataReloadStageChanged(stage) {
            window.metadataReloadStage = stage;
        }

        function onMetadataChanged() {
            if (window.metadataReloadActive) {
                window.metadataReloadActive = false;
                metadataReloadDialog.close();
            }
            const suppressDirty = !window.metadataInitialized;
            window.assignDefaultVoicebank(suppressDirty);
            window.assignDefaultSynthesisSettings(suppressDirty);
            window.metadataInitialized = true;
            if (suppressDirty)
                window.resetHistory(false);
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
            const old = utterances.get(index);
            const oldPoints = window.decodeSequence(old.pointsJson);
            const oldDurations = window.decodeSequence(old.moraDurationsJson);
            const oldPositions = window.decodeSequence(old.moraPositionsJson);
            const morae = window.copySequence(analysis.morae);
            if (index === window.selectedIndex)
                window.clearSynthesisView();
            const values = [];
            const durations = [];
            const positions = oldPositions.length === morae.length ? oldPositions.slice() : [];
            for (let i = 0; i < morae.length; ++i)
                values.push(i < oldPoints.length ? oldPoints[i] : 0);
            for (let i = 0; i < morae.length; ++i)
                durations.push(i < oldDurations.length ? oldDurations[i] : 0);
            utterances.setProperty(index, "reading", analysis.reading);
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
            if (!window.batchExportActive && index === window.selectedIndex)
                window.requestProsodyPreview(index);
        }

        function onProsodyChanged() {
            if (window.appBackend.prosodyRequestId !== window.pendingProsodyRequestId)
                return;
            const index = window.utteranceIndex(window.pendingProsodyUtteranceId);
            if (index < 0 || utterances.get(index).revision !== window.pendingProsodyRevision)
                return;
            let result;
            try {
                result = JSON.parse(window.appBackend.prosodyJson);
            } catch (error) {
                return;
            }
            const automaticPoints = window.copySequence(result.pitch_points);
            const automaticDurations = window.copySequence(result.mora_durations_ms);
            const automaticPositions = window.copySequence(result.mora_positions_ms);
            window.applyAutomaticProsody(index, automaticPoints, automaticDurations, automaticPositions);
            window.applyAutomaticFramePitch(index, window.copySequence(result.frame_pitch_cents),
                    Number(result.frame_ms) || 10);
            window.scheduleExtendedEditorWaveform();
        }

        function onPreviewReady() {
            const audio = window.appBackend.previewUrl;
            const pendingId = window.pendingUtteranceId;
            const pendingRevision = window.pendingRevision;
            const index = window.utteranceIndex(pendingId);
            if (window.batchExportActive) {
                if (index < 0 || utterances.get(index).revision !== pendingRevision) {
                    window.finishBatchExport(false);
                    return;
                }
                const fileName = window.batchExportMode === "drag"
                        ? window.dragAudioFileName(utterances.get(index), index)
                        : window.audioFileName(utterances.get(index));
                const destination = window.appBackend.fileInDirectory(
                            window.batchExportDirectory, fileName);
                window.pendingUtteranceId = "";
                window.pendingRevision = -1;
                if (!destination.toString().length || !window.appBackend.savePreview(destination)) {
                    window.finishBatchExport(false);
                    return;
                }
                ++window.batchExportCompleted;
                if (window.batchExportMode === "drag")
                    window.dragExportFiles.push(destination);
                Qt.callLater(function() { window.synthesizeBatchItem(); });
                return;
            }
            if (window.saveRequestPending) {
                if (index < 0 || utterances.get(index).revision !== pendingRevision) {
                    window.saveRequestPending = false;
                    window.pendingUtteranceId = "";
                    window.pendingRevision = -1;
                    return;
                }
                window.saveRequestPending = false;
                window.pendingUtteranceId = "";
                window.pendingRevision = -1;
                saveDialog.currentFile = window.appBackend.defaultSaveFile(window.audioFileName(utterances.get(index)));
                if (window.appBackend.closeLogOnSuccess)
                    synthesisLogWindow.close();
                saveDialog.open();
                return;
            }
            if (window.playbackQueueActive) {
                if (index < 0 || index !== window.selectedIndex || utterances.get(index).revision !== window.pendingRevision) {
                    window.stopPlaybackQueue();
                    return;
                }
                window.audioUtteranceId = window.pendingUtteranceId;
                window.audioRevision = window.pendingRevision;
                window.pendingUtteranceId = "";
                window.pendingRevision = -1;
                window.playbackError = "";
                window.playbackRequested = true;
                player.stop();
                player.source = audio;
                player.play();
                return;
            }
            if (index < 0 || index !== window.selectedIndex || utterances.get(index).revision !== window.pendingRevision) {
                window.pendingUtteranceId = "";
                window.pendingRevision = -1;
                window.autoplayPreview = true;
                window.scheduleAutoPreview();
                return;
            }
            window.updateSynthesisViewFromBackend(window.pendingUtteranceId,
                                                  window.pendingRevision);
            window.audioUtteranceId = window.pendingUtteranceId;
            window.audioRevision = window.pendingRevision;
            window.pendingUtteranceId = "";
            window.pendingRevision = -1;
            window.playbackError = "";
            player.stop();
            player.source = audio;
            if (window.autoplayPreview !== false) {
                window.playbackRequested = true;
                if (window.appBackend.closeLogOnSuccess)
                    synthesisLogWindow.close();
                player.play();
            } else {
                window.playbackRequested = false;
                window.autoplayPreview = true;
            }
        }

        function onErrorChanged() {
            if (window.metadataReloadActive && window.appBackend.error.length) {
                window.metadataReloadActive = false;
                metadataReloadDialog.close();
            }
            if (window.batchExportActive && !window.appBackend.busy
                    && window.pendingUtteranceId.length && window.appBackend.error.length)
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
        closeWarningDialog.open();
    }

    menuBar: MenuBar {
        Menu {
            title: window.translator.tr("menu.file")
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.open")
                enabled: !window.appBackend.busy && !window.batchExportActive
                onTriggered: projectOpenDialog.open()
            }
            Menu {
                id: recentProjectsMenu
                title: window.translator.tr("menu.file.recent")

                Instantiator {
                    model: window.appBackend.recentProjects
                    delegate: GrayscaleMenuItem {
                        required property string modelData
                        text: window.recentProjectLabel(modelData)
                        enabled: !window.appBackend.busy && !window.batchExportActive
                        ToolTip.visible: hovered
                        ToolTip.text: modelData
                        ToolTip.delay: 500
                        onTriggered: window.loadRecentProject(modelData)
                    }
                    onObjectAdded: (index, object) => recentProjectsMenu.insertItem(index, object)
                    onObjectRemoved: (index, object) => recentProjectsMenu.removeItem(object)
                }

                GrayscaleMenuItem {
                    text: window.translator.tr("menu.file.recent.empty")
                    enabled: false
                    visible: window.appBackend.recentProjects.length === 0
                }
                MenuSeparator {}
                GrayscaleMenuItem {
                    text: window.translator.tr("menu.file.recent.clear")
                    enabled: window.appBackend.recentProjects.length > 0
                    onTriggered: window.appBackend.clearRecentProjects()
                }
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.save")
                enabled: !window.appBackend.busy && !window.batchExportActive
                onTriggered: window.saveCurrentProject()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.saveAs")
                enabled: !window.appBackend.busy && !window.batchExportActive
                onTriggered: window.openProjectSaveDialog()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.exportUstx")
                enabled: utterances.count > 0 && !window.appBackend.busy && !window.batchExportActive
                onTriggered: window.openUstxExportDialog()
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.openVoiceDirectory")
                enabled: !window.appBackend.busy && !window.batchExportActive
                onTriggered: window.appBackend.openVoiceDirectory()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.openResamplersDirectory")
                onTriggered: window.appBackend.openClassicToolDirectory("resampler")
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.openWavtoolsDirectory")
                onTriggered: window.appBackend.openClassicToolDirectory("wavtool")
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.reloadVoicebanks")
                enabled: !window.appBackend.busy
                onTriggered: window.reloadVoicebanks()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.reloadClassicTools")
                enabled: !window.appBackend.busy
                onTriggered: window.appBackend.reloadClassicTools()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("plugins.title")
                enabled: !window.appBackend.busy && !window.batchExportActive
                onTriggered: rendererPackagesDialog.open()
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.saveWav")
                enabled: utterances.count > 0 && !window.appBackend.busy && !window.batchExportActive && window.current().reading.length > 0
                onTriggered: window.saveCurrentAudio()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.saveAllWav")
                enabled: !window.appBackend.busy && !window.batchExportActive && window.hasPlayableTextFrom(0)
                onTriggered: window.openSaveAllDialog()
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.exportExo")
                enabled: utterances.count > 0 && !window.appBackend.busy && !window.batchExportActive && window.current().reading.length > 0
                onTriggered: window.openDragExportDialog(true)
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.exportAllExo")
                enabled: !window.appBackend.busy && !window.batchExportActive && window.hasPlayableTextFrom(0)
                onTriggered: window.openDragExportDialog(false)
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.file.quit")
                onTriggered: Qt.quit()
            }
        }
        Menu {
            title: window.translator.tr("menu.edit")
            GrayscaleMenuItem {
                text: window.translator.tr("menu.edit.undo")
                enabled: window.canUndo && !window.appBackend.busy && !window.batchExportActive
                         && !window.playbackQueueActive
                onTriggered: window.undo()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.edit.redo")
                enabled: window.canRedo && !window.appBackend.busy && !window.batchExportActive
                         && !window.playbackQueueActive
                onTriggered: window.redo()
            }
        }
        Menu {
            title: window.translator.tr("menu.playback")
            GrayscaleMenuItem {
                text: window.translator.tr("menu.playback.current")
                enabled: utterances.count > 0 && !window.appBackend.busy && !window.batchExportActive
                         && !window.playbackQueueActive && window.current().reading.length > 0
                onTriggered: window.synthesizeCurrent()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.playback.all")
                enabled: !window.appBackend.busy && !window.batchExportActive && !window.playbackQueueActive
                         && window.hasPlayableTextFrom(0)
                onTriggered: window.startPlaybackQueue(0)
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.playback.fromSelected")
                enabled: !window.appBackend.busy && !window.batchExportActive && !window.playbackQueueActive
                         && window.hasPlayableTextFrom(window.selectedIndex)
                onTriggered: window.startPlaybackQueue(window.selectedIndex)
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: window.translator.tr("menu.playback.replay")
                enabled: !window.appBackend.busy && !window.batchExportActive && !window.playbackQueueActive
                         && window.hasCachedAudio()
                onTriggered: window.replayCachedAudio()
            }
        }
        Menu {
            title: window.translator.tr("menu.settings")
            GrayscaleMenuItem {
                text: window.translator.tr("menu.settings.settings")
                onTriggered: {
                    window.showAuxiliaryWindow(settingsWindow);
                    settingsWindow.loadCurrent();
                }
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.settings.dictionary")
                onTriggered: window.openDictionarySettings()
            }
        }
        Menu {
            title: window.translator.tr("menu.help")
            GrayscaleMenuItem {
                text: window.translator.tr("menu.help.about")
                onTriggered: {
                    if (!window.appBackend.showNativeAboutDialog())
                        aboutDialog.open();
                }
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.help.repository")
                onTriggered: Qt.openUrlExternally(window.repositoryUrl)
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.help.license")
                onTriggered: window.showAuxiliaryWindow(licenseWindow)
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.help.voicebankDetails")
                enabled: window.appBackend.voicebanks.length > 0
                onTriggered: window.showVoicebankDetails()
            }
            GrayscaleMenuItem {
                text: window.translator.tr("menu.help.exportDiagnostics")
                onTriggered: {
                    diagnosticSaveDialog.currentFile = window.appBackend.defaultSaveFile(
                            "utautts-diagnostics.json");
                    diagnosticSaveDialog.open();
                }
            }
        }
    }


    header: ToolBar {
        visible: window.intonationLab
        height: visible ? 44 : 0

        RowLayout {
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

    function current() {
        return utterances.get(selectedIndex);
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
            speech_timing: !!item.speechTiming,
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
        voicebankDetailsWindow.currentIndex = Math.max(0, Math.min(voicebankDetailsWindow.currentIndex, window.appBackend.voicebanks.length - 1));
        window.showAuxiliaryWindow(voicebankDetailsWindow);
    }

    function saveSettings(closeAfter) {
        const shortcuts = [settingsWindow.pendingSynthesizeShortcut,
                           settingsWindow.pendingSaveProjectShortcut,
                           settingsWindow.pendingReloadVoicebanksShortcut,
                           settingsWindow.pendingAddUtteranceShortcut,
                           settingsWindow.pendingRemoveUtteranceShortcut,
                           settingsWindow.pendingUndoShortcut,
                           settingsWindow.pendingRedoShortcut];
        const usedShortcuts = [];
        for (let index = 0; index < shortcuts.length; ++index) {
            const shortcut = String(shortcuts[index] || "").trim();
            if (!shortcut.length)
                continue;
            const normalized = window.qtShortcutSequence(shortcut).toLowerCase();
            if (usedShortcuts.indexOf(normalized) >= 0) {
                shortcutConflictDialog.open();
                return;
            }
            usedShortcuts.push(normalized);
        }
        window.appBackend.setSynthesisDefaults(settingsWindow.pendingDefaultModelId,
                                               settingsWindow.pendingDefaultRendererId,
                                               settingsWindow.pendingDefaultTone,
                                               settingsWindow.pendingDefaultAliasPolicy);
        window.appBackend.setDarkMode(settingsWindow.pendingDarkMode);
        window.appBackend.setLanguage(settingsWindow.pendingLanguage);
        window.appBackend.setFfmpegPath(settingsWindow.pendingFfmpegPath);
        window.appBackend.setAudioOutputDeviceId(settingsWindow.pendingAudioOutputDeviceId);
        window.appBackend.setCloseLogOnSuccess(settingsWindow.pendingCloseLogOnSuccess);
        window.appBackend.setUpdateCheckEnabled(settingsWindow.pendingUpdateCheckEnabled);
        window.appBackend.setPreReleaseUpdateCheckEnabled(
                    settingsWindow.pendingPreReleaseUpdateCheckEnabled);
        window.appBackend.setPreviewCacheFileCount(settingsWindow.pendingPreviewCacheFileCount);
        window.appBackend.setAutoPreviewEnabled(settingsWindow.pendingAutoPreviewEnabled);
        window.appBackend.setExtendedDetailsVisible(settingsWindow.pendingExtendedDetailsVisible);
        window.appBackend.setDefaultVoicebank(settingsWindow.pendingDefaultVoicebankId);
        window.appBackend.setExportSettings(settingsWindow.pendingExportTextWithWav,
                                            settingsWindow.pendingExportLabWithWav,
                                            settingsWindow.pendingExportTextEncoding);
        window.appBackend.setShortcutSequences(settingsWindow.pendingSynthesizeShortcut,
                                               settingsWindow.pendingSaveProjectShortcut,
                                               settingsWindow.pendingReloadVoicebanksShortcut,
                                               settingsWindow.pendingAddUtteranceShortcut,
                                               settingsWindow.pendingRemoveUtteranceShortcut,
                                               settingsWindow.pendingUndoShortcut,
                                               settingsWindow.pendingRedoShortcut);
        if (closeAfter) {
            settingsWindow.close();
            settingsWindow.visible = false;
        }
    }

    function showAuxiliaryWindow(auxiliaryWindow) {
        auxiliaryWindow.visible = true;
        auxiliaryWindow.raise();
        auxiliaryWindow.requestActivate();
    }

    function versionParts(version) {
        const match = /(\d+)(?:\.(\d+))?(?:\.(\d+))?/.exec(String(version));
        if (!match)
            return null;
        const parts = [];
        for (let index = 1; index <= 3; ++index)
            parts.push(match[index] ? parseInt(match[index], 10) : 0);
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
            updateDialog.open();
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
            updateDialog.close();
            updateProgressDialog.open();
        } else {
            updateDialog.close();
            Qt.openUrlExternally(window.updateReleaseUrl);
        }
    }

    function openSettings() {
        settingsWindow.loadCurrent();
        showAuxiliaryWindow(settingsWindow);
    }

    function openDictionarySettings() {
        dictionaryWindow.loadCurrent();
        showAuxiliaryWindow(dictionaryWindow);
    }

    function voicebankById(id) {
        for (let i = 0; i < window.appBackend.voicebanks.length; ++i)
            if (window.appBackend.voicebanks[i].id === id)
                return window.appBackend.voicebanks[i];
        return null;
    }

    function reloadVoicebanks() {
        if (window.appBackend.busy || window.batchExportActive)
            return;
        window.clearPlayback();
        window.appBackend.reloadVoicebanks();
    }

    function voicebankTypeOptions(id, selectedColor) {
        const voice = window.voicebankById(id);
        const raw = voice && voice.types ? voice.types : [];
        const options = [];
        for (let index = 0; index < raw.length; ++index) {
            const source = raw[index] || {};
            const color = String(source.color || "");
            const optionId = String(source.id || ("subbank-" + index));
            options.push({
                id: optionId,
                color: color,
                display_name: color.length ? color : window.translator.tr("main.color.default")
            });
        }
        if (!options.length) {
            options.push({
                id: "__default__",
                color: "",
                display_name: window.translator.tr("main.color.default")
            });
        }

        // 現メタデータにない旧プロジェクトの音源タイプも復元できるよう残す。
        if (selectedColor !== undefined && selectedColor !== null) {
            const selected = String(selectedColor || "");
            let found = false;
            for (let index = 0; index < options.length; ++index) {
                if (options[index].color === selected) {
                    found = true;
                    break;
                }
            }
            if (selected.length && !found) {
                options.push({
                    id: "__custom__:" + selected,
                    color: selected,
                    display_name: selected
                });
            }
        }
        return options;
    }

    function voicebankTypeOptionAt(id, index, selectedColor) {
        const options = window.voicebankTypeOptions(id, selectedColor);
        return index >= 0 && index < options.length ? options[index] : null;
    }

    function voicebankHasColor(id, color) {
        const target = String(color || "");
        const voice = window.voicebankById(id);
        const raw = voice && voice.types ? voice.types : [];
        if (!raw.length)
            return target === "";
        for (let index = 0; index < raw.length; ++index) {
            if (String((raw[index] || {}).color || "") === target)
                return true;
        }
        return false;
    }

    function typeIdForColor(id, color) {
        const target = String(color || "");
        const options = window.voicebankTypeOptions(id, target);
        for (let index = 0; index < options.length; ++index) {
            if (options[index].color === target)
                return options[index].id;
        }
        return options.length ? options[0].id : "";
    }

    function defaultVoicebank() {
        const configured = String(window.appBackend.defaultVoicebankId || "");
        const selected = configured.length ? window.voicebankById(configured) : null;
        return selected || (window.appBackend.voicebanks.length ? window.appBackend.voicebanks[0] : null);
    }

    function modelById(id) {
        for (let i = 0; i < window.appBackend.models.length; ++i)
            if (window.appBackend.models[i].id === id)
                return window.appBackend.models[i];
        return null;
    }

    function rendererById(id) {
        for (let i = 0; i < window.appBackend.renderers.length; ++i)
            if (window.appBackend.renderers[i].id === id)
                return window.appBackend.renderers[i];
        return null;
    }

    function defaultModelId() {
        const configured = String(window.appBackend.defaultModelId || "none");
        return configured === "none" || window.modelById(configured)
                ? configured : (window.appBackend.models.length ? window.appBackend.models[0].id : "none");
    }

    function defaultModelIdForLanguage(language) {
        const normalized = String(language || "ja").toLowerCase();
        if (normalized === "en") {
            if (String(window.appBackend.defaultModelId || "none") === "none")
                return "none";
            for (let index = 0; index < window.appBackend.models.length; ++index) {
                const model = window.appBackend.models[index];
                if (String(model.language || "").toLowerCase() === "en")
                    return model.id;
            }
            return "none";
        }
        if (normalized !== "ja")
            return "none";
        const selected = window.defaultModelId();
        if (selected === "none")
            return "none";
        const configured = window.modelById(selected);
        if (!configured || !String(configured.language || "").trim()
                || String(configured.language).toLowerCase() === "ja")
            return configured ? configured.id : "none";
        for (let index = 0; index < window.appBackend.models.length; ++index) {
            const model = window.appBackend.models[index];
            if (!String(model.language || "").trim()
                    || String(model.language).toLowerCase() === "ja")
                return model.id;
        }
        return "none";
    }

    function preferredRendererForModel(model) {
        const recommended = model && model.recommended_renderers ? model.recommended_renderers : [];
        for (let index = 0; index < recommended.length; ++index) {
            const renderer = window.rendererById(recommended[index]);
            if (renderer)
                return renderer.id;
        }
        return window.defaultRendererId();
    }

    function defaultRendererId() {
        const configured = String(window.appBackend.defaultRenderer || "");
        if (window.rendererById(configured))
            return configured;
        const available = window.appBackend.renderers;
        return available.length ? available[0].id : "";
    }

    function normalizeRendererId(id) {
        const rendererId = String(id || "");
        if (rendererId && window.rendererById(rendererId))
            return rendererId;
        return window.defaultRendererId();
    }

    function normalizeAliasPolicy(value) {
        const policy = String(value || "auto");
        return ["auto", "cvvc-enhanced", "vcv-prefer", "cvvc-prefer", "cv-only"].indexOf(policy) >= 0 ? policy : "auto";
    }

    function utteranceIndex(id) {
        for (let i = 0; i < utterances.count; ++i)
            if (utterances.get(i).utteranceId === id)
                return i;
        return -1;
    }

    function voicebankName(id) {
        const voice = voicebankById(id);
        return voice ? voice.name : window.translator.tr("main.voicebankNone");
    }

    function fileNamePart(value, fallback) {
        let result = String(value === undefined || value === null ? "" : value)
                .replace(/[<>:"\/\\|?*\x00-\x1F]/g, " ")
                .replace(/\s+/g, " ")
                .trim();
        while (result.endsWith(".") || result.endsWith(" "))
            result = result.slice(0, -1).trim();
        return result || fallback;
    }

    function audioFileName(item) {
        const voice = fileNamePart(window.voicebankName(item.voicebankId), "voicebank");
        const text = fileNamePart(item.content, "utterance-" + item.utteranceId);
        return voice + "_" + text + ".wav";
    }

    function dragAudioFileName(item, index) {
        const number = ("000" + String(index + 1)).slice(-3);
        return number + "_" + window.audioFileName(item);
    }

    function saveCurrentAudio() {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive || !window.current().reading.length)
            return;
        const item = window.current();
        window.clearPlayback();
        window.saveRequestPending = true;
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.clearLogs();
        window.showAuxiliaryWindow(synthesisLogWindow);
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function openSaveAllDialog() {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive)
            return;
        saveAllDialog.open();
    }

    function openDragExportDialog(selectedOnly) {
        if (!utterances.count || window.appBackend.busy || window.batchExportActive)
            return;
        if (selectedOnly && !window.current().reading.length)
            return;
        window.dragExportSelectedOnly = selectedOnly;
        frameRateDialog.open();
    }

    function projectNumber(value, fallback, minimum, maximum, integer) {
        const parsed = Number(value);
        if (!Number.isFinite(parsed))
            return fallback;
        const normalized = integer ? Math.round(parsed) : parsed;
        return Math.max(minimum, Math.min(maximum, normalized));
    }

    function historyUtterance(item) {
        return {
            utteranceId: item.utteranceId,
            pointsJson: item.pointsJson,
            pitchFramesJson: item.pitchFramesJson,
            moraDurationsJson: item.moraDurationsJson,
            moraPositionsJson: item.moraPositionsJson,
            phonemeOverridesJson: item.phonemeOverridesJson,
            manualPitchEdited: item.manualPitchEdited,
            manualMoraDurationEdited: item.manualMoraDurationEdited
        };
    }

    function historySnapshot() {
        const items = [];
        for (let index = 0; index < utterances.count; ++index)
            items.push(window.historyUtterance(utterances.get(index)));
        return JSON.stringify(items);
    }

    function editableFingerprint() {
        const items = [];
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            items.push({
                content: item.content,
                voicebankId: item.voicebankId,
                modelId: item.modelId,
                renderer: item.renderer,
                aliasPolicy: item.aliasPolicy,
                tone: item.tone,
                color: item.color,
                moraDuration: item.moraDuration,
                speechTiming: !!item.speechTiming,
                pauseDuration: item.pauseDuration,
                leadingPreutterance: item.leadingPreutterance,
                intonation: item.intonation,
                applyPitch: item.applyPitch,
                pointsJson: item.manualPitchEdited ? item.pointsJson : "[]",
                pitchFramesJson: item.manualPitchEdited ? item.pitchFramesJson : "[]",
                moraDurationsJson: item.manualMoraDurationEdited ? item.moraDurationsJson : "[]",
                moraPositionsJson: item.manualMoraDurationEdited ? item.moraPositionsJson : "[]",
                phonemeOverridesJson: item.phonemeOverridesJson || "[]",
                manualPitchEdited: item.manualPitchEdited,
                manualMoraDurationEdited: item.manualMoraDurationEdited
            });
        }
        return JSON.stringify(items);
    }

    function limitedHistory(stack, snapshot) {
        const result = stack.slice();
        result.push(snapshot);
        if (result.length > 100)
            result.shift();
        return result;
    }

    function beginHistoryChange(key, merge) {
        if (window.historyRestoring)
            return;
        const normalizedKey = String(key || "");
        if (merge && normalizedKey.length && window.historyMergeKey === normalizedKey) {
            historyMergeTimer.restart();
            return;
        }
        window.undoStack = window.limitedHistory(window.undoStack, window.historySnapshot());
        window.redoStack = [];
        window.historyMergeKey = merge ? normalizedKey : "";
        if (merge)
            historyMergeTimer.restart();
        else
            historyMergeTimer.stop();
    }

    function endHistoryGesture() {
        window.historyMergeKey = "";
        historyMergeTimer.stop();
    }

    function resetHistory(markDirty) {
        window.undoStack = [];
        window.redoStack = [];
        window.endHistoryGesture();
        window.savedProjectFingerprint = markDirty ? "__unsaved_project__" : window.editableFingerprint();
        window.projectDirty = !!markDirty;
    }

    function clearEditHistory() {
        window.undoStack = [];
        window.redoStack = [];
        window.endHistoryGesture();
    }

    function restoreHistorySnapshot(snapshot) {
        let savedItems;
        try {
            savedItems = JSON.parse(snapshot);
        } catch (error) {
            return false;
        }
        if (!Array.isArray(savedItems))
            return false;

        window.historyRestoring = true;
        window.clearPlayback();
        window.pendingProsodyRequestId = "";
        window.pendingProsodyUtteranceId = "";
        window.pendingProsodyRevision = -1;
        for (let savedIndex = 0; savedIndex < savedItems.length; ++savedIndex) {
            const saved = savedItems[savedIndex];
            const index = window.utteranceIndex(String(saved.utteranceId || ""));
            if (index < 0)
                continue;
            const item = utterances.get(index);
            if (item.pointsJson === saved.pointsJson
                    && item.pitchFramesJson === saved.pitchFramesJson
                    && item.moraDurationsJson === saved.moraDurationsJson
                    && item.moraPositionsJson === saved.moraPositionsJson
                    && item.phonemeOverridesJson === saved.phonemeOverridesJson
                    && item.manualPitchEdited === !!saved.manualPitchEdited
                    && item.manualMoraDurationEdited === !!saved.manualMoraDurationEdited)
                continue;
            utterances.setProperty(index, "pointsJson", String(saved.pointsJson || "[]"));
            utterances.setProperty(index, "pitchFramesJson", String(saved.pitchFramesJson || "[]"));
            utterances.setProperty(index, "moraDurationsJson", String(saved.moraDurationsJson || "[]"));
            utterances.setProperty(index, "moraPositionsJson", String(saved.moraPositionsJson || "[]"));
            utterances.setProperty(index, "phonemeOverridesJson",
                                   String(saved.phonemeOverridesJson || "[]"));
            utterances.setProperty(index, "manualPitchEdited", !!saved.manualPitchEdited);
            utterances.setProperty(index, "manualMoraDurationEdited", !!saved.manualMoraDurationEdited);
            window.markUtteranceDirty(index, false);
        }
        if (utterances.count)
            window.selectUtterance(window.selectedIndex);
        window.historyRestoring = false;
        window.projectDirty = window.editableFingerprint() !== window.savedProjectFingerprint;
        return true;
    }

    function undo() {
        if (!window.canUndo)
            return;
        window.endHistoryGesture();
        const previous = window.undoStack[window.undoStack.length - 1];
        const remaining = window.undoStack.slice(0, window.undoStack.length - 1);
        const currentSnapshot = window.historySnapshot();
        if (!window.restoreHistorySnapshot(previous))
            return;
        window.undoStack = remaining;
        window.redoStack = window.limitedHistory(window.redoStack, currentSnapshot);
    }

    function redo() {
        if (!window.canRedo)
            return;
        window.endHistoryGesture();
        const next = window.redoStack[window.redoStack.length - 1];
        const remaining = window.redoStack.slice(0, window.redoStack.length - 1);
        const currentSnapshot = window.historySnapshot();
        if (!window.restoreHistorySnapshot(next))
            return;
        window.redoStack = remaining;
        window.undoStack = window.limitedHistory(window.undoStack, currentSnapshot);
    }

    function runInterfaceSelfTest() {
        if (!window.injectedSelfTest)
            return "self-test mode is disabled";
        function check(condition, message) {
            return condition ? "" : message;
        }

        if (window.intonationLab) {
            let error = check(utterances.count === 50, "intonation lab examples were not loaded");
            if (error.length)
                return error;
            error = check(window.selectedIndex === 0
                          && window.current().content.length > 0
                          && window.intonationLabFirstIncomplete() === 0,
                          "intonation lab did not select the first example");
            return error;
        }

        let error = check(utterances.count === 1, "initial utterance is missing");
        if (error.length)
            return error;
        error = check(window.current().speechTiming === false
                      && window.current().phonemizer === "auto"
                      && window.buildSynthesisRequest(window.current()).phonemizer !== "auto",
                      "normal GUI defaults are incorrect");
        if (error.length)
            return error;
        const contextRequest = window.buildSynthesisRequest(window.current());
        const contextSettings = contextRequest.renderer_settings || {};
        error = check(contextSettings.context_duration === true
                      && contextSettings.context_duration_strength === 1.0,
                      "context duration settings were not injected into the request");
        if (error.length)
            return error;
        error = check(contextSettings.boundary_tone === true
                      && contextSettings.boundary_tone_strength === 1.0,
                      "boundary tone settings were not injected into the request");
        if (error.length)
            return error;
        error = check(contextSettings.stretch_adapt === true
                      && contextSettings.stretch_adapt_strength === 1.0,
                      "stretch adaptation settings were not injected into the request");
        if (error.length)
            return error;
        error = check(contextSettings.english_weak_form === true,
                      "English weak form setting was not injected into the request");
        if (error.length)
            return error;
        for (let voiceIndex = 0; voiceIndex < window.appBackend.voicebanks.length; ++voiceIndex) {
            const voice = window.appBackend.voicebanks[voiceIndex];
            if (!voice.suggested_language || !voice.suggested_phonemizer)
                continue;
            error = check(window.resolvedPhonemizer(voice.suggested_language, "auto", voice.id)
                          === voice.suggested_phonemizer,
                          "automatic phoneme format detection failed");
            if (error.length)
                return error;
        }
        const rendererId = window.defaultRendererId();
        const originalDefaultMoraDuration = window.appBackend.defaultMoraDuration;
        window.appBackend.setRendererSetting(rendererId, "mora_duration_ms",
                                             originalDefaultMoraDuration + 5);
        error = check(window.current().moraDuration === originalDefaultMoraDuration,
                      "changing renderer settings changed the current utterance");
        if (error.length)
            return error;
        window.addUtterance(false);
        error = check(window.current().moraDuration === originalDefaultMoraDuration + 5,
                      "new utterance did not use the renderer setting default");
        if (error.length)
            return error;
        window.removeUtterance();
        window.appBackend.setRendererSetting(rendererId, "mora_duration_ms",
                                             originalDefaultMoraDuration);
        const originalContextDuration = window.appBackend.rendererSetting(rendererId, "context_duration", true);
        window.appBackend.setRendererSetting(rendererId, "context_duration", false);
        error = check(window.buildSynthesisRequest(window.current()).renderer_settings.context_duration === false,
                      "context duration override was not injected into the request");
        if (error.length)
            return error;
        window.appBackend.setRendererSetting(rendererId, "context_duration", originalContextDuration);
        window.resetHistory(false);
        error = check(utterances.get(0).intonation === window.defaultIntonationStrength,
                      "initial intonation strength is incorrect");
        if (error.length)
            return error;
        window.updateUtteranceText(0, "こんにちは");
        analyzeTimer.stop();
        window.updatePitchPoints([20, -10]);
        error = check(window.canUndo && window.current().manualPitchEdited,
                      "pitch edit was not recorded");
        if (error.length)
            return error;
        window.undo();
        error = check(window.current().content === "こんにちは" && !window.current().manualPitchEdited,
                      "pitch undo changed text or kept the edit");
        if (error.length)
            return error;
        window.redo();
        error = check(window.current().content === "こんにちは" && window.current().manualPitchEdited,
                      "pitch redo failed");
        if (error.length)
            return error;

        window.updateMoraTiming([110, 130], [0, 110]);
        error = check(window.current().manualMoraDurationEdited, "mora timing edit was not recorded");
        if (error.length)
            return error;
        window.undo();
        error = check(!window.current().manualMoraDurationEdited, "mora timing undo failed");
        if (error.length)
            return error;
        window.redo();
        error = check(window.current().manualMoraDurationEdited, "mora timing redo failed");
        if (error.length)
            return error;

        editorContent.pitchEditor.morae = [{mora: "あ", pause: false}, {mora: "", pause: true}];
        editorContent.pitchEditor.moraDurations = [120, 180];
        editorContent.pitchEditor.moraPositions = [0, 120];
        error = check(editorContent.pitchEditor.durationIsEditable(1),
                      "pause duration is not editable");
        if (error.length)
            return error;
        editorContent.pitchEditor.updateEndPositionAt(
                    editorContent.pitchEditor.sidePadding
                    + 380 * editorContent.pitchEditor.durationScale);
        error = check(Math.round(editorContent.pitchEditor.durationAt(1)) === 260,
                      "pause duration edit failed");
        if (error.length)
            return error;

        window.addUtterance();
        error = check(utterances.count === 2, "utterance add failed");
        if (error.length)
            return error;
        utterances.setProperty(0, "moraeJson", JSON.stringify([{mora: "こ", pause: false}]));
        utterances.setProperty(0, "pointsJson", "[0]");
        utterances.setProperty(0, "autoPointsJson", "[75]");
        utterances.setProperty(0, "autoMoraDurationsJson", "[120]");
        utterances.setProperty(0, "autoMoraPositionsJson", "[0]");
        window.selectUtterance(0);
        error = check(Math.round(editorContent.pitchEditor.pitchAt(0)) === 75,
                      "card switch did not restore automatic prosody");
        if (error.length)
            return error;
        window.selectUtterance(1);
        window.moveUtterance(-1);
        error = check(window.selectedIndex === 0, "utterance move failed");
        if (error.length)
            return error;
        window.removeUtterance();
        error = check(utterances.count === 1, "utterance remove failed");
        if (error.length)
            return error;
        utterances.setProperty(0, "speechTiming", true);
        error = check(window.buildSynthesisRequest(window.current()).speech_timing === true
                      && window.buildProsodyRequest(window.current(), "speech-self-test").speech_timing === true,
                      "speech timing was lost from native requests");
        if (error.length)
            return error;
        const project = window.projectData();
        error = check(project.format === "utautts-project" && project.format_version === 8
                      && project.utterances.length === 1
                      && project.utterances[0].speech_timing === true, "project data generation failed");

        analyzeTimer.stop();
        utterances.clear();
        window.nextUtteranceId = 1;
        window.addUtterance(false);
        window.resetHistory(false);
        return error;
    }


    function intonationLabDefaultFile() {
        const stamp = new Date().toISOString().replace(/[-:.TZ]/g, "");
        return window.appBackend.defaultSaveFile("intonation-lab-" + stamp + ".utautts");
    }

    function intonationLabBaseModelPath() {
        return "models/frame-intonation-v9-t.json";
    }

    function intonationLabFirstIncomplete() {
        for (let index = 0; index < utterances.count; ++index) {
            if (!utterances.get(index).trainingAccepted)
                return index;
        }
        return -1;
    }

    function prepareIntonationLabEntry(index) {
        if (!window.intonationLab || index < 0 || index >= utterances.count)
            return;
        if (window.appBackend.busy || !window.metadataInitialized) {
            window.intonationLabPendingIndex = index;
            window.intonationLabStatus = "例文を準備しています。";
            intonationLabPrepareTimer.restart();
            return;
        }
        window.intonationLabPendingIndex = -1;
        window.selectUtterance(index);
        const item = window.current();
        if (item.reading.length)
            window.requestMissingProsodyPreview(index);
        else
            window.analyzeUtterance(index);
    }

    function initializeIntonationLab() {
        if (!window.intonationLab || window.intonationLabInitialized)
            return;
        window.intonationLabInitialized = true;
        window.projectFile = window.intonationLabDefaultFile();
        utterances.clear();
        window.nextUtteranceId = 1;

        try {
            const examples = JSON.parse(window.injectedIntonationLabExamples);
            if (!Array.isArray(examples) || !examples.length)
                throw new Error("例文がありません。");
            for (const example of examples) {
                window.addUtterance(false);
                const index = utterances.count - 1;
                utterances.setProperty(index, "content", String(example.text || ""));
                utterances.setProperty(index, "labEntryId", String(example.id || "entry-" + (index + 1)));
                utterances.setProperty(index, "trainingAccepted", false);
            }
            window.selectedIndex = 0;
            window.projectDirty = false;
            window.resetHistory(false);
            window.intonationLabStatus = "例文を準備しています。";
            window.prepareIntonationLabEntry(0);
        } catch (error) {
            window.intonationLabStatus = String(error);
        }
    }
    function completeIntonationLabEntry() {
        if (!window.intonationLab || window.appBackend.busy || !utterances.count)
            return;
        const item = window.current();
        if (!item || !item.reading.length)
            return;
        utterances.setProperty(window.selectedIndex, "trainingAccepted", true);
        window.projectDirty = true;
        if (!window.projectFile.toString().length)
            window.projectFile = window.intonationLabDefaultFile();
        if (!window.appBackend.saveProject(window.projectFile, window.projectData())) {
            window.intonationLabStatus = "書き出しに失敗しました。";
            return;
        }
        window.appBackend.rememberRecentProject(window.projectFile);
        window.projectDirty = false;
        const next = window.intonationLabFirstIncomplete();
        if (next < 0) {
            window.intonationLabStatus = "全ての例文を書き出しました。";
            return;
        }
        window.intonationLabStatus = "書き出しました。次の文を準備しています。";
        Qt.callLater(function() { window.prepareIntonationLabEntry(next); });
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
                speech_timing: !!item.speechTiming,
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
        projectSaveDialog.currentFile = window.projectFile.toString().length
                ? window.projectFile : window.appBackend.defaultSaveFile("untitled.utautts");
        projectSaveDialog.open();
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
        ustxExportFileDialog.currentFile = window.appBackend.defaultSaveFile("untitled.ustx");
        ustxExportFileDialog.open();
    }

    function exportUstxTo(destination) {
        if (!destination || !destination.toString().length)
            return;
        window.appBackend.exportUstx(destination, window.projectData());
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
            projectLoadErrorDialog.text = project && project._error !== undefined
                    ? String(project._error) : window.translator.tr("main.projectLoadError");
            projectLoadErrorDialog.open();
            return;
        }
        if (project.utterances === undefined || project.utterances === null) {
            projectLoadErrorDialog.text = window.translator.tr("main.projectNoUtterances");
            projectLoadErrorDialog.open();
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
                speechTiming: saved.speech_timing === undefined ? false : saved.speech_timing === true,
                pauseDuration: window.projectNumber(saved.pause_duration_ms, window.appBackend.defaultPauseDuration, 0, 3000, true),
                leadingPreutterance: window.projectNumber(saved.leading_preutterance_ms, 0, 0, 300, true),
                intonation: window.projectNumber(saved.intonation, window.defaultIntonationStrength, 0, window.maxIntonationStrength, false),
                applyPitch: saved.apply_pitch === undefined ? true : !!saved.apply_pitch,
                revision: 0
            });
        }

        window.projectDirty = migratedRenderer;
        window.projectFile = source;
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
        return path ? encodeURI("file:///" + path.replace(/\\/g, "/")) : "";
    }

    function defaultPhonemizer(language) {
        if (language === "en")
            return "en-arpasing";
        if (language === "zh")
            return "zh-cvvc";
        return "ja-kana";
    }

    function phonemizerOptions(language) {
        const labels = {
            "auto": window.translator.tr("main.phonemizer.auto"),
            "ja-kana": window.translator.tr("main.phonemizer.jaKana"),
            "en-arpasing": window.translator.tr("main.phonemizer.enArpasing"),
            "en-delta": window.translator.tr("main.phonemizer.enDelta"),
            "en-vccv": window.translator.tr("main.phonemizer.enVccv"),
            "en-cv": window.translator.tr("main.phonemizer.enCv"),
            "zh-cvvc": window.translator.tr("main.phonemizer.zhCvvc")
        };
        if (language === "en")
            return ["auto", "en-arpasing", "en-delta", "en-vccv", "en-cv"].map(
                        id => ({id: id, display_name: labels[id]}));
        const id = window.defaultPhonemizer(language);
        return ["auto", id].map(value => ({id: value, display_name: labels[value]}));
    }

    function resolvedPhonemizer(language, phonemizer, voicebankId) {
        language = language || "ja";
        phonemizer = phonemizer || "auto";
        if (phonemizer !== "auto")
            return phonemizer;
        const voice = window.voicebankById(voicebankId || "");
        if (voice && String(voice.suggested_language || "") === language
                && voice.suggested_phonemizer)
            return String(voice.suggested_phonemizer);
        return window.defaultPhonemizer(language);
    }

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
             "intonation", "applyPitch", "speechTiming"].indexOf(name) >= 0)
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
            if (voice && String(voice.kind || "") === "diffsinger") {
                utterances.setProperty(selectedIndex, "renderer", "diffsinger");
                selectCombo(editorContent.rendererCombo, "diffsinger");
            } else if (item.renderer === "diffsinger") {
                const renderer = window.defaultRendererId();
                utterances.setProperty(selectedIndex, "renderer", renderer);
                selectCombo(editorContent.rendererCombo, renderer);
            }
        }
        if (["voicebankId", "aliasPolicy", "modelId", "renderer", "tone", "color", "moraDuration",
             "pauseDuration", "intonation", "applyPitch", "speechTiming"].indexOf(name) >= 0) {
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
        analyzeTimer.restart();
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

    function updateUnitOverride(unitIndex, key, value) {
        if (!utterances.count || unitIndex < 0 || !String(key || "").length)
            return;
        const item = current();
        const overrides = decodeSequence(item.phonemeOverridesJson).map(value => {
            const copy = {};
            for (const name in value)
                copy[name] = value[name];
            return copy;
        });
        let override = null;
        let overrideIndex = -1;
        for (let index = 0; index < overrides.length; ++index) {
            if (Number(overrides[index].unit_index) === Number(unitIndex)) {
                override = overrides[index];
                overrideIndex = index;
                break;
            }
        }
        if (!override) {
            override = {unit_index: Number(unitIndex)};
            overrides.push(override);
            overrideIndex = overrides.length - 1;
        }
        const normalizedKey = String(key);
        if (value === undefined || value === null || (typeof value === "number" && !Number.isFinite(value)))
            return;
        override[normalizedKey] = value;
        if (Object.keys(override).length <= 1)
            overrides.splice(overrideIndex, 1);
        const encoded = JSON.stringify(overrides);
        if (item.phonemeOverridesJson === encoded)
            return;
        window.beginHistoryChange("unit:" + item.utteranceId + ":" + unitIndex + ":" + normalizedKey, true);
        utterances.setProperty(selectedIndex, "phonemeOverridesJson", encoded);
        markUtteranceDirty(selectedIndex);
        window.scheduleAutoPreview();
    }

    function clearUnitOverride(unitIndex) {
        if (!utterances.count || unitIndex < 0)
            return;
        const item = current();
        const overrides = decodeSequence(item.phonemeOverridesJson)
                .filter(value => Number(value.unit_index) !== Number(unitIndex));
        const encoded = JSON.stringify(overrides);
        if (item.phonemeOverridesJson === encoded)
            return;
        window.beginHistoryChange("unit:" + item.utteranceId + ":" + unitIndex + ":clear", true);
        utterances.setProperty(selectedIndex, "phonemeOverridesJson", encoded);
        markUtteranceDirty(selectedIndex);
        window.scheduleAutoPreview();
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
            if (!item.modelId) {
                utterances.setProperty(index, "modelId",
                                       window.defaultModelIdForLanguage(item.language || "ja"));
                changed = true;
            }
            if (!item.renderer) {
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
        const result = [];
        if (!sequence)
            return result;
        const size = sequence.length !== undefined ? sequence.length : sequence.count;
        for (let index = 0; index < size; ++index)
            result.push(sequence.get ? sequence.get(index) : sequence[index]);
        return result;
    }

    function decodeSequence(json) {
        if (!json || !json.length)
            return [];
        try {
            const value = JSON.parse(json);
            return Array.isArray(value) ? value : [];
        } catch (error) {
            return [];
        }
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
        if (window.batchExportActive || index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (!item.content.trim() || !item.reading || window.automaticProsodyReady(item))
            return;
        if (window.pendingProsodyUtteranceId === item.utteranceId
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
            if (window.pendingProsodyUtteranceId === utteranceId
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
        const normalized = window.copySequence(positions);
        if (!normalized.length)
            return normalized;
        if (normalized[0] === null || normalized[0] === undefined)
            return normalized;
        const first = Number(normalized[0]);
        if (!Number.isFinite(first))
            return normalized;
        const origin = Math.max(0, first);
        for (let index = 0; index < normalized.length; ++index) {
            if (normalized[index] === null || normalized[index] === undefined)
                continue;
            const value = Number(normalized[index]);
            if (Number.isFinite(value))
                normalized[index] = Math.max(0, value - origin);
        }
        normalized[0] = 0;
        return normalized;
    }

    function moraStartsFromCenters(centers, durations) {
        const starts = [];
        const size = centers ? centers.length : 0;
        for (let index = 0; index < size; ++index) {
            const center = Number(centers[index]);
            const duration = index < (durations ? durations.length : 0) ? Number(durations[index]) : 0;
            const start = Number.isFinite(center) && Number.isFinite(duration) && duration > 0
                    ? center - duration / 2 : null;
            starts.push(start !== null && start >= 0 ? start : null);
        }
        return window.normalizedMoraPositions(starts);
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
            modelId: window.defaultModelIdForLanguage(language),
            renderer: voice && String(voice.kind || "") === "diffsinger"
                    ? "diffsinger" : (window.appBackend.renderers.length ? window.defaultRendererId() : ""),
            aliasPolicy: window.appBackend.defaultAliasPolicy,
            tone: window.appBackend.defaultTone,
            color: "",
            moraDuration: window.appBackend.defaultMoraDuration,
            speechTiming: false,
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
        window.showAuxiliaryWindow(synthesisLogWindow);
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
            synthesisLogWindow.close();
    }

    function synthesizeCurrent() {
        const item = current();
        if (!item || !item.reading)
            return;
        clearPlayback();
        window.autoplayPreview = true;
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.clearLogs();
        window.showAuxiliaryWindow(synthesisLogWindow);
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
    }

    function scheduleAutoPreview() {
        if (!window.appBackend.autoPreviewEnabled || window.batchExportActive
                || window.saveRequestPending || window.playbackQueueActive || !utterances.count)
            return;
        const item = current();
        if (!item || !item.reading)
            return;
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
                || window.saveRequestPending || window.playbackQueueActive || !utterances.count)
            return;
        if (window.appBackend.busy) {
            autoPreviewTimer.restart();
            return;
        }
        const item = current();
        if (!item || !item.reading)
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
            speech_timing: !!item.speechTiming,
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
            speech_timing: !!item.speechTiming,
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
        if (window.batchExportActive || index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (!item.content.trim() || !item.reading)
            return;
        const requestId = item.utteranceId + ":" + item.revision + ":" + Date.now();
        window.pendingProsodyRequestId = requestId;
        window.pendingProsodyUtteranceId = item.utteranceId;
        window.pendingProsodyRevision = item.revision;
        window.appBackend.predictProsody(window.buildProsodyRequest(item, requestId));
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
        window.clearPlayback();
        window.pendingUtteranceId = "";
        window.pendingRevision = -1;
        window.pendingProsodyRequestId = "";
        window.pendingProsodyUtteranceId = "";
        window.pendingProsodyRevision = -1;
        window.appBackend.clearLogs();
        window.showAuxiliaryWindow(synthesisLogWindow);
        Qt.callLater(function() { window.synthesizeBatchItem(); });
    }

    function startBatchExport(directory) {
        window.beginBatchExport(directory, "save", window.buildExportQueue(false));
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
        if (window.appBackend.busy) {
            Qt.callLater(function() { window.synthesizeBatchItem(); });
            return;
        }
        while (window.batchExportIndex < window.batchExportQueue.length) {
            const index = window.batchExportQueue[window.batchExportIndex++];
            const item = utterances.get(index);
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
        window.batchExportMode = "";
        window.batchExportQueue = [];
        window.pendingUtteranceId = "";
        window.pendingRevision = -1;
        if (utterances.count)
            window.selectUtterance(Math.min(window.batchExportOriginalIndex, utterances.count - 1));
        if (success && (window.appBackend.closeLogOnSuccess || wasDragExport))
            synthesisLogWindow.close();
        if (dragExportSucceeded && files.length) {
            window.dragExportFiles = window.dragFilesWithExo(files);
            window.dragExportReady = true;
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
