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
    backend.setExportTextEncoding(draft.exportTextEncoding);
    backend.setExportDirectory(draft.exportDirectory);
    backend.setExportOverwrite(draft.exportOverwrite);
    backend.setShortcutSequences(draft.synthesizeShortcut,
                                 draft.saveProjectShortcut,
                                 draft.reloadVoicebanksShortcut,
                                 draft.addUtteranceShortcut,
                                 draft.removeUtteranceShortcut,
                                 draft.undoShortcut,
                                 draft.redoShortcut,
                                 draft.exportShortcut,
                                 draft.exportAllShortcut);
    const synthesisSettings = draft.synthesisSettings || {};
    const settingIds = Object.keys(synthesisSettings);
    for (let index = 0; index < settingIds.length; ++index) {
        const id = settingIds[index];
        backend.setSynthesisSetting(id, synthesisSettings[id]);
    }
}

// loadCurrentは、バックエンドの設定を編集中の値へ読み込む。
// validは既定の音源とモデルが今も使えるかを確かめる関数を持つ。
function loadCurrent(draft, backend, valid) {
    draft.defaultVoicebankId = valid.voicebank(backend.defaultVoicebankId);
    draft.defaultModelId = valid.model(backend.defaultModelId);
    draft.defaultAliasPolicy = backend.defaultAliasPolicy;
    draft.defaultTone = backend.defaultTone;
    draft.exportTextEncoding = backend.exportTextEncoding;
    draft.exportDirectory = backend.exportDirectory;
    draft.exportOverwrite = backend.exportOverwrite;
    draft.darkMode = backend.darkMode;
    draft.language = backend.language;
    draft.ffmpegPath = backend.ffmpegPath;
    draft.audioOutputDeviceId = backend.audioOutputDeviceId;
    draft.closeLogOnSuccess = backend.closeLogOnSuccess;
    draft.updateCheckEnabled = backend.updateCheckEnabled;
    draft.preReleaseUpdateCheckEnabled = backend.preReleaseUpdateCheckEnabled;
    draft.previewCacheFileCount = backend.previewCacheFileCount;
    draft.autoPreviewEnabled = backend.autoPreviewEnabled;
    draft.extendedDetailsVisible = backend.extendedDetailsVisible;
    draft.synthesizeShortcut = backend.synthesizeShortcut;
    draft.saveProjectShortcut = backend.saveProjectShortcut;
    draft.reloadVoicebanksShortcut = backend.reloadVoicebanksShortcut;
    draft.addUtteranceShortcut = backend.addUtteranceShortcut;
    draft.removeUtteranceShortcut = backend.removeUtteranceShortcut;
    draft.undoShortcut = backend.undoShortcut;
    draft.redoShortcut = backend.redoShortcut;
    draft.exportShortcut = backend.exportShortcut;
    draft.exportAllShortcut = backend.exportAllShortcut;

    const synthesisSettings = {};
    const definitions = backend.settingDefinitions || [];
    for (let index = 0; index < definitions.length; ++index) {
        const setting = definitions[index] || {};
        const id = String(setting.id || "");
        if (!id.length)
            continue;
        synthesisSettings[id] = backend.synthesisSetting(id, setting.default);
    }
    draft.synthesisSettings = synthesisSettings;
}

// defaultsは、各項目の初期値。defaultModelIdだけはカタログの先頭を使う。
const defaults = {
    defaultVoicebankId: "",
    defaultAliasPolicy: "auto",
    defaultTone: "",
    exportTextEncoding: "utf-8",
    exportOverwrite: false,
    darkMode: false,
    language: "auto",
    ffmpegPath: "",
    audioOutputDeviceId: "",
    closeLogOnSuccess: true,
    updateCheckEnabled: true,
    preReleaseUpdateCheckEnabled: false,
    previewCacheFileCount: 32,
    autoPreviewEnabled: true,
    extendedDetailsVisible: false,
    synthesizeShortcut: "Ctrl+Enter",
    saveProjectShortcut: "Ctrl+S",
    reloadVoicebanksShortcut: "Ctrl+O",
    addUtteranceShortcut: "Ctrl+D",
    removeUtteranceShortcut: "Delete",
    undoShortcut: "Ctrl+Z",
    redoShortcut: "Ctrl+Y",
    exportShortcut: "Ctrl+E",
    exportAllShortcut: "Ctrl+Shift+E"
};

function reset(draft, field, backend) {
    if (field === "defaultModelId") {
        draft.defaultModelId = backend.models.length ? String(backend.models[0].id) : "none";
        return;
    }
    if (field === "exportDirectory") {
        draft.exportDirectory = backend.defaultExportDirectory;
        return;
    }
    if (Object.prototype.hasOwnProperty.call(defaults, field))
        draft[field] = defaults[field];
}
