// 編集履歴のスナップショットとundo/redo。QMLのidに依存せず、ctxで受け取る。
// ctx = {window: ApplicationWindow相当, utterances: ListModel, mergeTimer: Timer}

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

function historySnapshot(ctx) {
    const items = [];
    for (let index = 0; index < ctx.utterances.count; ++index)
        items.push(historyUtterance(ctx.utterances.get(index)));
    return JSON.stringify(items);
}

function editableFingerprint(ctx) {
    const items = [];
    for (let index = 0; index < ctx.utterances.count; ++index) {
        const item = ctx.utterances.get(index);
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

function beginHistoryChange(ctx, key, merge) {
    const host = ctx.window;
    if (host.historyRestoring)
        return;
    const normalizedKey = String(key || "");
    if (merge && normalizedKey.length && host.historyMergeKey === normalizedKey) {
        ctx.mergeTimer.restart();
        return;
    }
    host.undoStack = limitedHistory(host.undoStack, historySnapshot(ctx));
    host.redoStack = [];
    host.historyMergeKey = merge ? normalizedKey : "";
    if (merge)
        ctx.mergeTimer.restart();
    else
        ctx.mergeTimer.stop();
}

function endHistoryGesture(ctx) {
    ctx.window.historyMergeKey = "";
    ctx.mergeTimer.stop();
}

function resetHistory(ctx, markDirty) {
    const host = ctx.window;
    host.undoStack = [];
    host.redoStack = [];
    endHistoryGesture(ctx);
    host.savedProjectFingerprint = markDirty ? "__unsaved_project__" : editableFingerprint(ctx);
    host.projectDirty = !!markDirty;
}

function clearEditHistory(ctx) {
    const host = ctx.window;
    host.undoStack = [];
    host.redoStack = [];
    endHistoryGesture(ctx);
}

function restoreHistorySnapshot(ctx, snapshot) {
    const host = ctx.window;
    let savedItems;
    try {
        savedItems = JSON.parse(snapshot);
    } catch (error) {
        return false;
    }
    if (!Array.isArray(savedItems))
        return false;

    host.historyRestoring = true;
    host.clearPlayback();
    host.pendingProsodyRequestId = "";
    host.pendingProsodyUtteranceId = "";
    host.pendingProsodyRevision = -1;
    for (let savedIndex = 0; savedIndex < savedItems.length; ++savedIndex) {
        const saved = savedItems[savedIndex];
        const index = host.utteranceIndex(String(saved.utteranceId || ""));
        if (index < 0)
            continue;
        const item = ctx.utterances.get(index);
        if (item.pointsJson === saved.pointsJson
                && item.pitchFramesJson === saved.pitchFramesJson
                && item.moraDurationsJson === saved.moraDurationsJson
                && item.moraPositionsJson === saved.moraPositionsJson
                && item.phonemeOverridesJson === saved.phonemeOverridesJson
                && item.manualPitchEdited === !!saved.manualPitchEdited
                && item.manualMoraDurationEdited === !!saved.manualMoraDurationEdited)
            continue;
        ctx.utterances.setProperty(index, "pointsJson", String(saved.pointsJson || "[]"));
        ctx.utterances.setProperty(index, "pitchFramesJson", String(saved.pitchFramesJson || "[]"));
        ctx.utterances.setProperty(index, "moraDurationsJson", String(saved.moraDurationsJson || "[]"));
        ctx.utterances.setProperty(index, "moraPositionsJson", String(saved.moraPositionsJson || "[]"));
        ctx.utterances.setProperty(index, "phonemeOverridesJson",
                                   String(saved.phonemeOverridesJson || "[]"));
        ctx.utterances.setProperty(index, "manualPitchEdited", !!saved.manualPitchEdited);
        ctx.utterances.setProperty(index, "manualMoraDurationEdited", !!saved.manualMoraDurationEdited);
        host.markUtteranceDirty(index, false);
    }
    if (ctx.utterances.count)
        host.selectUtterance(host.selectedIndex);
    host.historyRestoring = false;
    host.projectDirty = editableFingerprint(ctx) !== host.savedProjectFingerprint;
    return true;
}

function undo(ctx) {
    const host = ctx.window;
    if (!host.canUndo)
        return;
    endHistoryGesture(ctx);
    const previous = host.undoStack[host.undoStack.length - 1];
    const remaining = host.undoStack.slice(0, host.undoStack.length - 1);
    const currentSnapshot = historySnapshot(ctx);
    if (!restoreHistorySnapshot(ctx, previous))
        return;
    host.undoStack = remaining;
    host.redoStack = limitedHistory(host.redoStack, currentSnapshot);
}

function redo(ctx) {
    const host = ctx.window;
    if (!host.canRedo)
        return;
    endHistoryGesture(ctx);
    const next = host.redoStack[host.redoStack.length - 1];
    const remaining = host.redoStack.slice(0, host.redoStack.length - 1);
    const currentSnapshot = historySnapshot(ctx);
    if (!restoreHistorySnapshot(ctx, next))
        return;
    host.redoStack = remaining;
    host.undoStack = limitedHistory(host.undoStack, currentSnapshot);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {historyUtterance, historySnapshot, editableFingerprint, limitedHistory,
                       beginHistoryChange, endHistoryGesture, resetHistory, clearEditHistory,
                       restoreHistorySnapshot, undo, redo};
