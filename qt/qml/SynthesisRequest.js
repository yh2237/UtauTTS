.pragma library

// synthesisは、発話から合成の要求を作る。
// ctxはhost（Main.qmlのwindow）とvoicebankFallback（音源が未選択のときの値）を持つ。
function synthesis(item, ctx) {
    const host = ctx.host;
    const points = host.decodeSequence(item.pointsJson);
    const morae = host.decodeSequence(item.moraeJson);
    const manualPitch = host.hasManualPitch(item);
    const manualDurations = host.hasManualMoraDurations(item)
            ? host.decodeSequence(item.moraDurationsJson) : [];
    const request = {
        text: item.content,
        reading: item.reading || "",
        language: item.language || "ja",
        phonemizer: host.resolvedPhonemizer(item.language, item.phonemizer, item.voicebankId),
        method: host.normalizeMethod(item.method, item.language || "ja"),
        dictionary: host.appBackend.dictionaryEntries,
        voicebank_id: item.voicebankId || ctx.voicebankFallback,
        model_id: item.modelId,
        alias_policy: host.normalizeAliasPolicy(item.aliasPolicy),
        tone: item.tone,
        color: item.color || "",
        mora_duration_ms: item.moraDuration,
        pause_duration_ms: item.pauseDuration,
        leading_preutterance_ms: item.leadingPreutterance,
        mora_durations_ms: manualDurations,
        intonation_strength: item.intonation,
        apply_pitch: item.applyPitch,
        unit_overrides: host.decodeSequence(item.phonemeOverridesJson)
    };
    host.addSynthesisSettings(request);
    const frameOffsets = host.decodeSequence(item.pitchFramesJson);
    const hasFrameOffsets = frameOffsets.some(value => Math.abs(Number(value)) > .1);
    if (item.applyPitch && item.reading && manualPitch && hasFrameOffsets) {
        request.manual_pitch = {
            version: 1,
            reading: item.reading,
            mode: "frames",
            frames: frameOffsets.map(value => Math.max(-4800, Math.min(4800, Number(value) || 0)))
        };
    } else if (item.applyPitch && item.reading && manualPitch
               && points.some(value => Math.abs(Number(value)) > .1)) {
        const manualPoints = [];
        for (let index = 0; index < points.length; ++index) {
            const mora = index < morae.length ? morae[index] : null;
            if (mora && mora.pause)
                continue;
            manualPoints.push({
                position: index,
                mora: mora ? mora.mora || "" : "",
                cents: Math.max(-4800, Math.min(4800, Number(points[index]) || 0))
            });
        }
        request.manual_pitch = {
            version: 1,
            reading: item.reading,
            mode: "offset",
            points: manualPoints
        };
    }
    if (host.intonationLab)
        request.model_path = host.intonationLabBaseModelPath();
    return request;
}

// prosodyは、発話から抑揚のプレビューの要求を作る。
function prosody(item, requestId, ctx) {
    const host = ctx.host;
    const request = {
        request_id: requestId,
        text: item.content,
        reading: item.reading || "",
        language: item.language || "ja",
        phonemizer: host.resolvedPhonemizer(item.language, item.phonemizer, item.voicebankId),
        method: host.normalizeMethod(item.method, item.language || "ja"),
        dictionary: host.appBackend.dictionaryEntries,
        model_id: item.modelId,
        model_path: host.intonationLab ? host.intonationLabBaseModelPath() : "",
        mora_duration_ms: item.moraDuration,
        pause_duration_ms: item.pauseDuration,
        mora_durations_ms: host.hasManualMoraDurations(item)
                ? host.decodeSequence(item.moraDurationsJson) : [],
        intonation_strength: item.intonation,
        apply_pitch: item.applyPitch
    };
    host.addSynthesisSettings(request);
    return request;
}

// settingContextは、合成に効く設定の現在値をまとめる。
function settingContext(host) {
    const context = {};
    const settings = host.appBackend.settingDefinitions;
    for (let index = 0; index < settings.length; ++index) {
        const setting = settings[index];
        context[String(setting.id)] = host.appBackend.synthesisSetting(setting.id, setting.default);
    }
    return context;
}

// addSettingsは、要求にまだ入っていない設定だけを足す。
function addSettings(request, host) {
    const settings = settingContext(host);
    const keys = Object.keys(settings);
    for (let index = 0; index < keys.length; ++index) {
        if (Object.prototype.hasOwnProperty.call(request, keys[index]))
            delete settings[keys[index]];
    }
    if (Object.keys(settings).length)
        request.settings = settings;
    return request;
}
