// 文章カードの状態更新とプレビュー処理。QMLのidに依存せず、ctxで受け取る。
// ctx = {window: ApplicationWindow相当, utterances: ListModel, player: MediaPlayer,
//        overrides: UnitOverrides, qt: Qt, platform: Platform, saveDialog}

function updateUnitOverride(ctx, unitIndex, key, value) {
    if (!ctx.utterances.count || unitIndex < 0 || !String(key || "").length)
        return;
    const host = ctx.window;
    const item = host.current();
    const normalizedKey = String(key);
    if (value === undefined || value === null || (typeof value === "number" && !Number.isFinite(value)))
        return;
    const values = ctx.overrides.update(host.decodeSequence(item.phonemeOverridesJson),
                                        unitIndex, normalizedKey, value);
    const encoded = JSON.stringify(values);
    if (item.phonemeOverridesJson === encoded)
        return;
    host.beginHistoryChange("unit:" + item.utteranceId + ":" + unitIndex + ":" + normalizedKey, true);
    ctx.utterances.setProperty(host.selectedIndex, "phonemeOverridesJson", encoded);
    host.markUtteranceDirty(host.selectedIndex);
    host.scheduleAutoPreview();
}

function clearUnitOverride(ctx, unitIndex) {
    if (!ctx.utterances.count || unitIndex < 0)
        return;
    const host = ctx.window;
    const item = host.current();
    const values = ctx.overrides.remove(host.decodeSequence(item.phonemeOverridesJson), unitIndex);
    const encoded = JSON.stringify(values);
    if (item.phonemeOverridesJson === encoded)
        return;
    host.beginHistoryChange("unit:" + item.utteranceId + ":" + unitIndex + ":clear", true);
    ctx.utterances.setProperty(host.selectedIndex, "phonemeOverridesJson", encoded);
    host.markUtteranceDirty(host.selectedIndex);
    host.scheduleAutoPreview();
}

function requestProsodyPreview(ctx, index) {
    const host = ctx.window;
    if (host.batchExportActive || index < 0 || index >= ctx.utterances.count)
        return;
    const item = ctx.utterances.get(index);
    if (!item.content.trim())
        return;
    if (host.appBackend.busy) {
        // 合成中の予測要求は破棄されるため、完了後に再試行する。
        host.pendingProsodyPreviewIndex = index;
        return;
    }
    if (host.pendingProsodyInFlight && host.pendingProsodyUtteranceId === item.utteranceId
            && host.pendingProsodyRevision === item.revision)
        return;
    const requestId = item.utteranceId + ":" + item.revision + ":" + Date.now();
    host.pendingProsodyRequestId = requestId;
    host.pendingProsodyUtteranceId = item.utteranceId;
    host.pendingProsodyRevision = item.revision;
    host.pendingProsodyInFlight = true;
    host.pendingProsodyPreviewIndex = -1;
    host.appBackend.predictProsody(host.buildProsodyRequest(item, requestId));
}

function handleProsodyChanged(ctx) {
    const host = ctx.window;
    if (host.appBackend.prosodyRequestId !== host.pendingProsodyRequestId)
        return;
    host.pendingProsodyInFlight = false;
    const index = host.utteranceIndex(host.pendingProsodyUtteranceId);
    if (index < 0 || ctx.utterances.get(index).revision !== host.pendingProsodyRevision) {
        ctx.qt.callLater(host.flushProsodyPreviewRequest);
        return;
    }
    let result;
    try {
        result = JSON.parse(host.appBackend.prosodyJson);
    } catch (error) {
        return;
    }
    // 解析を省いた要求では、予測結果から読みを補う。
    if (!ctx.utterances.get(index).reading && result.reading) {
        host.applyPronunciation(index, result.reading, host.copySequence(result.morae));
    }
    const automaticPoints = host.copySequence(result.pitch_points);
    const automaticDurations = host.copySequence(result.mora_durations_ms);
    const automaticPositions = host.copySequence(result.mora_positions_ms);
    host.applyAutomaticProsody(index, automaticPoints, automaticDurations, automaticPositions);
    host.applyAutomaticFramePitch(index, host.copySequence(result.frame_pitch_cents),
            Number(result.frame_ms) || 10);
    host.scheduleExtendedEditorWaveform();
}

function handlePreviewReady(ctx) {
    const host = ctx.window;
    const audio = host.appBackend.previewUrl;
    const pendingId = host.pendingUtteranceId;
    const pendingRevision = host.pendingRevision;
    const index = host.utteranceIndex(pendingId);
    if (host.batchExportActive) {
        if (index < 0 || ctx.utterances.get(index).revision !== pendingRevision) {
            host.finishBatchExport(false);
            return;
        }
        const fileName = host.batchExportMode === "drag"
                ? host.dragAudioFileName(ctx.utterances.get(index), index)
                : host.audioFileName(ctx.utterances.get(index));
        const destination = ctx.platform.hasNativeFileDialog
                ? host.appBackend.fileInDirectory(host.batchExportDirectory, fileName)
                : host.appBackend.defaultSaveFile(fileName);
        host.pendingUtteranceId = "";
        host.pendingRevision = -1;
        if (!destination.toString().length || !host.appBackend.savePreview(destination)) {
            host.finishBatchExport(false);
            return;
        }
        ++host.batchExportCompleted;
        if (host.batchExportMode === "drag")
            host.dragExportFiles.push(destination);
        ctx.qt.callLater(function() { host.synthesizeBatchItem(); });
        return;
    }
    if (host.saveRequestPending) {
        if (index < 0 || ctx.utterances.get(index).revision !== pendingRevision) {
            host.saveRequestPending = false;
            host.pendingUtteranceId = "";
            host.pendingRevision = -1;
            return;
        }
        host.saveRequestPending = false;
        host.pendingUtteranceId = "";
        host.pendingRevision = -1;
        const audioDestination = host.appBackend.defaultSaveFile(host.audioFileName(ctx.utterances.get(index)));
        if (host.appBackend.closeLogOnSuccess)
            host.closeLogWindow();
        if (ctx.platform.hasNativeFileDialog) {
            ctx.saveDialog.currentFile = audioDestination;
            ctx.saveDialog.open();
        } else {
            host.appBackend.savePreview(audioDestination);
        }
        return;
    }
    if (host.playbackQueueActive) {
        if (index < 0 || index !== host.selectedIndex || ctx.utterances.get(index).revision !== host.pendingRevision) {
            host.stopPlaybackQueue();
            return;
        }
        host.audioUtteranceId = host.pendingUtteranceId;
        host.audioRevision = host.pendingRevision;
        host.pendingUtteranceId = "";
        host.pendingRevision = -1;
        host.playbackError = "";
        host.playbackRequested = true;
        ctx.player.stop();
        ctx.player.source = audio;
        ctx.player.play();
        return;
    }
    if (index < 0 || index !== host.selectedIndex || ctx.utterances.get(index).revision !== host.pendingRevision) {
        host.pendingUtteranceId = "";
        host.pendingRevision = -1;
        host.autoplayPreview = false;
        host.scheduleAutoPreview();
        return;
    }
    host.updateSynthesisViewFromBackend(host.pendingUtteranceId, host.pendingRevision);
    host.audioUtteranceId = host.pendingUtteranceId;
    host.audioRevision = host.pendingRevision;
    host.pendingUtteranceId = "";
    host.pendingRevision = -1;
    host.playbackError = "";
    ctx.player.stop();
    ctx.player.source = audio;
    if (host.autoplayPreview !== false) {
        host.playbackRequested = true;
        if (host.appBackend.closeLogOnSuccess)
            host.closeLogWindow();
        ctx.player.play();
    } else {
        host.playbackRequested = false;
        host.autoplayPreview = true;
    }
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {updateUnitOverride, clearUnitOverride, requestProsodyPreview, handleProsodyChanged, handlePreviewReady};
