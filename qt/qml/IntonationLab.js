// Intonation Labの例文準備と書き出し。QMLのidに依存せず、ctxで受け取る。
// ctx = {window: ApplicationWindow相当, utterances: ListModel, prepareTimer: Timer, qt: Qt}

function defaultFile(ctx) {
    const stamp = new Date().toISOString().replace(/[-:.TZ]/g, "");
    return ctx.window.appBackend.defaultSaveFile("intonation-lab-" + stamp + ".utautts");
}

function baseModelPath() {
    return "models/frame-intonation-tcn-v10.json";
}

function firstIncomplete(ctx) {
    for (let index = 0; index < ctx.utterances.count; ++index) {
        if (!ctx.utterances.get(index).trainingAccepted)
            return index;
    }
    return -1;
}

function prepareEntry(ctx, index) {
    const host = ctx.window;
    if (!host.intonationLab || index < 0 || index >= ctx.utterances.count)
        return;
    if (host.appBackend.busy || !host.metadataInitialized) {
        host.intonationLabPendingIndex = index;
        host.intonationLabStatus = "例文を準備しています。";
        ctx.prepareTimer.restart();
        return;
    }
    host.intonationLabPendingIndex = -1;
    host.selectUtterance(index);
    const item = host.current();
    if (item.reading.length)
        host.requestMissingProsodyPreview(index);
    else
        host.analyzeUtterance(index);
}

function initialize(ctx) {
    const host = ctx.window;
    if (!host.intonationLab || host.intonationLabInitialized)
        return;
    host.intonationLabInitialized = true;
    host.projectFile = defaultFile(ctx);
    ctx.utterances.clear();
    host.nextUtteranceId = 1;

    try {
        const examples = JSON.parse(host.injectedIntonationLabExamples);
        if (!Array.isArray(examples) || !examples.length)
            throw new Error("例文がありません。");
        for (const example of examples) {
            host.addUtterance(false);
            const index = ctx.utterances.count - 1;
            ctx.utterances.setProperty(index, "content", String(example.text || ""));
            ctx.utterances.setProperty(index, "labEntryId", String(example.id || "entry-" + (index + 1)));
            ctx.utterances.setProperty(index, "trainingAccepted", false);
        }
        host.selectedIndex = 0;
        host.projectDirty = false;
        host.resetHistory(false);
        host.intonationLabStatus = "例文を準備しています。";
        prepareEntry(ctx, 0);
    } catch (error) {
        host.intonationLabStatus = String(error);
    }
}

function completeEntry(ctx) {
    const host = ctx.window;
    if (!host.intonationLab || host.appBackend.busy || !ctx.utterances.count)
        return;
    const item = host.current();
    if (!item || !item.reading.length)
        return;
    ctx.utterances.setProperty(host.selectedIndex, "trainingAccepted", true);
    host.projectDirty = true;
    if (!host.projectFile.toString().length)
        host.projectFile = defaultFile(ctx);
    if (!host.appBackend.saveProject(host.projectFile, host.projectData())) {
        host.intonationLabStatus = "書き出しに失敗しました。";
        return;
    }
    host.appBackend.rememberRecentProject(host.projectFile);
    host.projectDirty = false;
    const next = firstIncomplete(ctx);
    if (next < 0) {
        host.intonationLabStatus = "全ての例文を書き出しました。";
        return;
    }
    host.intonationLabStatus = "書き出しました。次の文を準備しています。";
    ctx.qt.callLater(function() { prepareEntry(ctx, next); });
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {defaultFile, baseModelPath, firstIncomplete, prepareEntry, initialize, completeEntry};
