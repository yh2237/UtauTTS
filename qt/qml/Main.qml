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
    width: 1240
    height: 850
    minimumWidth: 880
    minimumHeight: 600
    visible: !injectedSelfTest
    title: injectedAppName
    color: palette.window
    palette: Palette {
        window: window.darkMode ? "#202124" : "#f6f6f6"
        windowText: window.darkMode ? "#e8eaed" : "#202124"
        base: window.darkMode ? "#292a2d" : "#ffffff"
        alternateBase: window.darkMode ? "#303134" : "#f0f1f2"
        text: window.darkMode ? "#e8eaed" : "#202124"
        button: window.darkMode ? "#303134" : "#f0f1f2"
        buttonText: window.darkMode ? "#e8eaed" : "#202124"
        highlight: window.darkMode ? "#e8837d" : "#d35f6b"
        highlightedText: window.darkMode ? "#202124" : "#ffffff"
        placeholderText: window.darkMode ? "#aeb4ba" : "#697078"
        mid: window.darkMode ? "#5f6368" : "#aeb4ba"
    }

    property color accent: palette.highlight
    property color borderColor: palette.mid
    property color mutedText: palette.placeholderText
    readonly property url repositoryUrl: injectedRepositoryUrl
    readonly property var appBackend: injectedBackend
    readonly property bool darkMode: appBackend.darkMode
    readonly property var licenseDocuments: injectedLegalDocuments
    readonly property real defaultIntonationStrength: appBackend.defaultIntonationStrength
    readonly property real maxIntonationStrength: 4.0

    property var translator: translatorInstance
    property string updateAvailableVersion: ""
    property string updateReleaseNotes: ""
    property url updateReleaseUrl: ""
    property url updateDownloadUrl: ""
    property double updateDownloadReceived: 0
    property double updateDownloadTotal: 0
    property bool updateSuppressVersion: false
    property bool voicebankReloadActive: false

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

            Label {
                Layout.fillWidth: true
                text: window.translator.tr("main.exoFrameRateMessage")
                wrapMode: Text.WordWrap
            }

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
        hostWindow: prosodyTrainingWindow.visible ? prosodyTrainingWindow : window
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

    UsageWindow {
        id: usageWindow
        hostWindow: window
        hostPalette: window.palette
        translator: window.translator
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

    ProsodyTrainingWindow {
        id: prosodyTrainingWindow
        hostWindow: window
        hostPalette: window.palette
        backend: window.appBackend
        translator: window.translator
        synthesisLogWindow: synthesisLogWindow
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
        id: voicebankReloadDialog
        title: window.translator.tr("menu.file.reloadVoicebanks")
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

            Label {
                Layout.fillWidth: true
                text: window.translator.tr("voicebank.loadingDetail")
                color: window.mutedText
                font.pixelSize: 11
                wrapMode: Text.WordWrap
                horizontalAlignment: Text.AlignHCenter
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

        function onMetadataChanged() {
            if (window.voicebankReloadActive) {
                window.voicebankReloadActive = false;
                voicebankReloadDialog.close();
            }
            const suppressDirty = !window.metadataInitialized;
            window.assignDefaultVoicebank(suppressDirty);
            window.assignDefaultSynthesisSettings(suppressDirty);
            window.metadataInitialized = true;
            if (suppressDirty)
                window.resetHistory(false);
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
                return;
            }
            window.audioUtteranceId = window.pendingUtteranceId;
            window.audioRevision = window.pendingRevision;
            window.pendingUtteranceId = "";
            window.pendingRevision = -1;
            window.playbackError = "";
            window.playbackRequested = true;
            if (window.appBackend.closeLogOnSuccess)
                synthesisLogWindow.close();
            player.stop();
            player.source = audio;
            player.play();
        }

        function onErrorChanged() {
            if (window.voicebankReloadActive && window.appBackend.error.length) {
                window.voicebankReloadActive = false;
                voicebankReloadDialog.close();
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
    }

    Component.onCompleted: {
        window.translator.load(window.appBackend.resolvedLanguage());
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
                    onObjectAdded: recentProjectsMenu.insertItem(index, object)
                    onObjectRemoved: recentProjectsMenu.removeItem(object)
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
            GrayscaleMenuItem {
                visible: window.appBackend.developerMode
                height: visible ? implicitHeight : 0
                text: window.translator.tr("menu.settings.trainingData")
                enabled: !window.appBackend.busy
                onTriggered: prosodyTrainingWindow.openWindow()
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
                text: window.translator.tr("menu.help.usage")
                onTriggered: window.showAuxiliaryWindow(usageWindow)
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

    EditorContent {
        id: editorContent
        window: window
        anchors.fill: parent
    }

    function current() {
        return utterances.get(selectedIndex);
    }

    function diagnosticContext() {
        if (!utterances.count)
            return {};
        const item = window.current();
        return {
            voicebank_id: item.voicebankId || "",
            model_id: item.modelId || "",
            renderer: item.renderer || "",
            resampler: item.resampler || "",
            wavtool: item.wavtool || "builtin",
            alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone || "C4",
            color: item.color || "",
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            leading_preutterance_ms: item.leadingPreutterance,
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch,
            resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson)
        };
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
        if (utterances.count) {
            window.updateSetting("aliasPolicy", settingsWindow.pendingDefaultAliasPolicy);
            window.updateSetting("tone", settingsWindow.pendingDefaultTone);
            window.updateSetting("moraDuration", settingsWindow.pendingMoraDuration);
            window.updateSetting("pauseDuration", settingsWindow.pendingPauseDuration);
            window.updateSetting("leadingPreutterance", settingsWindow.pendingLeadingPreutterance);
            window.updateSetting("intonation", settingsWindow.pendingDefaultIntonationStrength);
            window.updateSetting("modelId", settingsWindow.pendingDefaultModelId);
            window.updateSetting("renderer", settingsWindow.pendingDefaultRendererId);
            window.selectUtterance(window.selectedIndex);
        }
        window.appBackend.setSynthesisDefaults(settingsWindow.pendingMoraDuration,
                                               settingsWindow.pendingPauseDuration,
                                               settingsWindow.pendingLeadingPreutterance,
                                               settingsWindow.pendingDefaultIntonationStrength,
                                               settingsWindow.pendingDefaultModelId,
                                               settingsWindow.pendingDefaultRendererId,
                                               settingsWindow.pendingDefaultTone,
                                               settingsWindow.pendingDefaultAliasPolicy);
        window.appBackend.setDarkMode(settingsWindow.pendingDarkMode);
        window.appBackend.setLanguage(settingsWindow.pendingLanguage);
        window.appBackend.setCloseLogOnSuccess(settingsWindow.pendingCloseLogOnSuccess);
        window.appBackend.setUpdateCheckEnabled(settingsWindow.pendingUpdateCheckEnabled);
        window.appBackend.setPreviewCacheFileCount(settingsWindow.pendingPreviewCacheFileCount);
        window.appBackend.setDeveloperMode(settingsWindow.pendingDeveloperMode);
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

    function checkForUpdates() {
        const request = new XMLHttpRequest();
        request.timeout = 10000;
        request.open("GET", "https://api.github.com/repos/yh2237/UtauTTS/releases/latest");
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
            if (!data || !data.tag_name)
                return;
            const latest = String(data.tag_name);
            if (window.compareVersions(latest, Qt.application.version) > 0) {
                const suppressed = window.appBackend.suppressedUpdateVersion();
                if (suppressed && window.compareVersions(latest, suppressed) <= 0)
                    return;
                window.updateSuppressVersion = false;
                window.updateAvailableVersion = latest;
                window.updateReleaseNotes = data.body ? String(data.body) : "";
                window.updateReleaseUrl = data.html_url ? String(data.html_url) : "";
                let downloadUrl = "";
                const assets = Array.isArray(data.assets) ? data.assets : [];
                const packageName = Qt.platform.os === "linux"
                    ? "UtauTTS-linux-x64.zip" : "UtauTTS-win-x64.zip";
                for (const asset of assets) {
                    if (asset && asset.name === packageName && asset.browser_download_url) {
                        downloadUrl = String(asset.browser_download_url);
                        break;
                    }
                }
                window.updateDownloadUrl = downloadUrl;
                updateDialog.open();
            }
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
        window.voicebankReloadActive = true;
        voicebankReloadDialog.open();
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

    function modelDescription(id) {
        if (!id || id === "none")
            return window.translator.tr("main.modelDescriptionNone");
        const model = modelById(id);
        return model ? model.description || "" : "";
    }

    function rendererDescription(id) {
        if (!id)
            return window.translator.tr("main.rendererDescriptionDefault");
        const renderer = rendererById(id);
        return renderer ? renderer.description || "" : "";
    }

    function defaultModelId() {
        const configured = String(window.appBackend.defaultModelId || "none");
        return configured === "none" || window.modelById(configured)
                ? configured : (window.appBackend.models.length ? window.appBackend.models[0].id : "none");
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
        return ["auto", "legacy", "cvvc-enhanced", "vcv-prefer", "cvvc-prefer", "cv-only"].indexOf(policy) >= 0 ? policy : "auto";
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
            moraDurationsJson: item.moraDurationsJson,
            moraPositionsJson: item.moraPositionsJson,
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
                pauseDuration: item.pauseDuration,
                leadingPreutterance: item.leadingPreutterance,
                intonation: item.intonation,
                applyPitch: item.applyPitch,
                pointsJson: item.manualPitchEdited ? item.pointsJson : "[]",
                moraDurationsJson: item.manualMoraDurationEdited ? item.moraDurationsJson : "[]",
                moraPositionsJson: item.manualMoraDurationEdited ? item.moraPositionsJson : "[]",
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
                    && item.moraDurationsJson === saved.moraDurationsJson
                    && item.moraPositionsJson === saved.moraPositionsJson
                    && item.manualPitchEdited === !!saved.manualPitchEdited
                    && item.manualMoraDurationEdited === !!saved.manualMoraDurationEdited)
                continue;
            utterances.setProperty(index, "pointsJson", String(saved.pointsJson || "[]"));
            utterances.setProperty(index, "moraDurationsJson", String(saved.moraDurationsJson || "[]"));
            utterances.setProperty(index, "moraPositionsJson", String(saved.moraPositionsJson || "[]"));
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

        let error = check(utterances.count === 1, "initial utterance is missing");
        if (error.length)
            return error;
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

        window.updateMoraDurations([110, 130]);
        window.updateMoraPositions([0, 110]);
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
        const project = window.projectData();
        error = check(project.format === "utautts-project" && project.format_version === 6
                      && project.utterances.length === 1, "project data generation failed");

        analyzeTimer.stop();
        utterances.clear();
        window.nextUtteranceId = 1;
        window.addUtterance(false);
        window.resetHistory(false);
        return error;
    }

    function projectData() {
        const savedUtterances = [];
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            savedUtterances.push({
                text: item.content || "",
                language: item.language || "ja",
                phonemizer: item.phonemizer || window.defaultPhonemizer(item.language || "ja"),
                voicebank_id: item.voicebankId || "",
                model_id: item.modelId || "",
                renderer_id: item.renderer || "",
                resampler: item.resampler || "",
                wavtool: item.wavtool || "builtin",
                alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
                tone: item.tone || "C4",
                color: item.color || "",
                mora_duration_ms: item.moraDuration,
                pause_duration_ms: item.pauseDuration,
                leading_preutterance_ms: item.leadingPreutterance,
                intonation: item.intonation,
                apply_pitch: !!item.applyPitch,
                pitch_points: window.decodeSequence(item.pointsJson),
                mora_durations_ms: window.decodeSequence(item.moraDurationsJson),
                mora_positions_ms: window.decodeSequence(item.moraPositionsJson),
                automatic_pitch_points: window.automaticSequence(item, "autoPointsJson"),
                automatic_mora_durations_ms: window.automaticSequence(item, "autoMoraDurationsJson"),
                automatic_mora_positions_ms: window.automaticSequence(item, "autoMoraPositionsJson"),
                manual_pitch_edited: window.hasManualPitch(item),
                manual_mora_duration_edited: window.hasManualMoraDurations(item),
                resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson),
                analysis_cache: {
                    reading: item.reading || "",
                    morae: window.decodeSequence(item.moraeJson)
                }
            });
        }
        return {
            format: "utautts-project",
            format_version: 6,
            app_version: Qt.application.version,
            utterances: savedUtterances,
            selected_index: utterances.count ? selectedIndex : 0
        };
    }

    function reanalyzeAll() {
        window.clearPlayback();
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            utterances.setProperty(index, "reading", "");
            utterances.setProperty(index, "moraeJson", "[]");
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
            const content = String(saved.text || "");
            let rendererId = window.normalizeRendererId(saved.renderer_id);
            let resamplerId = String(saved.resampler || "");
            let wavtoolId = String(saved.wavtool || "builtin");
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
                moraDurationsJson: JSON.stringify(manualDurations),
                moraPositionsJson: JSON.stringify(manualPositions),
                autoPointsJson: JSON.stringify(window.copySequence(saved.automatic_pitch_points)),
                autoMoraDurationsJson: JSON.stringify(automaticDurations),
                autoMoraPositionsJson: JSON.stringify(automaticPositions),
                resamplerExpressionsJson: JSON.stringify(window.copySequence(saved.resampler_expressions)),
                manualPitchEdited: saved.manual_pitch_edited === undefined
                        ? points.some(value => Math.abs(Number(value)) > .1) : !!saved.manual_pitch_edited,
                manualMoraDurationEdited: saved.manual_mora_duration_edited === undefined
                        ? window.copySequence(saved.mora_durations_ms).some(value => Number(value) > 0)
                        : !!saved.manual_mora_duration_edited,
                voicebankId: voicebankId,
                imagePath: voice ? voice.image_path || "" : "",
                modelId: String(saved.model_id || ""),
                renderer: rendererId,
                resampler: resamplerId,
                wavtool: wavtoolId,
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
        window.projectFile = source;
        window.appBackend.rememberRecentProject(source);

        if (!utterances.count) {
            selectedIndex = 0;
            editorContent.pitchEditor.points = [];
            editorContent.pitchEditor.morae = [];
            editorContent.pitchEditor.moraDurations = [];
            editorContent.pitchEditor.moraPositions = [];
            window.resetHistory(migratedRenderer);
            return;
        }
        selectedIndex = Math.max(0, Math.min(Number(project.selected_index) || 0, utterances.count - 1));
        window.selectUtterance(selectedIndex);
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            if (item.content.trim())
                window.analyzeUtterance(index);
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
            "ja-kana": window.translator.tr("main.phonemizer.jaKana"),
            "en-arpasing": window.translator.tr("main.phonemizer.enArpasing"),
            "en-delta": window.translator.tr("main.phonemizer.enDelta"),
            "en-vccv": window.translator.tr("main.phonemizer.enVccv"),
            "zh-cvvc": window.translator.tr("main.phonemizer.zhCvvc")
        };
        if (language === "en")
            return ["en-arpasing", "en-delta", "en-vccv"].map(
                        id => ({id: id, display_name: labels[id]}));
        const id = window.defaultPhonemizer(language);
        return [{id: id, display_name: labels[id]}];
    }

    function analyzeUtterance(index) {
        if (index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        if (!item.content.trim())
            return;
        window.appBackend.analyzeSpeech(item.content, item.utteranceId,
                                        item.language || "ja",
                                        item.phonemizer || window.defaultPhonemizer(item.language || "ja"),
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
        if (language !== "ja")
            utterances.setProperty(selectedIndex, "modelId", "none");
        utterances.setProperty(selectedIndex, "reading", "");
        utterances.setProperty(selectedIndex, "moraeJson", "[]");
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
            window.analyzeUtterance(selectedIndex);
        }
        if (name === "voicebankId" && (item.language || "ja") === "en") {
            utterances.setProperty(selectedIndex, "reading", "");
            utterances.setProperty(selectedIndex, "moraeJson", "[]");
            window.analyzeUtterance(selectedIndex);
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
        utterances.setProperty(index, "content", text);
        utterances.setProperty(index, "reading", "");
        utterances.setProperty(index, "moraeJson", "[]");
        utterances.setProperty(index, "pointsJson", "[]");
        utterances.setProperty(index, "moraDurationsJson", "[]");
        utterances.setProperty(index, "moraPositionsJson", "[]");
        utterances.setProperty(index, "autoPointsJson", "[]");
        utterances.setProperty(index, "autoMoraDurationsJson", "[]");
        utterances.setProperty(index, "autoMoraPositionsJson", "[]");
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
    }

    function updateMoraDurations(durations) {
        if (!utterances.count)
            return;
        const durationsJson = JSON.stringify(durations);
        if (current().moraDurationsJson === durationsJson && current().manualMoraDurationEdited)
            return;
        window.beginHistoryChange("timing:" + current().utteranceId, true);
        utterances.setProperty(selectedIndex, "moraDurationsJson", durationsJson);
        utterances.setProperty(selectedIndex, "manualMoraDurationEdited", true);
        markUtteranceDirty(selectedIndex);
    }

    function updateMoraPositions(positions) {
        if (!utterances.count)
            return;
        const positionsJson = JSON.stringify(positions);
        if (current().moraPositionsJson === positionsJson && current().manualMoraDurationEdited)
            return;
        window.beginHistoryChange("timing:" + current().utteranceId, true);
        utterances.setProperty(selectedIndex, "moraPositionsJson", positionsJson);
        utterances.setProperty(selectedIndex, "manualMoraDurationEdited", true);
        markUtteranceDirty(selectedIndex);
    }

    function markUtteranceDirty(index, markProject) {
        if (index < 0 || index >= utterances.count)
            return;
        const item = utterances.get(index);
        utterances.setProperty(index, "revision", item.revision + 1);
        if (markProject !== false)
            window.projectDirty = true;
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
            if (!item.voicebankId || !window.voicebankById(item.voicebankId)) {
                utterances.setProperty(i, "voicebankId", voice.id);
                utterances.setProperty(i, "imagePath", voice.image_path || "");
                markUtteranceDirty(i, suppressDirty !== true);
            }
        }
        selectUtterance(selectedIndex);
    }

    function assignDefaultSynthesisSettings(suppressDirty) {
        if (!utterances.count || !window.appBackend.models.length || !window.appBackend.renderers.length)
            return;
        const modelId = window.defaultModelId();
        const rendererId = window.defaultRendererId();
        for (let index = 0; index < utterances.count; ++index) {
            const item = utterances.get(index);
            let changed = false;
            if (!item.modelId) {
                utterances.setProperty(index, "modelId", modelId);
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
                return;
            }
        }
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
        }
        selectedIndex = index;
        const item = current();
        editorContent.toneField.text = item.tone;
        editorContent.moraSlider.value = item.moraDuration;
        editorContent.pauseSlider.value = item.pauseDuration;
        editorContent.leadingPreutteranceSlider.value = item.leadingPreutterance;
        editorContent.moraInput.value = item.moraDuration;
        editorContent.pauseInput.value = item.pauseDuration;
        editorContent.leadingPreutteranceInput.value = item.leadingPreutterance;
        editorContent.intonationSlider.value = item.intonation;
        editorContent.intonationInput.value = Math.round(item.intonation * 100);
        editorContent.pitchEditor.points = window.decodeSequence(item.pointsJson);
        editorContent.pitchEditor.autoPoints = window.automaticSequence(item, "autoPointsJson");
        editorContent.pitchEditor.morae = window.decodeSequence(item.moraeJson);
        editorContent.pitchEditor.defaultMoraDuration = item.moraDuration;
        editorContent.pitchEditor.defaultPauseDuration = item.pauseDuration;
        editorContent.pitchEditor.moraDurations = window.displayedMoraDurations(item);
        editorContent.pitchEditor.moraPositions = window.displayedMoraPositions(item);
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
        selectCombo(editorContent.resamplerCombo, item.resampler || "");
        selectCombo(editorContent.wavtoolCombo, item.wavtool || "builtin");
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
        return decodeSequence(item ? item.pointsJson : "").some(value => Math.abs(Number(value)) > .1);
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
            return decodeSequence(item.moraPositionsJson);
        return automaticSequence(item, "autoMoraPositionsJson");
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
        return starts;
    }

    function clearAutomaticArrays(index) {
        if (index < 0 || index >= utterances.count)
            return;
        utterances.setProperty(index, "autoPointsJson", "[]");
        utterances.setProperty(index, "autoMoraDurationsJson", "[]");
        utterances.setProperty(index, "autoMoraPositionsJson", "[]");
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
            editorContent.pitchEditor.moraPositions = hasManualMoraDurations(item)
                    ? decodeSequence(item.moraPositionsJson) : automaticStarts.slice();
        }
    }

    function clearAutomaticProsody(index) {
        window.clearAutomaticArrays(index);
        if (index === window.selectedIndex) {
            const item = utterances.get(index);
            editorContent.pitchEditor.autoPoints = [];
            editorContent.pitchEditor.moraDurations = hasManualMoraDurations(item)
                    ? decodeSequence(item.moraDurationsJson) : [];
            editorContent.pitchEditor.moraPositions = hasManualMoraDurations(item)
                    ? decodeSequence(item.moraPositionsJson) : [];
        }
    }

    function resetMoraDuration() {
        editorContent.moraSlider.value = 120;
        editorContent.moraInput.value = 120;
        window.updateSetting("moraDuration", 120);
    }

    function resetIntonation() {
        editorContent.intonationSlider.value = window.defaultIntonationStrength;
        editorContent.intonationInput.value = Math.round(window.defaultIntonationStrength * 100);
        window.updateSetting("intonation", window.defaultIntonationStrength);
    }

    function resetPauseDuration() {
        editorContent.pauseSlider.value = 180;
        editorContent.pauseInput.value = 180;
        window.updateSetting("pauseDuration", 180);
    }

    function resetLeadingPreutterance() {
        editorContent.leadingPreutteranceSlider.value = 0;
        editorContent.leadingPreutteranceInput.value = 0;
        window.updateSetting("leadingPreutterance", 0);
    }

    function addUtterance(markDirty) {
        const voice = window.defaultVoicebank();
        const language = voice && voice.suggested_language
                ? String(voice.suggested_language) : "ja";
        const phonemizer = voice && voice.suggested_phonemizer
                ? String(voice.suggested_phonemizer) : window.defaultPhonemizer(language);
        utterances.append({
            utteranceId: "utterance-" + nextUtteranceId++,
            content: "",
            language: language,
            phonemizer: phonemizer,
            reading: "",
            moraeJson: "[]",
            pointsJson: "[]",
            moraDurationsJson: "[]",
            moraPositionsJson: "[]",
            autoPointsJson: "[]",
            autoMoraDurationsJson: "[]",
            autoMoraPositionsJson: "[]",
            resamplerExpressionsJson: "[]",
            manualPitchEdited: false,
            manualMoraDurationEdited: false,
            voicebankId: voice ? voice.id : "",
            imagePath: voice ? voice.image_path || "" : "",
            modelId: window.appBackend.models.length ? window.defaultModelId() : "",
            renderer: window.appBackend.renderers.length ? window.defaultRendererId() : "",
            resampler: window.appBackend.resamplers.length ? window.appBackend.resamplers[0].id : "",
            wavtool: "builtin",
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
        window.pendingUtteranceId = item.utteranceId;
        window.pendingRevision = item.revision;
        window.appBackend.clearLogs();
        window.showAuxiliaryWindow(synthesisLogWindow);
        window.appBackend.synthesize(window.buildSynthesisRequest(item));
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
            phonemizer: item.phonemizer || window.defaultPhonemizer(item.language || "ja"),
            dictionary: window.appBackend.dictionaryEntries,
            voicebank_id: item.voicebankId || editorContent.voiceCombo.currentValue,
            model_id: item.modelId,
            renderer: item.renderer,
            resampler: item.resampler || "",
            wavtool: item.wavtool || "builtin",
            alias_policy: window.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone,
            color: item.color || "",
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            leading_preutterance_ms: item.leadingPreutterance,
            mora_durations_ms: manualDurations,
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch,
            resampler_expressions: window.decodeSequence(item.resamplerExpressionsJson)
        };
        if (item.applyPitch && item.reading && manualPitch && points.some(value => Math.abs(Number(value)) > .1)) {
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
        return request;
    }

    function buildProsodyRequest(item, requestId) {
        return {
            request_id: requestId,
            text: item.content,
            reading: item.reading || "",
            language: item.language || "ja",
            phonemizer: item.phonemizer || window.defaultPhonemizer(item.language || "ja"),
            dictionary: window.appBackend.dictionaryEntries,
            model_id: item.modelId,
            renderer: item.renderer,
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            mora_durations_ms: window.hasManualMoraDurations(item)
                    ? window.decodeSequence(item.moraDurationsJson) : [],
            intonation_strength: item.intonation,
            apply_pitch: item.applyPitch
        };
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
