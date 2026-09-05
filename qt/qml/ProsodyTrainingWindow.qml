pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import QtMultimedia

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
    required property var synthesisLogWindow

    title: root.translator.tr("training.title")
    visible: false
    width: 980
    height: 700
    minimumWidth: 980
    maximumWidth: 980
    minimumHeight: 700
    maximumHeight: 700
    transientParent: hostWindow
    modality: Qt.WindowModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    property var promptSet: ({})
    property var promptPack: ({})
    property var savedSession: ({})
    property bool sessionActive: false
    property bool sessionFinished: false
    property string sessionId: ""
    property int shuffleSeed: 0
    property var order: []
    property var records: []
    property int currentIndex: 0
    property var currentPrompt: ({})
    property string reading: ""
    property var morae: []
    property var features: []
    property var basePoints: []
    property var manualOffsets: []
    property var editMask: []
    property var moraDurations: []
    property var moraPositions: []
    property int playedCount: 0
    property bool promptReady: false
    property string requestId: ""
    property string sessionMessage: ""

    readonly property var baseModel: root.findById(root.backend.models, "frame-intonation-v8")
    readonly property int acceptedCount: root.countStatus("accepted")
    readonly property int skippedCount: root.countStatus("skipped")

    function findById(values, id) {
        for (let index = 0; index < values.length; ++index) {
            if (String(values[index].id) === String(id))
                return values[index];
        }
        return null;
    }

    function copy(values) {
        const result = [];
        if (!values)
            return result;
        for (let index = 0; index < values.length; ++index)
            result.push(values[index]);
        return result;
    }

    function promptById(id) {
        const prompts = root.promptPack.prompts || [];
        return root.findById(prompts, id);
    }

    function packById(id) {
        return root.findById(root.promptSet.packs || [], id);
    }

    function countStatus(status) {
        let count = 0;
        for (let index = 0; index < root.records.length; ++index) {
            if (root.records[index] && root.records[index].status === status)
                ++count;
        }
        return count;
    }

    function modelFingerprint() {
        return root.baseModel ? String(root.baseModel.sha256 || "") : "";
    }

    function contextObject() {
        return {
            renderer: String(rendererCombo.currentValue || ""),
            voicebank_id: String(voicebankCombo.currentValue || ""),
            tone: toneField.text.trim() || "C4",
            color: root.selectedColor(),
            alias_policy: "auto",
            intonation_strength: 1.0,
            mora_duration_ms: root.backend.defaultMoraDuration,
            pause_duration_ms: root.backend.defaultPauseDuration
        };
    }

    function selectedColor() {
        const option = root.hostWindow.voicebankTypeOptionAt(
                    voicebankCombo.currentValue, colorCombo.currentIndex, "");
        return option ? String(option.color || "") : "";
    }

    function clearPlayback() {
        player.stop();
        player.source = "";
    }

    function defaultRendererIndex() {
        const recommended = root.baseModel ? root.baseModel.recommended_renderers || [] : [];
        for (let wanted = 0; wanted < recommended.length; ++wanted) {
            for (let index = 0; index < root.backend.renderers.length; ++index) {
                if (root.backend.renderers[index].id === recommended[wanted])
                    return index;
            }
        }
        for (let index = 0; index < root.backend.renderers.length; ++index) {
            if (root.backend.renderers[index].id === root.backend.defaultRenderer)
                return index;
        }
        return 0;
    }

    function defaultVoicebankIndex() {
        const id = String(root.backend.defaultVoicebankId || "");
        for (let index = 0; index < root.backend.voicebanks.length; ++index) {
            if (root.backend.voicebanks[index].id === id)
                return index;
        }
        return 0;
    }

    function shuffledPromptIds(seed) {
        const prompts = root.promptPack.prompts || [];
        const values = [];
        for (let index = 0; index < prompts.length; ++index)
            values.push(String(prompts[index].id));
        let state = Number(seed) & 0x7fffffff;
        for (let index = values.length - 1; index > 0; --index) {
            state = (Math.imul(state, 1103515245) + 12345) & 0x7fffffff;
            const target = state % (index + 1);
            const value = values[index];
            values[index] = values[target];
            values[target] = value;
        }
        return values;
    }

    function openWindow() {
        root.promptSet = root.backend.loadProsodyPromptSet();
        root.savedSession = root.backend.loadProsodyTrainingSession();
        root.sessionMessage = root.promptSet._error ? String(root.promptSet._error) : "";
        root.sessionActive = false;
        root.sessionFinished = false;
        root.promptPack = {};
        promptPackCombo.currentIndex = 0;
        rendererCombo.currentIndex = root.defaultRendererIndex();
        voicebankCombo.currentIndex = root.defaultVoicebankIndex();
        root.show();
        root.raise();
        root.requestActivate();
    }

    function startNewSession() {
        root.promptPack = root.packById(promptPackCombo.currentValue) || {};
        if (!root.baseModel || !(root.promptPack.prompts || []).length
                || !root.backend.voicebanks.length || !root.backend.renderers.length) {
            root.sessionMessage = root.translator.tr("training.unavailable");
            return;
        }
        root.backend.clearProsodyTrainingSession();
        root.sessionId = "prosody-" + new Date().toISOString().replace(/[-:.TZ]/g, "");
        root.shuffleSeed = Date.now() & 0x7fffffff;
        root.order = root.shuffledPromptIds(root.shuffleSeed);
        root.records = [];
        for (let index = 0; index < root.order.length; ++index)
            root.records.push(null);
        root.currentIndex = 0;
        root.sessionActive = true;
        root.sessionFinished = false;
        root.sessionMessage = "";
        root.loadCurrentPrompt(false);
    }

    function requestNewSession() {
        if (root.savedSession && root.savedSession.session_id)
            replaceSessionDialog.open();
        else
            root.startNewSession();
    }

    function resumeSession() {
        const session = root.savedSession || {};
        if (session.format !== "utautts-prosody-training-session" || Number(session.format_version) !== 1) {
            root.sessionMessage = root.translator.tr("training.resumeInvalid");
            return;
        }
        const savedBase = session.base_model || {};
        const savedPromptSet = session.prompt_set || {};
        const savedPack = root.packById(savedPromptSet.pack_id);
        if (String(savedPromptSet.id || "") !== String(root.promptSet.id || "")
                || Number(savedPromptSet.version) !== Number(root.promptSet.version)
                || !savedPack) {
            root.sessionMessage = root.translator.tr("training.resumePromptMismatch");
            return;
        }
        if (!root.baseModel || String(savedBase.sha256 || "") !== root.modelFingerprint()
                || String((session.frontend || {}).dictionary_fingerprint || "") !== root.backend.dictionaryFingerprint()) {
            root.sessionMessage = root.translator.tr("training.resumeMismatch");
            return;
        }
        root.sessionId = String(session.session_id || "");
        root.promptPack = savedPack;
        for (let index = 0; index < promptPackCombo.count; ++index) {
            if (String(promptPackCombo.valueAt(index)) === String(savedPromptSet.pack_id))
                promptPackCombo.currentIndex = index;
        }
        root.shuffleSeed = Number(session.shuffle_seed) || 0;
        root.order = root.copy(session.order || []);
        root.records = root.copy(session.records || []);
        while (root.records.length < root.order.length)
            root.records.push(null);
        root.currentIndex = Math.max(0, Math.min(Number(session.current_index) || 0, root.order.length));
        root.applyContext(session.synthesis_context || {});
        root.sessionActive = true;
        root.sessionFinished = !!session.finished || root.currentIndex >= root.order.length;
        root.sessionMessage = "";
        if (!root.sessionFinished)
            root.loadCurrentPrompt(true);
    }

    function applyContext(context) {
        for (let index = 0; index < voicebankCombo.count; ++index) {
            if (voicebankCombo.valueAt(index) === context.voicebank_id)
                voicebankCombo.currentIndex = index;
        }
        for (let index = 0; index < rendererCombo.count; ++index) {
            if (rendererCombo.valueAt(index) === context.renderer)
                rendererCombo.currentIndex = index;
        }
        toneField.text = String(context.tone || "C4");
        root.hostWindow.selectCombo(colorCombo,
                root.hostWindow.typeIdForColor(context.voicebank_id, context.color || ""));
    }

    function sessionObject() {
        return {
            format: "utautts-prosody-training-session",
            format_version: 1,
            session_id: root.sessionId,
            shuffle_seed: root.shuffleSeed,
            prompt_set: {
                id: String(root.promptSet.id || ""),
                version: Number(root.promptSet.version) || 1,
                pack_id: String(root.promptPack.id || "")
            },
            base_model: {
                id: "frame-intonation-v8",
                sha256: root.modelFingerprint()
            },
            frontend: {
                feature_version: root.baseModel ? Number(root.baseModel.feature_version) || 1 : 1,
                dictionary_fingerprint: root.backend.dictionaryFingerprint()
            },
            synthesis_context: root.contextObject(),
            order: root.copy(root.order),
            records: root.copy(root.records),
            current_index: root.currentIndex,
            finished: root.sessionFinished,
            updated_at: new Date().toISOString()
        };
    }

    function persist() {
        if (root.sessionActive)
            root.backend.saveProsodyTrainingSession(root.sessionObject());
    }

    function makeRecord(accepted, status) {
        return {
            version: 1,
            id: root.sessionId + "-" + String(root.currentPrompt.id || ""),
            session_id: root.sessionId,
            prompt_set: {
                id: String(root.promptSet.id || ""),
                version: Number(root.promptSet.version) || 1,
                pack_id: String(root.promptPack.id || ""),
                prompt_id: String(root.currentPrompt.id || "")
            },
            text: String(root.currentPrompt.text || ""),
            reading: root.reading,
            morae: root.copy(root.morae),
            features: root.copy(root.features),
            base_model: {
                id: "frame-intonation-v8",
                sha256: root.modelFingerprint()
            },
            frontend: {
                feature_version: root.baseModel ? Number(root.baseModel.feature_version) || 1 : 1,
                dictionary_fingerprint: root.backend.dictionaryFingerprint()
            },
            synthesis_context: root.contextObject(),
            base_points_cents: root.copy(root.basePoints),
            mora_durations_ms: root.copy(root.moraDurations),
            mora_positions_ms: root.copy(root.moraPositions),
            manual_offsets_cents: root.copy(root.manualOffsets),
            edit_mask: root.copy(root.editMask),
            review_kind: root.editMask.some(value => !!value)
                         ? "edited-and-reviewed" : "unchanged-and-reviewed",
            accepted: accepted,
            status: status,
            source_kind: "gui-confirmed",
            played_count: root.playedCount,
            accepted_at: accepted ? new Date().toISOString() : ""
        };
    }

    function saveDraft() {
        if (!root.sessionActive || root.sessionFinished || !root.currentPrompt.id)
            return;
        const next = root.copy(root.records);
        next[root.currentIndex] = root.makeRecord(false, "draft");
        root.records = next;
        root.persist();
    }

    function loadCurrentPrompt(fromSaved) {
        root.clearPlayback();
        root.promptReady = false;
        root.reading = "";
        root.morae = [];
        root.features = [];
        root.basePoints = [];
        root.manualOffsets = [];
        root.editMask = [];
        root.moraDurations = [];
        root.moraPositions = [];
        root.playedCount = 0;
        root.currentPrompt = root.promptById(root.order[root.currentIndex]) || {};
        const saved = fromSaved ? root.records[root.currentIndex] : null;
        if (saved && saved.text === root.currentPrompt.text && (saved.base_points_cents || []).length) {
            root.reading = String(saved.reading || "");
            root.morae = root.copy(saved.morae || []);
            root.features = root.copy(saved.features || []);
            root.basePoints = root.copy(saved.base_points_cents || []);
            root.moraDurations = root.copy(saved.mora_durations_ms || []);
            root.moraPositions = root.copy(saved.mora_positions_ms || []);
            root.manualOffsets = root.copy(saved.manual_offsets_cents || []);
            root.editMask = root.copy(saved.edit_mask || []);
            root.playedCount = Number(saved.played_count) || 0;
            root.promptReady = true;
            pitchEditor.points = root.copy(root.manualOffsets);
            pitchEditor.autoPoints = root.copy(root.basePoints);
            pitchEditor.morae = root.copy(root.morae);
            pitchEditor.moraDurations = root.copy(root.moraDurations);
            const next = root.copy(root.records);
            next[root.currentIndex] = root.makeRecord(false, "draft");
            root.records = next;
            root.persist();
            return;
        }
        root.requestId = "training:" + root.sessionId + ":" + root.currentPrompt.id + ":" + Date.now();
        root.backend.analyze(String(root.currentPrompt.text || ""), root.requestId);
        root.persist();
    }

    function requestProsody() {
        root.backend.predictProsody({
            request_id: root.requestId,
            text: String(root.currentPrompt.text || ""),
            kana: root.reading,
            dictionary: root.backend.dictionaryEntries,
            model_id: "frame-intonation-v8",
            renderer: String(rendererCombo.currentValue || ""),
            mora_duration_ms: root.backend.defaultMoraDuration,
            pause_duration_ms: root.backend.defaultPauseDuration,
            mora_durations_ms: [],
            intonation_strength: 1.0,
            apply_pitch: true
        });
    }

    function synthesizeCurrent() {
        if (!root.promptReady || root.backend.busy)
            return;
        const request = {
            text: String(root.currentPrompt.text || ""),
            kana: root.reading,
            dictionary: root.backend.dictionaryEntries,
            voicebank_id: String(voicebankCombo.currentValue || ""),
            model_id: "frame-intonation-v8",
            renderer: String(rendererCombo.currentValue || ""),
            alias_policy: "auto",
            tone: toneField.text.trim() || "C4",
            color: root.selectedColor(),
            mora_duration_ms: root.backend.defaultMoraDuration,
            pause_duration_ms: root.backend.defaultPauseDuration,
            mora_durations_ms: [],
            intonation_strength: 1.0,
            apply_pitch: true
        };
        if (root.manualOffsets.some(value => Math.abs(Number(value)) > .1)) {
            const points = [];
            for (let index = 0; index < root.manualOffsets.length; ++index) {
                if (root.morae[index] && root.morae[index].pause)
                    continue;
                points.push({
                    position: index,
                    mora: root.morae[index] ? String(root.morae[index].mora || "") : "",
                    cents: Number(root.manualOffsets[index]) || 0
                });
            }
            request.manual_pitch = {version: 1, reading: root.reading, mode: "offset", points: points};
        }
        root.backend.clearLogs();
        root.hostWindow.showAuxiliaryWindow(root.synthesisLogWindow);
        root.backend.synthesize(request);
    }

    function acceptCurrent() {
        if (root.playedCount < 1 || !root.promptReady)
            return;
        if (!root.editMask.some(value => !!value)) {
            unchangedDialog.open();
            return;
        }
        root.finishAccept();
    }

    function finishAccept() {
        const next = root.copy(root.records);
        next[root.currentIndex] = root.makeRecord(true, "accepted");
        root.records = next;
        root.advance();
    }

    function skipCurrent() {
        const next = root.copy(root.records);
        next[root.currentIndex] = {
            version: 1,
            id: root.sessionId + "-" + String(root.currentPrompt.id || ""),
            session_id: root.sessionId,
            prompt_set: {
                id: root.promptSet.id,
                version: root.promptSet.version,
                pack_id: root.promptPack.id,
                prompt_id: root.currentPrompt.id
            },
            text: root.currentPrompt.text,
            accepted: false,
            status: "skipped",
            source_kind: "gui-confirmed"
        };
        root.records = next;
        root.advance();
    }

    function advance() {
        ++root.currentIndex;
        if (root.currentIndex >= root.order.length) {
            root.sessionFinished = true;
            root.persist();
            return;
        }
        root.persist();
        root.loadCurrentPrompt(true);
    }

    function goBack() {
        if (root.currentIndex <= 0 || root.backend.busy)
            return;
        root.saveDraft();
        --root.currentIndex;
        root.sessionFinished = false;
        root.loadCurrentPrompt(true);
    }

    Connections {
        target: root.backend

        function onAnalysisChanged() {
            if (!root.sessionActive || root.backend.analysisRequestId !== root.requestId)
                return;
            let analysis;
            try {
                analysis = JSON.parse(root.backend.analysisJson);
            } catch (error) {
                root.sessionMessage = String(error);
                return;
            }
            root.reading = String(analysis.reading || "");
            root.morae = root.copy(analysis.morae || []);
            root.requestProsody();
        }

        function onProsodyChanged() {
            if (!root.sessionActive || root.backend.prosodyRequestId !== root.requestId)
                return;
            let result;
            try {
                result = JSON.parse(root.backend.prosodyJson);
            } catch (error) {
                root.sessionMessage = String(error);
                return;
            }
            root.features = root.copy(result.features || []);
            root.basePoints = root.copy(result.pitch_points || []);
            root.moraDurations = root.copy(result.mora_durations_ms || []);
            root.moraPositions = root.copy(result.mora_positions_ms || []);
            root.manualOffsets = [];
            root.editMask = [];
            for (let index = 0; index < root.basePoints.length; ++index) {
                root.manualOffsets.push(0);
                root.editMask.push(false);
            }
            pitchEditor.points = root.copy(root.manualOffsets);
            pitchEditor.autoPoints = root.copy(root.basePoints);
            pitchEditor.morae = root.copy(root.morae);
            pitchEditor.moraDurations = root.copy(root.moraDurations);
            pitchEditor.moraPositions = [];
            root.promptReady = root.basePoints.length === root.morae.length;
            root.saveDraft();
        }

        function onPreviewReady() {
            if (!root.sessionActive || !root.visible)
                return;
            player.stop();
            player.source = root.backend.previewUrl;
            player.play();
            if (root.backend.closeLogOnSuccess)
                root.synthesisLogWindow.close();
            ++root.playedCount;
            root.saveDraft();
        }
    }

    AudioOutput {
        id: audioOutput
    }

    MediaPlayer {
        id: player
        audioOutput: audioOutput
    }

    FileDialog {
        id: exportDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [root.translator.tr("training.datasetFilter")]
        defaultSuffix: "jsonl"
        onAccepted: {
            if (root.backend.exportProsodyTrainingDataset(selectedFile, root.sessionObject()))
                root.sessionMessage = root.translator.tr("training.exported");
        }
    }

    Dialog {
        id: unchangedDialog
        width: Math.min(480, root.width - 32)
        title: root.translator.tr("training.unchangedTitle")
        modal: true
        anchors.centerIn: Overlay.overlay
        standardButtons: Dialog.Yes | Dialog.No
        onAccepted: root.finishAccept()

        contentItem: Label {
            text: root.translator.tr("training.unchangedMessage")
            wrapMode: Text.WordWrap
        }
    }

    Dialog {
        id: replaceSessionDialog
        width: Math.min(480, root.width - 32)
        title: root.translator.tr("training.replaceTitle")
        modal: true
        anchors.centerIn: Overlay.overlay
        standardButtons: Dialog.Yes | Dialog.No
        onAccepted: root.startNewSession()

        contentItem: Label {
            text: root.translator.tr("training.replaceMessage")
            wrapMode: Text.WordWrap
        }
    }

    StackLayout {
        anchors.fill: parent
        anchors.margins: 16
        currentIndex: !root.sessionActive ? 0 : root.sessionFinished ? 2 : 1

        ColumnLayout {
            spacing: 14

            Label {
                Layout.fillWidth: true
                text: root.translator.tr("training.description")
                wrapMode: Text.WordWrap
            }

            GridLayout {
                Layout.fillWidth: true
                columns: 2
                columnSpacing: 12
                rowSpacing: 10

                Label { text: root.translator.tr("training.promptPack") }
                ComboBox {
                    id: promptPackCombo
                    Layout.fillWidth: true
                    model: root.promptSet.packs || []
                    textRole: "name"
                    valueRole: "id"
                }

                Label { text: root.translator.tr("training.voicebank") }
                ComboBox {
                    id: voicebankCombo
                    Layout.fillWidth: true
                    model: root.backend.voicebanks
                    textRole: "name"
                    valueRole: "id"
                    onActivated: colorCombo.currentIndex = 0
                }
                Label { text: root.translator.tr("training.renderer") }
                ComboBox {
                    id: rendererCombo
                    Layout.fillWidth: true
                    model: root.backend.renderers
                    textRole: "display_name"
                    valueRole: "id"
                }
                Label { text: root.translator.tr("training.tone") }
                TextField {
                    id: toneField
                    Layout.fillWidth: true
                    text: "C4"
                }
                Label { text: root.translator.tr("training.color") }
                ComboBox {
                    id: colorCombo
                    Layout.fillWidth: true
                    model: root.hostWindow.voicebankTypeOptions(voicebankCombo.currentValue)
                    textRole: "display_name"
                    valueRole: "id"
                }
            }

            Label {
                Layout.fillWidth: true
                visible: root.sessionMessage.length > 0
                text: root.sessionMessage
                color: root.palette.text
                wrapMode: Text.WordWrap
            }

            Item { Layout.fillHeight: true }

            RowLayout {
                Layout.fillWidth: true
                Button {
                    visible: !!(root.savedSession && root.savedSession.session_id)
                    text: root.translator.tr("training.resume")
                    onClicked: root.resumeSession()
                }
                Item { Layout.fillWidth: true }
                Button {
                    text: root.translator.tr("training.start")
                    highlighted: true
                    onClicked: root.requestNewSession()
                }
            }
        }

        ColumnLayout {
            spacing: 10

            RowLayout {
                Layout.fillWidth: true
                Label {
                    text: String(root.promptPack.name || "") + "  "
                          + root.translator.tr("training.progress", root.currentIndex + 1, root.order.length)
                    font.bold: true
                }
                Item { Layout.fillWidth: true }
            }

            Frame {
                Layout.fillWidth: true
                Layout.preferredHeight: 96
                Label {
                    anchors.fill: parent
                    text: String(root.currentPrompt.text || "")
                    font.pixelSize: 22
                    wrapMode: Text.WordWrap
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }

            PitchEditor {
                id: pitchEditor
                Layout.fillWidth: true
                Layout.fillHeight: true
                translator: root.translator
                accentColor: root.hostWindow.accent
                axisColor: root.hostWindow.borderColor
                gridColor: root.palette.alternateBase
                labelColor: root.palette.text
                defaultMoraDuration: root.backend.defaultMoraDuration
                defaultPauseDuration: root.backend.defaultPauseDuration
                onPitchPointTouched: index => {
                    root.clearPlayback();
                    const mask = root.copy(root.editMask);
                    if (index >= 0 && index < mask.length)
                        mask[index] = true;
                    root.editMask = mask;
                }
                onPointsEdited: points => {
                    root.clearPlayback();
                    root.manualOffsets = root.copy(points);
                    root.saveDraft();
                }
            }

            PitchHorizontalScrollBar {
                Layout.fillWidth: true
                Layout.preferredHeight: 14
                Layout.leftMargin: 12
                Layout.rightMargin: 12
                editor: pitchEditor
                trackColor: root.palette.mid
                thumbColor: root.hostWindow.accent
            }

            Label {
                Layout.fillWidth: true
                visible: root.backend.error.length > 0 || root.sessionMessage.length > 0
                text: root.backend.error.length > 0 ? root.backend.error : root.sessionMessage
                wrapMode: Text.WordWrap
            }

            PlaybackControls {
                Layout.fillWidth: true
                Layout.preferredHeight: 52
                Layout.leftMargin: 10
                Layout.rightMargin: 10
                translator: root.translator
                mutedText: root.hostWindow.mutedText
                busy: root.backend.busy
                playing: player.playbackState === MediaPlayer.PlayingState
                hasAudio: player.source.toString().length > 0
                canGenerate: root.promptReady
                position: player.position
                duration: player.duration
                errorText: root.backend.error
                onPrimaryClicked: {
                    if (player.playbackState === MediaPlayer.PlayingState) {
                        player.pause();
                    } else if (player.source.toString().length > 0) {
                        if (player.duration > 0 && player.position >= player.duration - 1)
                            player.position = 0;
                        player.play();
                    } else {
                        root.synthesizeCurrent();
                    }
                }
                onSeekRequested: position => player.position = position
            }

            RowLayout {
                Layout.fillWidth: true
                Button {
                    text: root.translator.tr("training.back")
                    enabled: root.currentIndex > 0 && !root.backend.busy
                    onClicked: root.goBack()
                }
                Item { Layout.fillWidth: true }
                Button {
                    text: root.translator.tr("training.skip")
                    enabled: !root.backend.busy
                    onClicked: root.skipCurrent()
                }
                Button {
                    text: root.translator.tr("training.ok")
                    highlighted: true
                    enabled: root.promptReady && root.playedCount > 0 && !root.backend.busy
                    onClicked: root.acceptCurrent()
                }
            }
        }

        ColumnLayout {
            spacing: 16
            Item { Layout.fillHeight: true }
            Label {
                Layout.fillWidth: true
                text: root.translator.tr("training.complete")
                font.pixelSize: 24
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                Layout.fillWidth: true
                text: root.translator.tr("training.completeCounts", root.acceptedCount, root.skippedCount)
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                Layout.fillWidth: true
                visible: root.sessionMessage.length > 0
                text: root.sessionMessage
                horizontalAlignment: Text.AlignHCenter
            }
            RowLayout {
                Layout.alignment: Qt.AlignHCenter
                Button {
                    text: root.translator.tr("training.back")
                    enabled: root.currentIndex > 0
                    onClicked: root.goBack()
                }
                Button {
                    text: root.translator.tr("training.export")
                    highlighted: true
                    enabled: root.acceptedCount > 0
                    onClicked: {
                        exportDialog.currentFile = root.backend.defaultSaveFile(
                                    "manual-prosody-" + root.sessionId + ".jsonl");
                        exportDialog.open();
                    }
                }
            }
            Item { Layout.fillHeight: true }
        }
    }

    onClosing: close => {
        if (root.sessionActive) {
            root.saveDraft();
            root.synthesisLogWindow.close();
        }
    }
}
