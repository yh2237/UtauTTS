package worldrender

import (
	"encoding/json"
	"fmt"
	"math"

	"utautts/internal/speechtiming"
)

// timingWarpはWORLD特徴量の時間伸縮の入力（合成計画のモーラ）。
type timingWarp struct {
	Strength        float64
	LeadingMarginMS float64
	Morae           []speechtiming.Mora
}

// decodeTimingWarpは合成計画から、時間伸縮に使うモーラだけを読む。
func decodeTimingWarp(planData []byte, strength float64) (*timingWarp, error) {
	if strength <= 0 {
		return nil, nil
	}
	var synthesisPlan struct {
		LeadingMarginMS float64 `json:"leading_margin_ms"`
		Units           []struct {
			Position                int     `json:"position"`
			Role                    string  `json:"role"`
			Mora                    string  `json:"mora"`
			Silent                  bool    `json:"silent"`
			NoteStartMS             float64 `json:"note_start_ms"`
			DurationMS              float64 `json:"duration_ms"`
			EffectivePreutteranceMS float64 `json:"effective_preutterance_ms"`
		} `json:"units"`
	}
	if err := json.Unmarshal(planData, &synthesisPlan); err != nil {
		return nil, fmt.Errorf("decode timing warp plan: %w", err)
	}
	result := &timingWarp{Strength: strength, LeadingMarginMS: synthesisPlan.LeadingMarginMS}
	// CVVCでは子音がVC（transition）から始まるので、子音の長さはVCの長さまで含める。
	transition := map[int]float64{}
	for _, item := range synthesisPlan.Units {
		if item.Role == "transition" && !item.Silent {
			transition[item.Position] = item.DurationMS
		}
	}
	for _, item := range synthesisPlan.Units {
		if item.Role != "mora" || item.Silent || item.Mora == "" {
			continue
		}
		result.Morae = append(result.Morae, speechtiming.Mora{
			Text: item.Mora, NoteStartMS: item.NoteStartMS, DurationMS: item.DurationMS,
			EffectivePreutteranceMS: math.Max(item.EffectivePreutteranceMS, transition[item.Position]),
		})
	}
	return result, nil
}

// applyTimingWarpは合成直前の特徴量を、学習した読み上げの動きに合わせて時間方向だけ伸縮する。
func applyTimingWarp(warp *timingWarp, sampleRate int, features *worldFeatures) error {
	if warp == nil || warp.Strength <= 0 || len(warp.Morae) == 0 {
		return nil
	}
	model, err := speechtiming.DefaultTarget()
	if err != nil {
		return fmt.Errorf("load timing warp model: %w", err)
	}
	warped, err := speechtiming.Warp(model, warp.Morae, warp.LeadingMarginMS, speechtiming.Features{
		Frames: features.Frames, FFTSize: features.FFTSize, SampleRate: sampleRate,
		F0: features.F0, Spectrum: features.Spectrum, Aperiodicity: features.Aperiodicity,
	}, warp.Strength)
	if err != nil {
		return fmt.Errorf("timing warp: %w", err)
	}
	features.F0, features.Spectrum, features.Aperiodicity = warped.F0, warped.Spectrum, warped.Aperiodicity
	return nil
}
