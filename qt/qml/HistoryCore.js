// 編集履歴のスナップショットとundo/redo。QMLのidに依存せず、ctxで受け取る。
// ctx = {window: ApplicationWindow相当, utterances: ListModel, mergeTimer: Timer}

// 削除した発話を戻せるよう、編集対象だけでなく発話の全項目を控える。
function historyUtterance(item) {
    return {
        utteranceId: item.utteranceId,
        content: item.content,
        language: item.language,
        phonemizer: item.phonemizer,
        method: item.method,
        reading: item.reading,
        moraeJson: item.moraeJson,
        pointsJson: item.pointsJson,
        pitchFramesJson: item.pitchFramesJson,
        moraDurationsJson: item.moraDurationsJson,
        moraPositionsJson: item.moraPositionsJson,
        autoPointsJson: item.autoPointsJson,
        autoPitchFramesJson: item.autoPitchFramesJson,
        autoFrameMs: item.autoFrameMs,
        autoMoraDurationsJson: item.autoMoraDurationsJson,
        autoMoraPositionsJson: item.autoMoraPositionsJson,
        phonemeOverridesJson: item.phonemeOverridesJson,
        manualPitchEdited: item.manualPitchEdited,
        manualMoraDurationEdited: item.manualMoraDurationEdited,
        trainingAccepted: item.trainingAccepted,
        labEntryId: item.labEntryId,
        voicebankId: item.voicebankId,
        imagePath: item.imagePath,
        modelId: item.modelId,
        aliasPolicy: item.aliasPolicy,
        tone: item.tone,
        color: item.color,
        moraDuration: item.moraDuration,
        pauseDuration: item.pauseDuration,
        leadingPreutterance: item.leadingPreutterance,
        intonation: item.intonation,
        applyPitch: item.applyPitch
    };
}

// savedUtteranceItemは、控えた項目から模型へ入れる発話を作る。
function savedUtteranceItem(saved) {
    return {
        utteranceId: String(saved.utteranceId || ""),
        content: String(saved.content || ""),
        language: String(saved.language || "ja"),
        phonemizer: String(saved.phonemizer || "auto"),
        method: String(saved.method || ""),
        reading: String(saved.reading || ""),
        moraeJson: String(saved.moraeJson || "[]"),
        pointsJson: String(saved.pointsJson || "[]"),
        pitchFramesJson: String(saved.pitchFramesJson || "[]"),
        moraDurationsJson: String(saved.moraDurationsJson || "[]"),
        moraPositionsJson: String(saved.moraPositionsJson || "[]"),
        autoPointsJson: String(saved.autoPointsJson || "[]"),
        autoPitchFramesJson: String(saved.autoPitchFramesJson || "[]"),
        autoFrameMs: Number(saved.autoFrameMs) || 10,
        autoMoraDurationsJson: String(saved.autoMoraDurationsJson || "[]"),
        autoMoraPositionsJson: String(saved.autoMoraPositionsJson || "[]"),
        phonemeOverridesJson: String(saved.phonemeOverridesJson || "[]"),
        manualPitchEdited: !!saved.manualPitchEdited,
        manualMoraDurationEdited: !!saved.manualMoraDurationEdited,
        trainingAccepted: !!saved.trainingAccepted,
        labEntryId: String(saved.labEntryId || ""),
        voicebankId: String(saved.voicebankId || ""),
        imagePath: String(saved.imagePath || ""),
        modelId: String(saved.modelId || ""),
        aliasPolicy: String(saved.aliasPolicy || ""),
        tone: String(saved.tone || ""),
        color: String(saved.color || ""),
        moraDuration: Number(saved.moraDuration) || 0,
        pauseDuration: Number(saved.pauseDuration) || 0,
        leadingPreutterance: Number(saved.leadingPreutterance) || 0,
        intonation: Number(saved.intonation) || 0,
        applyPitch: saved.applyPitch === undefined ? true : !!saved.applyPitch,
        revision: 0
    };
}

// restoreUtteranceListは、模型の発話を控えた並びに合わせる。消えた発話は戻し、増えた発話は消す。
function restoreUtteranceList(ctx, savedItems) {
    const utterances = ctx.utterances;
    const savedIds = [];
    for (let index = 0; index < savedItems.length; ++index)
        savedIds.push(String(savedItems[index].utteranceId || ""));
    for (let index = utterances.count - 1; index >= 0; --index) {
        if (savedIds.indexOf(String(utterances.get(index).utteranceId)) < 0)
            utterances.remove(index);
    }
    for (let index = 0; index < savedItems.length; ++index) {
        const current = ctx.window.utteranceIndex(savedIds[index]);
        if (current === index)
            continue;
        if (current >= 0)
            utterances.move(current, index, 1);
        else
            utterances.insert(index, savedUtteranceItem(savedItems[index]));
    }
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
    restoreUtteranceList(ctx, savedItems);
    for (let savedIndex = 0; savedIndex < savedItems.length; ++savedIndex) {
        const saved = savedItems[savedIndex];
        const index = host.utteranceIndex(String(saved.utteranceId || ""));
        if (index < 0)
            continue;
        const item = ctx.utterances.get(index);
        if (item.content === saved.content
                && item.reading === saved.reading
                && item.moraeJson === saved.moraeJson
                && item.pointsJson === saved.pointsJson
                && item.pitchFramesJson === saved.pitchFramesJson
                && item.moraDurationsJson === saved.moraDurationsJson
                && item.moraPositionsJson === saved.moraPositionsJson
                && item.phonemeOverridesJson === saved.phonemeOverridesJson
                && item.manualPitchEdited === !!saved.manualPitchEdited
                && item.manualMoraDurationEdited === !!saved.manualMoraDurationEdited)
            continue;
        ctx.utterances.setProperty(index, "content", String(saved.content || ""));
        ctx.utterances.setProperty(index, "reading", String(saved.reading || ""));
        ctx.utterances.setProperty(index, "moraeJson", String(saved.moraeJson || "[]"));
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
    if (ctx.utterances.count) {
        host.selectedIndex = Math.max(0, Math.min(host.selectedIndex, ctx.utterances.count - 1));
        host.selectUtterance(host.selectedIndex);
    }
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
