package main

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/openutau"
	"utautts/internal/plan"
	"utautts/internal/tts"
)

// ustxFrameCurvesはUSTX出力用の10msピッチ輪郭を再計算する。
func ustxFrameCurves(cfg tts.Config, count int) []openutau.FrameCurve {
	curves := make([]openutau.FrameCurve, count)
	if cfg.ProsodyModelPath == "" {
		return curves
	}
	predictConfig := cfg
	predictConfig.ApplyPitch = true
	if predictConfig.IntonationStrength <= 0 {
		predictConfig.IntonationStrength = 1
	}
	if predictConfig.Renderer == "" {
		predictConfig.Renderer = "waveform"
	}
	preview, err := tts.PredictProsody(predictConfig)
	if err != nil || preview == nil || preview.FramePitchCurve == nil {
		return curves
	}
	curves[0] = openutau.FrameCurve{
		FrameMS: preview.FramePitchCurve.FrameMS,
		Cents:   preview.FramePitchCurve.Cents,
	}
	return curves
}

// ustxProjectFromSynthesisは合成結果からUSTX出力用の1発話プロジェクトを作る。
func ustxProjectFromSynthesis(cfg tts.Config, p *plan.Plan, voicebankID string) *openutau.UtauTTSProject {
	utterance := openutau.UtauTTSUtterance{
		Text:            cfg.Text,
		VoicebankID:     voicebankID,
		Tone:            p.Tone,
		MoraDurationMS:  cfg.MoraDurationMS,
		PauseDurationMS: cfg.PauseDurationMS,
		ApplyPitch:      cfg.ApplyPitch,
		Intonation:      cfg.IntonationStrength,
		AnalysisCache: openutau.UtauTTSAnalysisCache{
			Reading: p.Reading,
			Morae:   planMorae(p),
		},
	}
	// フレーム輪郭がない場合に使うモーラ単位のピッチ補正。
	cents := make([]float64, len(utterance.AnalysisCache.Morae))
	for _, unit := range p.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		if unit.Position < 0 || unit.Position >= len(cents) {
			continue
		}
		factor := unit.PitchFactor
		if factor <= 0 {
			factor = 1
		}
		cents[unit.Position] = 1200 * math.Log2(factor)
	}
	utterance.AutomaticPitchPoints = cents
	// プロソディ予測後の実際の配置と長さをUSTXへ反映する。
	morae := utterance.AnalysisCache.Morae
	durationsMS := make([]float64, len(morae))
	positionsMS := make([]float64, len(morae))
	for _, unit := range p.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		if unit.Position < 0 || unit.Position >= len(morae) {
			continue
		}
		positionsMS[unit.Position] = unit.NoteStartMS
		if unit.DurationMS > 0 {
			durationsMS[unit.Position] = unit.DurationMS
		}
	}
	utterance.AutomaticMoraDurMS = durationsMS
	utterance.AutomaticMoraPosMS = positionsMS
	return &openutau.UtauTTSProject{
		Format:        "utautts-project",
		FormatVersion: 5,
		Utterances:    []openutau.UtauTTSUtterance{utterance},
	}
}

// planMoraeは計画と同じ解析でモーラ列を復元し、長音の母音も保持する。
func planMorae(p *plan.Plan) []openutau.UtauTTSMora {
	morae, err := frontend.ParseKana(p.Reading)
	if err == nil && len(morae) > 0 {
		result := make([]openutau.UtauTTSMora, len(morae))
		for index, mora := range morae {
			result[index] = openutau.UtauTTSMora{
				Position: index, Mora: mora.Text, Pause: mora.Pause,
				Consonant: mora.Consonant, Vowel: mora.Vowel,
			}
		}
		return result
	}
	// 解析できない場合は計画ユニットから復元する(母音情報はない)。
	var fallback []openutau.UtauTTSMora
	seen := make(map[int]bool)
	for _, unit := range p.Units {
		if seen[unit.Position] {
			continue
		}
		seen[unit.Position] = true
		if unit.Silent {
			fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Pause: true})
			continue
		}
		fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Mora: unit.Mora})
	}
	return fallback
}
