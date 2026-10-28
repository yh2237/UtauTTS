.pragma library

// shortcutConflictは、同じショートカットが2つ以上あるかを返す。
// normalizeは、比較用の文字列へ直す関数。
function shortcutConflict(shortcuts, normalize) {
    const used = [];
    for (let index = 0; index < shortcuts.length; ++index) {
        const shortcut = String(shortcuts[index] || "").trim();
        if (!shortcut.length)
            continue;
        const key = normalize(shortcut);
        if (used.indexOf(key) >= 0)
            return true;
        used.push(key);
    }
    return false;
}

// applySettingsは、設定画面の値を保存する。
function applySettings(view, backend) {
    backend.setSynthesisDefaults(view.pendingDefaultModelId,
                                 view.pendingDefaultTone,
                                 view.pendingDefaultAliasPolicy);
    backend.setDarkMode(view.pendingDarkMode);
    backend.setLanguage(view.pendingLanguage);
    backend.setFfmpegPath(view.pendingFfmpegPath);
    backend.setAudioOutputDeviceId(view.pendingAudioOutputDeviceId);
    backend.setCloseLogOnSuccess(view.pendingCloseLogOnSuccess);
    backend.setUpdateCheckEnabled(view.pendingUpdateCheckEnabled);
    backend.setPreReleaseUpdateCheckEnabled(view.pendingPreReleaseUpdateCheckEnabled);
    backend.setPreviewCacheFileCount(view.pendingPreviewCacheFileCount);
    backend.setAutoPreviewEnabled(view.pendingAutoPreviewEnabled);
    backend.setExtendedDetailsVisible(view.pendingExtendedDetailsVisible);
    backend.setDefaultVoicebank(view.pendingDefaultVoicebankId);
    backend.setExportSettings(view.pendingExportTextWithWav,
                             view.pendingExportLabWithWav,
                             view.pendingExportTextEncoding);
    backend.setShortcutSequences(view.pendingSynthesizeShortcut,
                                 view.pendingSaveProjectShortcut,
                                 view.pendingReloadVoicebanksShortcut,
                                 view.pendingAddUtteranceShortcut,
                                 view.pendingRemoveUtteranceShortcut,
                                 view.pendingUndoShortcut,
                                 view.pendingRedoShortcut);
}
