package synth

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/tts"
)

// 数値はnumber/integerともfloat64で扱う。
type settingKind int

const (
	settingKindNumber settingKind = iota
	settingKindBoolean
)

// 既定値はinternal/settingsから取得し、このテーブルで適用先を指定する。
type settingSpec struct {
	id           string
	kind         settingKind
	defaultValue any
	apply        func(value any, cfg *tts.Config)
	// nilならsettingsだけで受け取る。
	typed func(request Request) any
}

var settingSpecs = []settingSpec{
	numberSetting("mora_duration_ms",
		func(r Request) any { return r.MoraDurationMS },
		func(value float64, cfg *tts.Config) { cfg.MoraDurationMS = value }),
	numberSetting("pause_duration_ms",
		func(r Request) any { return r.PauseDurationMS },
		func(value float64, cfg *tts.Config) { cfg.PauseDurationMS = value }),
	numberSetting("leading_preutterance_ms",
		func(r Request) any { return r.LeadingPreutteranceMS },
		func(value float64, cfg *tts.Config) { cfg.LeadingPreutteranceMS = value }),
	numberSetting("intonation_strength",
		func(r Request) any { return r.IntonationStrength },
		func(value float64, cfg *tts.Config) { cfg.IntonationStrength = value }),
	boolSetting("context_duration",
		func(r Request) any { return r.ContextDuration },
		func(value bool, cfg *tts.Config) { cfg.ContextDuration = &value }),
	numberSetting("context_duration_strength",
		func(r Request) any { return r.ContextDurationStrength },
		func(value float64, cfg *tts.Config) { cfg.ContextDurationStrength = value }),
	boolSetting("boundary_tone",
		func(r Request) any { return r.BoundaryTone },
		func(value bool, cfg *tts.Config) { cfg.BoundaryTone = &value }),
	numberSetting("boundary_tone_strength",
		func(r Request) any { return r.BoundaryToneStrength },
		func(value float64, cfg *tts.Config) { cfg.BoundaryToneStrength = value }),
	boolSetting("stretch_adapt",
		func(r Request) any { return r.StretchAdapt },
		func(value bool, cfg *tts.Config) { cfg.StretchAdapt = &value }),
	numberSetting("stretch_adapt_strength",
		func(r Request) any { return r.StretchAdaptStrength },
		func(value float64, cfg *tts.Config) { cfg.StretchAdaptStrength = value }),
	boolSetting("pause_context",
		func(r Request) any { return r.PauseContext },
		func(value bool, cfg *tts.Config) { cfg.PauseContext = &value }),
	numberSetting("pause_context_strength",
		func(r Request) any { return r.PauseContextStrength },
		func(value float64, cfg *tts.Config) { cfg.PauseContextStrength = value }),
	{
		id: "timing_warp", kind: settingKindBoolean, defaultValue: settings.Bool("timing_warp"),
		typed: func(r Request) any { return r.World.TimingWarpEnabled() },
		apply: func(value any, cfg *tts.Config) {
			enabled := value.(bool)
			cfg.World.TimingWarp = &enabled
		},
	},
	{
		id: "microprosody", kind: settingKindBoolean, defaultValue: settings.Bool("microprosody"),
		typed: func(r Request) any { return r.World.MicroprosodyEnabled() },
		apply: func(value any, cfg *tts.Config) {
			enabled := value.(bool)
			cfg.World.Microprosody = &enabled
		},
	},
}

func numberSetting(id string, typed func(Request) any, apply func(float64, *tts.Config)) settingSpec {
	return settingSpec{
		id: id, kind: settingKindNumber, defaultValue: settings.Number(id), typed: typed,
		apply: func(value any, cfg *tts.Config) {
			apply(value.(float64), cfg)
		},
	}
}

func boolSetting(id string, typed func(Request) any, apply func(bool, *tts.Config)) settingSpec {
	return settingSpec{
		id: id, kind: settingKindBoolean, defaultValue: settings.Bool(id), typed: typed,
		apply: func(value any, cfg *tts.Config) {
			apply(value.(bool), cfg)
		},
	}
}

// 優先順はsettings、型付き設定、既定値。使わなかった値は診断で返す。
func resolveSettings(request Request, cfg *tts.Config) []render.RenderDiagnostic {
	for _, spec := range settingSpecs {
		value := spec.defaultValue
		if spec.typed != nil {
			value = spec.typed(request)
		}
		spec.apply(value, cfg)
	}
	return applySettingsMap(settingSpecs, request.Settings, cfg)
}

// 未知のID、型の合わない値、範囲外の値は使わず、診断を返す。
func applySettingsMap(specs []settingSpec, values map[string]json.RawMessage, cfg *tts.Config) []render.RenderDiagnostic {
	var diagnostics []render.RenderDiagnostic
	ignore := func(format string, args ...any) {
		diagnostics = append(diagnostics, render.RenderDiagnostic{
			Severity: "warning", Code: "setting-ignored", Message: fmt.Sprintf(format, args...),
		})
	}
	for _, id := range slices.Sorted(maps.Keys(values)) {
		spec, known := settingSpecFor(specs, id)
		if !known {
			ignore("unknown setting %q was ignored", id)
			continue
		}
		raw := values[id]
		switch spec.kind {
		case settingKindBoolean:
			var value bool
			if err := json.Unmarshal(raw, &value); err != nil {
				ignore("setting %q expects a boolean: %v", id, err)
				continue
			}
			spec.apply(value, cfg)
		default:
			var value float64
			if err := json.Unmarshal(raw, &value); err != nil {
				ignore("setting %q expects a number: %v", id, err)
				continue
			}
			if setting, known := settings.Lookup(id); known &&
				(math.IsNaN(value) || setting.Min != nil && value < *setting.Min || setting.Max != nil && value > *setting.Max) {
				ignore("setting %q is outside the supported range: %v", id, value)
				continue
			}
			spec.apply(value, cfg)
		}
	}
	return diagnostics
}

func settingSpecFor(specs []settingSpec, id string) (settingSpec, bool) {
	for _, spec := range specs {
		if spec.id == id {
			return spec, true
		}
	}
	return settingSpec{}, false
}
