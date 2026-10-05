pragma ComponentBehavior: Bound

import QtQuick
import "MoraPositions.js" as MoraPositions

QtObject {
    id: core
    required property var backend
    required property var translator

    function voicebankById(id) {
        for (let i = 0; i < core.backend.voicebanks.length; ++i)
            if (core.backend.voicebanks[i].id === id)
                return core.backend.voicebanks[i];
        return null;
    }

    function voicebankName(id) {
        const voice = core.voicebankById(id);
        return voice ? voice.name : core.translator.tr("main.voicebankNone");
    }

    function modelById(id) {
        for (let i = 0; i < core.backend.models.length; ++i)
            if (core.backend.models[i].id === id)
                return core.backend.models[i];
        return null;
    }

    function rendererById(id) {
        for (let i = 0; i < core.backend.renderers.length; ++i)
            if (core.backend.renderers[i].id === id)
                return core.backend.renderers[i];
        return null;
    }

    function defaultVoicebank() {
        const configured = String(core.backend.defaultVoicebankId || "");
        const selected = configured.length ? core.voicebankById(configured) : null;
        return selected || (core.backend.voicebanks.length ? core.backend.voicebanks[0] : null);
    }

    function defaultModelId() {
        const configured = String(core.backend.defaultModelId || "none");
        return configured === "none" || core.modelById(configured)
                ? configured : (core.backend.models.length ? core.backend.models[0].id : "none");
    }

    function defaultModelIdForLanguage(language) {
        const normalized = String(language || "ja").toLowerCase();
        if (normalized === "en" || normalized === "zh") {
            if (String(core.backend.defaultModelId || "none") === "none")
                return "none";
            for (let index = 0; index < core.backend.models.length; ++index) {
                const model = core.backend.models[index];
                if (String(model.language || "").toLowerCase() === normalized)
                    return model.id;
            }
            return "none";
        }
        if (normalized !== "ja")
            return "none";
        const selected = core.defaultModelId();
        if (selected === "none")
            return "none";
        const configured = core.modelById(selected);
        if (!configured || !String(configured.language || "").trim()
                || String(configured.language).toLowerCase() === "ja")
            return configured ? configured.id : "none";
        for (let index = 0; index < core.backend.models.length; ++index) {
            const model = core.backend.models[index];
            if (!String(model.language || "").trim()
                    || String(model.language).toLowerCase() === "ja")
                return model.id;
        }
        return "none";
    }

    function preferredRendererForModel(model) {
        const recommended = model && model.recommended_renderers ? model.recommended_renderers : [];
        for (let index = 0; index < recommended.length; ++index) {
            const renderer = core.rendererById(recommended[index]);
            if (renderer)
                return renderer.id;
        }
        return core.defaultRendererId();
    }

    function defaultRendererId() {
        const configured = String(core.backend.defaultRenderer || "");
        if (core.rendererById(configured))
            return configured;
        const available = core.backend.renderers;
        return available.length ? available[0].id : "";
    }

    function normalizeRendererId(id) {
        const rendererId = String(id || "");
        if (rendererId && core.rendererById(rendererId))
            return rendererId;
        return core.defaultRendererId();
    }

    function normalizeAliasPolicy(value) {
        const policy = String(value || "auto");
        return ["auto", "cvvc-enhanced", "vcv-prefer", "cvvc-prefer", "cv-only"].indexOf(policy) >= 0 ? policy : "auto";
    }

    function fileNamePart(value, fallback) {
        let result = String(value === undefined || value === null ? "" : value)
                .replace(/[<>:"\/\\|?*\x00-\x1F]/g, " ")
                .replace(/\s+/g, " ")
                .trim();
        while (result.endsWith(".") || result.endsWith(" "))
            result = result.slice(0, -1).trim();
        return result || fallback;
    }

    function audioFileName(item) {
        const voice = core.fileNamePart(core.voicebankName(item.voicebankId), "voicebank");
        const text = core.fileNamePart(item.content, "utterance-" + item.utteranceId);
        return voice + "_" + text + ".wav";
    }

    function dragAudioFileName(item, index) {
        const number = ("000" + String(index + 1)).slice(-3);
        return number + "_" + core.audioFileName(item);
    }

    function defaultPhonemizer(language) {
        if (language === "en")
            return "en-arpasing";
        if (language === "zh")
            return "zh-cvvc";
        return "ja-kana";
    }

    function phonemizerOptions(language) {
        const labels = {
            "auto": core.translator.tr("main.phonemizer.auto"),
            "ja-kana": core.translator.tr("main.phonemizer.jaKana"),
            "en-arpasing": core.translator.tr("main.phonemizer.enArpasing"),
            "en-delta": core.translator.tr("main.phonemizer.enDelta"),
            "en-vccv": core.translator.tr("main.phonemizer.enVccv"),
            "en-cv": core.translator.tr("main.phonemizer.enCv"),
            "zh-cvvc": core.translator.tr("main.phonemizer.zhCvvc")
        };
        if (language === "en")
            return ["auto", "en-arpasing", "en-delta", "en-vccv", "en-cv"].map(
                        id => ({id: id, display_name: labels[id]}));
        const id = core.defaultPhonemizer(language);
        return ["auto", id].map(value => ({id: value, display_name: labels[value]}));
    }

    function resolvedPhonemizer(language, phonemizer, voicebankId) {
        language = language || "ja";
        phonemizer = phonemizer || "auto";
        if (phonemizer !== "auto")
            return phonemizer;
        const voice = core.voicebankById(voicebankId || "");
        if (voice && String(voice.suggested_language || "") === language
                && voice.suggested_phonemizer)
            return String(voice.suggested_phonemizer);
        return core.defaultPhonemizer(language);
    }

    function voicebankTypeOptions(id, selectedColor) {
        const voice = core.voicebankById(id);
        const raw = voice && voice.types ? voice.types : [];
        const options = [];
        for (let index = 0; index < raw.length; ++index) {
            const source = raw[index] || {};
            const color = String(source.color || "");
            const optionId = String(source.id || ("subbank-" + index));
            options.push({
                id: optionId,
                color: color,
                display_name: color.length ? color : core.translator.tr("main.color.default")
            });
        }
        if (!options.length) {
            options.push({
                id: "__default__",
                color: "",
                display_name: core.translator.tr("main.color.default")
            });
        }
        if (selectedColor !== undefined && selectedColor !== null) {
            const selected = String(selectedColor || "");
            let found = false;
            for (let index = 0; index < options.length; ++index) {
                if (options[index].color === selected) {
                    found = true;
                    break;
                }
            }
            if (selected.length && !found) {
                options.push({
                    id: "__custom__:" + selected,
                    color: selected,
                    display_name: selected
                });
            }
        }
        return options;
    }

    function voicebankTypeOptionAt(id, index, selectedColor) {
        const options = core.voicebankTypeOptions(id, selectedColor);
        return index >= 0 && index < options.length ? options[index] : null;
    }

    function voicebankHasColor(id, color) {
        const target = String(color || "");
        const voice = core.voicebankById(id);
        const raw = voice && voice.types ? voice.types : [];
        if (!raw.length)
            return target === "";
        for (let index = 0; index < raw.length; ++index) {
            if (String((raw[index] || {}).color || "") === target)
                return true;
        }
        return false;
    }

    function typeIdForColor(id, color) {
        const target = String(color || "");
        const options = core.voicebankTypeOptions(id, target);
        for (let index = 0; index < options.length; ++index) {
            if (options[index].color === target)
                return options[index].id;
        }
        return options.length ? options[0].id : "";
    }

    function normalizedMoraPositions(positions) {
        return MoraPositions.normalizedMoraPositions(positions);
    }

    function moraStartsFromCenters(centers, durations) {
        return MoraPositions.moraStartsFromCenters(centers, durations);
    }
}
