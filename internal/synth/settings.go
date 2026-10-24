package synth

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/tts"
)

// settingTargetは、設定の値を読む型付きの入力と、入れる先の合成の設定。
// IDと型・既定値・範囲はinternal/settingsの表で決める。表のIDはすべてここにもある。
type settingTarget struct {
	typed func(request Request) any
	apply func(value any, cfg *tts.Config)
}

var settingTargets = map[string]settingTarget{
	"mora_duration_ms": number(func(r Request) float64 { return r.MoraDurationMS },
		func(value float64, cfg *tts.Config) { cfg.MoraDurationMS = value }),
	"pause_duration_ms": number(func(r Request) float64 { return r.PauseDurationMS },
		func(value float64, cfg *tts.Config) { cfg.PauseDurationMS = value }),
	"leading_preutterance_ms": number(func(r Request) float64 { return r.LeadingPreutteranceMS },
		func(value float64, cfg *tts.Config) { cfg.LeadingPreutteranceMS = value }),
	"intonation_strength": number(func(r Request) float64 { return r.IntonationStrength },
		func(value float64, cfg *tts.Config) { cfg.IntonationStrength = value }),
	"context_duration": boolean(func(r Request) bool { return r.ContextDuration },
		func(value bool, cfg *tts.Config) { cfg.ContextDuration = &value }),
	"context_duration_strength": number(func(r Request) float64 { return r.ContextDurationStrength },
		func(value float64, cfg *tts.Config) { cfg.ContextDurationStrength = value }),
	"boundary_tone": boolean(func(r Request) bool { return r.BoundaryTone },
		func(value bool, cfg *tts.Config) { cfg.BoundaryTone = &value }),
	"boundary_tone_strength": number(func(r Request) float64 { return r.BoundaryToneStrength },
		func(value float64, cfg *tts.Config) { cfg.BoundaryToneStrength = value }),
	"stretch_adapt": boolean(func(r Request) bool { return r.StretchAdapt },
		func(value bool, cfg *tts.Config) { cfg.StretchAdapt = &value }),
	"stretch_adapt_strength": number(func(r Request) float64 { return r.StretchAdaptStrength },
		func(value float64, cfg *tts.Config) { cfg.StretchAdaptStrength = value }),
	"pause_context": boolean(func(r Request) bool { return r.PauseContext },
		func(value bool, cfg *tts.Config) { cfg.PauseContext = &value }),
	"pause_context_strength": number(func(r Request) float64 { return r.PauseContextStrength },
		func(value float64, cfg *tts.Config) { cfg.PauseContextStrength = value }),
	"timing_warp": boolean(func(r Request) bool { return r.World.TimingWarpEnabled() },
		func(value bool, cfg *tts.Config) { cfg.World.TimingWarp = &value }),
	"microprosody": boolean(func(r Request) bool { return r.World.MicroprosodyEnabled() },
		func(value bool, cfg *tts.Config) { cfg.World.Microprosody = &value }),
}

func number(typed func(Request) float64, apply func(float64, *tts.Config)) settingTarget {
	return settingTarget{
		typed: func(request Request) any { return typed(request) },
		apply: func(value any, cfg *tts.Config) { apply(value.(float64), cfg) },
	}
}

func boolean(typed func(Request) bool, apply func(bool, *tts.Config)) settingTarget {
	return settingTarget{
		typed: func(request Request) any { return typed(request) },
		apply: func(value any, cfg *tts.Config) { apply(value.(bool), cfg) },
	}
}

// 優先順はsettings、型付き設定。使わなかった値は診断で返す。
func resolveSettings(request Request, cfg *tts.Config) []render.RenderDiagnostic {
	for _, setting := range settings.Table {
		target := settingTargets[setting.ID]
		target.apply(target.typed(request), cfg)
	}
	return applySettingsMap(request.Settings, cfg)
}

// 未知のID、型の合わない値、範囲外の値は使わず、診断を返す。
func applySettingsMap(values map[string]json.RawMessage, cfg *tts.Config) []render.RenderDiagnostic {
	var diagnostics []render.RenderDiagnostic
	ignore := func(message string) {
		diagnostics = append(diagnostics, render.RenderDiagnostic{Severity: "warning", Code: "setting-ignored", Message: message})
	}
	for _, id := range slices.Sorted(maps.Keys(values)) {
		setting, known := settings.Lookup(id)
		if !known {
			ignore(fmt.Sprintf("unknown setting %q was ignored", id))
			continue
		}
		value, err := setting.Decode(values[id])
		if err != nil {
			ignore(err.Error())
			continue
		}
		settingTargets[id].apply(value, cfg)
	}
	return diagnostics
}
