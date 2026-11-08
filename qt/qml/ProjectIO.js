.pragma library

// hashStringは、入力の一致判定に使う指紋を作る。暗号用途ではない。
function hashString(value) {
    let first = 0;
    let second = 0;
    for (let index = 0; index < value.length; ++index) {
        const code = value.charCodeAt(index);
        first = (first * 31 + code) | 0;
        second = (second * 131 + code) | 0;
    }
    return (first >>> 0).toString(36) + "-" + (second >>> 0).toString(36);
}

// analysisInputContextは、全発話で共通の解析入力をまとめる。
// 判定方法を変えたら版の値を上げる。保存済みのキャッシュは使わなくなる。
function analysisInputContext(host, appVersion) {
    return [
        "analysis-cache-1",
        String(appVersion || ""),
        host.appBackend.dictionaryFingerprint(),
        JSON.stringify(host.synthesisSettingContext()),
        host.intonationLab ? host.intonationLabBaseModelPath() : ""
    ].join("\u0001");
}

// analysisInputHashは、発話ごとの解析入力を指紋にする。
// 読み・原音境界・自動抑揚に効く値をすべて含める。
function analysisInputHash(item, host, context) {
    return hashString([
        context,
        item.content,
        item.language,
        host.resolvedPhonemizer(item.language, item.phonemizer, item.voicebankId),
        item.method,
        item.voicebankId,
        item.modelId,
        item.intonation,
        item.applyPitch ? "1" : "0",
        item.moraDuration,
        item.pauseDuration,
        host.hasManualMoraDurations(item)
                ? JSON.stringify(host.decodeSequence(item.moraDurationsJson)) : ""
    ].join("\u0001"));
}

// analysisCacheableは、保存した解析キャッシュを使ってよい発話かを返す。
// 英語の読みは音源の arpasing.yaml にも依存する。その内容は指紋にできないため、英語は使わない。
function analysisCacheable(item) {
    return String(item.language || "ja") !== "en";
}

// projectDataは、発話の一覧をプロジェクトの形式へまとめる。
// ctxはutterances（模型）、selectedIndex、host（Main.qmlのwindow）、appVersionを持つ。
function projectData(ctx) {
    const utterances = ctx.utterances;
    const host = ctx.host;
    const inputContext = analysisInputContext(host, ctx.appVersion);
    const savedUtterances = [];
    for (let index = 0; index < utterances.count; ++index) {
        const item = utterances.get(index);
        const saved = {
            text: item.content || "",
            language: item.language || "ja",
            phonemizer: item.phonemizer || host.defaultPhonemizer(item.language || "ja"),
            method: host.normalizeMethod(item.method, item.language || "ja"),
            voicebank_id: item.voicebankId || "",
            model_id: host.normalizeModelId(item.modelId, item.language || "ja"),
            alias_policy: host.normalizeAliasPolicy(item.aliasPolicy),
            tone: item.tone || "",
            color: item.color || "",
            mora_duration_ms: item.moraDuration,
            pause_duration_ms: item.pauseDuration,
            leading_preutterance_ms: item.leadingPreutterance,
            intonation: item.intonation,
            apply_pitch: !!item.applyPitch,
            pitch_points: host.decodeSequence(item.pointsJson),
            pitch_frames: host.decodeSequence(item.pitchFramesJson),
            mora_durations_ms: host.decodeSequence(item.moraDurationsJson),
            mora_positions_ms: host.decodeSequence(item.moraPositionsJson),
            automatic_pitch_points: host.automaticSequence(item, "autoPointsJson"),
            automatic_frame_pitch: host.automaticSequence(item, "autoPitchFramesJson"),
            automatic_frame_ms: Number(item.autoFrameMs) || 10,
            automatic_mora_durations_ms: host.automaticSequence(item, "autoMoraDurationsJson"),
            automatic_mora_positions_ms: host.automaticSequence(item, "autoMoraPositionsJson"),
            manual_pitch_edited: host.hasManualPitch(item),
            manual_mora_duration_edited: host.hasManualMoraDurations(item),
            phoneme_overrides: host.decodeSequence(item.phonemeOverridesJson),
            analysis_cache: {
                reading: item.reading || "",
                morae: host.decodeSequence(item.moraeJson),
                input_hash: analysisInputHash(item, host, inputContext)
            }
        };
        if (host.intonationLab) {
            saved.training_accepted = !!item.trainingAccepted;
            saved.lab_entry_id = item.labEntryId || "";
        }
        savedUtterances.push(saved);
    }
    const project = {
        format: "utautts-project",
        format_version: 9,
        app_version: ctx.appVersion,
        utterances: savedUtterances,
        selected_index: utterances.count ? ctx.selectedIndex : 0
    };
    if (host.intonationLab) {
        project.intonation_lab = {
            version: 2,
            corpus: "japanese-v1",
            base_model_path: host.intonationLabBaseModelPath(),
            completed: host.intonationLabFirstIncomplete() < 0
        };
    }
    return project;
}

// utteranceFromSavedは、保存された発話から、模型へ入れる項目を作る。
// ctxはhost（Main.qmlのwindow）、voicebank、id（発話ID）、formatVersion、analysisContextを持つ。
function utteranceFromSaved(saved, ctx) {
    const host = ctx.host;
    const bank = ctx.voicebank;
    const language = String(saved.language || "ja");
    const voicebankId = String(saved.voicebank_id || "");
    const points = host.copySequence(saved.pitch_points);
    const pitchFrames = host.copySequence(saved.pitch_frames);
    const manualDurations = host.copySequence(saved.mora_durations_ms);
    const automaticDurations = host.copySequence(saved.automatic_mora_durations_ms);
    let manualPositions = host.copySequence(saved.mora_positions_ms);
    let automaticPositions = host.copySequence(saved.automatic_mora_positions_ms);
    if (ctx.formatVersion < 2) {
        manualPositions = host.moraStartsFromCenters(manualPositions, manualDurations);
        automaticPositions = host.moraStartsFromCenters(automaticPositions, automaticDurations);
    }
    const item = {
        utteranceId: "utterance-" + ctx.id,
        content: String(saved.text || ""),
        language: language,
        phonemizer: String(saved.phonemizer || host.defaultPhonemizer(language)),
        method: host.normalizeMethod(saved.method, language),
        reading: "",
        moraeJson: "[]",
        pointsJson: JSON.stringify(points),
        pitchFramesJson: JSON.stringify(pitchFrames),
        moraDurationsJson: JSON.stringify(manualDurations),
        moraPositionsJson: JSON.stringify(manualPositions),
        autoPointsJson: JSON.stringify(host.copySequence(saved.automatic_pitch_points)),
        autoPitchFramesJson: JSON.stringify(host.copySequence(saved.automatic_frame_pitch)),
        autoFrameMs: Number(saved.automatic_frame_ms) || 10,
        autoMoraDurationsJson: JSON.stringify(automaticDurations),
        autoMoraPositionsJson: JSON.stringify(automaticPositions),
        phonemeOverridesJson: JSON.stringify(host.copySequence(saved.phoneme_overrides)),
        manualPitchEdited: saved.manual_pitch_edited === undefined
                ? points.some(value => Math.abs(Number(value)) > .1)
                  || pitchFrames.some(value => Math.abs(Number(value)) > .1)
                : !!saved.manual_pitch_edited,
        manualMoraDurationEdited: saved.manual_mora_duration_edited === undefined
                ? manualDurations.some(value => Number(value) > 0)
                : !!saved.manual_mora_duration_edited,
        trainingAccepted: !!saved.training_accepted,
        labEntryId: String(saved.lab_entry_id || ""),
        voicebankId: voicebankId,
        imagePath: bank ? bank.image_path || "" : "",
        modelId: host.normalizeModelId(saved.model_id, language),
        aliasPolicy: saved.alias_policy === undefined
                ? host.appBackend.defaultAliasPolicy : host.normalizeAliasPolicy(saved.alias_policy),
        // 形式8までは指定なしもC4で保存していた。
        tone: ctx.formatVersion < 9 && String(saved.tone) === "C4"
                ? "" : String(saved.tone === undefined ? host.appBackend.defaultTone : saved.tone),
        color: String(saved.color || ""),
        moraDuration: host.projectNumber(saved.mora_duration_ms, host.appBackend.defaultMoraDuration, 20, 1000, true),
        pauseDuration: host.projectNumber(saved.pause_duration_ms, host.appBackend.defaultPauseDuration, 0, 3000, true),
        leadingPreutterance: host.projectNumber(saved.leading_preutterance_ms, 0, 0, 300, true),
        intonation: host.projectNumber(saved.intonation, host.defaultIntonationStrength, 0, host.maxIntonationStrength, false),
        applyPitch: saved.apply_pitch === undefined ? true : !!saved.apply_pitch,
        revision: 0
    };
    // 解析の入力が保存時と同じときだけ、保存した読みと自動抑揚を使う。
    const cache = saved.analysis_cache || {};
    const cachedMorae = host.copySequence(cache.morae);
    if (analysisCacheable(item) && String(cache.reading || "").length && cachedMorae.length
            && String(cache.input_hash || "") === analysisInputHash(item, host, ctx.analysisContext)) {
        item.reading = String(cache.reading);
        item.moraeJson = JSON.stringify(cachedMorae);
    } else {
        // 入力が変わった。古い自動抑揚は使わず、解析し直してから予測させる。
        item.autoPointsJson = "[]";
        item.autoPitchFramesJson = "[]";
        item.autoMoraDurationsJson = "[]";
        item.autoMoraPositionsJson = "[]";
    }
    return item;
}

// missingVoicebanksは、保存された発話が指す音源のうち、今の環境に無いもののIDを返す。
function missingVoicebanks(savedUtterances, host) {
    const missing = [];
    const items = savedUtterances || [];
    for (let index = 0; index < items.length; ++index) {
        const id = String((items[index] || {}).voicebank_id || "");
        if (id.length && !host.voicebankById(id) && missing.indexOf(id) < 0)
            missing.push(id);
    }
    return missing;
}
