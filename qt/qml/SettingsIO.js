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

// applySettingsは、編集中の設定をまとめて保存する。
function applySettings(draft, backend) {
    backend.setSynthesisDefaults(draft.defaultModelId,
                                 draft.defaultTone,
                                 draft.defaultAliasPolicy);
    backend.setDarkMode(draft.darkMode);
    backend.setLanguage(draft.language);
    backend.setFfmpegPath(draft.ffmpegPath);
    backend.setAudioOutputDeviceId(draft.audioOutputDeviceId);
    backend.setCloseLogOnSuccess(draft.closeLogOnSuccess);
    backend.setUpdateCheckEnabled(draft.updateCheckEnabled);
    backend.setPreReleaseUpdateCheckEnabled(draft.preReleaseUpdateCheckEnabled);
    backend.setPreviewCacheFileCount(draft.previewCacheFileCount);
    backend.setAutoPreviewEnabled(draft.autoPreviewEnabled);
    backend.setExtendedDetailsVisible(draft.extendedDetailsVisible);
    backend.setDefaultVoicebank(draft.defaultVoicebankId);
    backend.setExportSettings(draft.exportTextWithWav,
                             draft.exportLabWithWav,
                             draft.exportTextEncoding);
    backend.setShortcutSequences(draft.synthesizeShortcut,
                                 draft.saveProjectShortcut,
                                 draft.reloadVoicebanksShortcut,
                                 draft.addUtteranceShortcut,
                                 draft.removeUtteranceShortcut,
                                 draft.undoShortcut,
                                 draft.redoShortcut);
}
