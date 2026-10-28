.pragma library

// projectDataは、発話の一覧をプロジェクトの形式へまとめる。
// ctxはutterances（模型）、selectedIndex、host（Main.qmlのwindow）、appVersionを持つ。
function projectData(ctx) {
    const utterances = ctx.utterances;
    const host = ctx.host;
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
                morae: host.decodeSequence(item.moraeJson)
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
