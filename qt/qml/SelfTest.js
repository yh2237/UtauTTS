// GUIセルフテスト。QMLのidに依存せず、ctxで受け取る。
// ctx = {window, utterances, editorContent, analyzeTimer}

function run(ctx) {
    const host = ctx.window;
    if (!host.injectedSelfTest)
        return "self-test mode is disabled";
    function check(condition, message) {
        return condition ? "" : message;
    }

    if (host.intonationLab) {
        let error = check(ctx.utterances.count === 50, "intonation lab examples were not loaded");
        if (error.length)
            return error;
        error = check(host.selectedIndex === 0
                      && host.current().content.length > 0
                      && host.intonationLabFirstIncomplete() === 0,
                      "intonation lab did not select the first example");
        return error;
    }

    let error = check(ctx.utterances.count === 1, "initial utterance is missing");
    if (error.length)
        return error;
    error = check(ctx.editorContent.phonemeEditor.sidePadding === ctx.editorContent.pitchEditor.sidePadding
                  && ctx.editorContent.phonemeEditor.timeToX(0) === ctx.editorContent.pitchEditor.pointX(0)
                  && Math.abs(ctx.editorContent.phonemeEditor.xToTime(
                          ctx.editorContent.phonemeEditor.timeToX(120)) - 120) < 0.001,
                  "editor timeline padding or hit coordinates are inconsistent");
    if (error.length)
        return error;
    error = checkFrameEditing(ctx.editorContent);
    if (error.length)
        return error;
    error = check(host.current().phonemizer === "auto"
                  && host.buildSynthesisRequest(host.current()).phonemizer !== "auto",
                  "normal GUI defaults are incorrect");
    if (error.length)
        return error;
    const contextRequest = host.buildSynthesisRequest(host.current());
    const contextSettings = contextRequest.renderer_settings || {};
    error = check(contextSettings.context_duration === undefined
                  && contextSettings.context_duration_strength === undefined,
                  "context duration was injected into the normal request");
    if (error.length)
        return error;
    error = check(contextSettings.boundary_tone === true,
                  "boundary tone settings were not injected into the request");
    if (error.length)
        return error;
    error = check(contextSettings.stretch_adapt === true,
                  "stretch adaptation settings were not injected into the request");
    if (error.length)
        return error;
    error = check(contextSettings.english_weak_form === true,
                  "English weak form setting was not injected into the request");
    if (error.length)
        return error;
    for (let voiceIndex = 0; voiceIndex < host.appBackend.voicebanks.length; ++voiceIndex) {
        const voice = host.appBackend.voicebanks[voiceIndex];
        if (!voice.suggested_language || !voice.suggested_phonemizer)
            continue;
        error = check(host.resolvedPhonemizer(voice.suggested_language, "auto", voice.id)
                      === voice.suggested_phonemizer,
                      "automatic phoneme format detection failed");
        if (error.length)
            return error;
    }
    const rendererId = host.defaultRendererId();
    const originalDefaultMoraDuration = host.appBackend.defaultMoraDuration;
    host.appBackend.setRendererSetting(rendererId, "mora_duration_ms",
                                       originalDefaultMoraDuration + 5);
    error = check(host.current().moraDuration === originalDefaultMoraDuration,
                  "changing renderer settings changed the current utterance");
    if (error.length)
        return error;
    host.addUtterance(false);
    error = check(host.current().moraDuration === originalDefaultMoraDuration + 5,
                  "new utterance did not use the renderer setting default");
    if (error.length)
        return error;
    host.removeUtterance();
    host.appBackend.setRendererSetting(rendererId, "mora_duration_ms",
                                       originalDefaultMoraDuration);
    error = check(
        (host.buildSynthesisRequest(host.current()).renderer_settings || {}).mora_duration_ms
            === undefined,
        "per-card mora duration was injected as a renderer setting override");
    if (error.length)
        return error;
    host.resetHistory(false);
    error = check(ctx.utterances.get(0).intonation === host.defaultIntonationStrength,
                  "initial intonation strength is incorrect");
    if (error.length)
        return error;
    host.updateUtteranceText(0, "こんにちは");
    ctx.analyzeTimer.stop();
    host.updatePitchPoints([20, -10]);
    error = check(host.canUndo && host.current().manualPitchEdited,
                  "pitch edit was not recorded");
    if (error.length)
        return error;
    host.undo();
    error = check(host.current().content === "こんにちは" && !host.current().manualPitchEdited,
                  "pitch undo changed text or kept the edit");
    if (error.length)
        return error;
    host.redo();
    error = check(host.current().content === "こんにちは" && host.current().manualPitchEdited,
                  "pitch redo failed");
    if (error.length)
        return error;

    host.updateMoraTiming([110, 130], [0, 110]);
    error = check(host.current().manualMoraDurationEdited, "mora timing edit was not recorded");
    if (error.length)
        return error;
    host.undo();
    error = check(!host.current().manualMoraDurationEdited, "mora timing undo failed");
    if (error.length)
        return error;
    host.redo();
    error = check(host.current().manualMoraDurationEdited, "mora timing redo failed");
    if (error.length)
        return error;

    ctx.editorContent.pitchEditor.morae = [{mora: "あ", pause: false}, {mora: "", pause: true}];
    ctx.editorContent.pitchEditor.moraDurations = [120, 180];
    ctx.editorContent.pitchEditor.moraPositions = [0, 120];
    error = check(ctx.editorContent.pitchEditor.durationIsEditable(1),
                  "pause duration is not editable");
    if (error.length)
        return error;
    ctx.editorContent.pitchEditor.updateEndPositionAt(
                ctx.editorContent.pitchEditor.sidePadding
                + 380 * ctx.editorContent.pitchEditor.durationScale);
    error = check(Math.round(ctx.editorContent.pitchEditor.durationAt(1)) === 260,
                  "pause duration edit failed");
    if (error.length)
        return error;

    host.addUtterance();
    error = check(ctx.utterances.count === 2, "utterance add failed");
    if (error.length)
        return error;
    ctx.utterances.setProperty(0, "moraeJson", JSON.stringify([{mora: "こ", pause: false}]));
    ctx.utterances.setProperty(0, "pointsJson", "[0]");
    ctx.utterances.setProperty(0, "autoPointsJson", "[75]");
    ctx.utterances.setProperty(0, "autoMoraDurationsJson", "[120]");
    ctx.utterances.setProperty(0, "autoMoraPositionsJson", "[0]");
    host.selectUtterance(0);
    error = check(Math.round(ctx.editorContent.pitchEditor.pitchAt(0)) === 75,
                  "card switch did not restore automatic prosody");
    if (error.length)
        return error;
    host.selectUtterance(1);
    host.moveUtterance(-1);
    error = check(host.selectedIndex === 0, "utterance move failed");
    if (error.length)
        return error;
    host.removeUtterance();
    error = check(ctx.utterances.count === 1, "utterance remove failed");
    if (error.length)
        return error;
    const project = host.projectData();
    error = check(project.format === "utautts-project" && project.format_version === 8
                  && project.utterances.length === 1, "project data generation failed");

    ctx.analyzeTimer.stop();
    ctx.utterances.clear();
    host.nextUtteranceId = 1;
    host.addUtterance(false);
    host.resetHistory(false);
    return error;
}

function checkFrameEditing(editorContent) {
    const component = Qt.createComponent("editors/PhonemeEditor.qml");
    if (component.status !== Component.Ready)
        return component.errorString();
    const editor = component.createObject(editorContent, {
        visible: false, translator: editorContent.phonemeEditor.translator,
        timingEditor: editorContent.pitchEditor,
        leadingMargin: 20, frameMs: 10, waveformDuration: 1000,
        moraPositions: [0, 100, 300], moraDurations: [50, 50, 50],
        units: [{position: 2, render_start_ms: 330}, {position: 0, render_start_ms: 0},
                {position: 2, render_start_ms: 290}, {position: 1, silent: true}]
    });
    if (!editor)
        return "フレーム編集テスト用のエディターを作れません";
    try {
        if (Math.abs(editor.timeToX(120) - editor.timeToX(0)
                - 120 * editorContent.pitchEditor.durationScale) > 0.001)
            return "拡張編集の初期倍率が基本編集と一致しません";
        editor.setFrameRange(0, 40, 100);
        const first = editor.manualFrames;
        if (first[0] !== 100 || first[5] !== 100 || first[6] !== 0
                || first[10] !== 0 || first[26] !== 0 || first[27] !== 100
                || first[35] !== 100 || first[36] !== 0)
            return "発音区間の両端・重なり・休止でフレーム補正が変わりました";
        editor.moraPositions = [0, 100, 400];
        editor.moraDurations = [100, 50, 100];
        editor.setFrameRange(0, 60, 200);
        if (editor.manualFrames[10] !== 200 || editor.manualFrames[11] !== 0
                || editor.manualFrames[50] !== 200 || editor.manualFrames[51] !== 0)
            return "時間編集後の発音区間が更新されません";
        return "";
    } finally {
        editor.destroy();
    }
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {run};
