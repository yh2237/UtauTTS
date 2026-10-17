package tts

import (
	"fmt"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func buildSynthesisPlan(cfg Config, profile languageProfile, bank *voicebank.Bank, reading, language, phonemizer string, morae []frontend.Mora, selections []voicebank.Selection, predictions []prosody.Prediction, requestedAliasPolicy voicebank.AliasPolicy) (*plan.Plan, bool, error) {
	phoneWeights, phoneTimingSource := profile.PhoneTiming(cfg, morae, voicebank.IsSingleCVSelections(selections))
	if len(cfg.PitchFactors) > 0 {
		if len(cfg.PitchFactors) != len(morae) {
			return nil, false, fmt.Errorf("pitch factors: got %d values for %d morae", len(cfg.PitchFactors), len(morae))
		}
		if len(predictions) == 0 {
			predictions = make([]prosody.Prediction, len(morae))
			for i := range predictions {
				predictions[i].DurationFactor = 1
				predictions[i].EnergyFactor = 1
			}
		} else {
			// キャッシュ内の自動予測へ手動値を残さない。
			predictions = append([]prosody.Prediction(nil), predictions...)
		}
		for i, factor := range cfg.PitchFactors {
			if factor <= 0 {
				return nil, false, fmt.Errorf("pitch factors: value %d is %.4f, want positive", i, factor)
			}
			predictions[i].PitchFactor = factor
		}
	}
	// 短いモーラでは不要な原音解析を避ける。
	stretchAdapt := stretchAdaptEnabled(cfg) && profile.SupportsStretchAdapt()
	synthesisPlan, err := plan.Build(bank, reading, morae, selections, plan.Config{
		PauseDurationSet:     cfg.PauseDurationSet,
		MoraDurationMS:       cfg.MoraDurationMS,
		PauseDurationMS:      cfg.PauseDurationMS,
		PauseContext:         pauseContextEnabled(cfg),
		PauseContextStrength: pauseContextStrength(cfg),
		MoraDurationsMS:      cfg.MoraDurationsMS,
		PhoneWeights:         phoneWeights,
		PhoneWeightsSource:   phoneTimingSource,
		Predictions:          predictions,
		AliasPolicy:          cfg.AliasPolicy,
		Tone:                 cfg.Tone,
		Color:                cfg.Color,
		StretchAdapt:         stretchAdapt,
	})
	if err != nil {
		return nil, false, fmt.Errorf("build synthesis plan: %w", err)
	}
	if err := plan.ApplyUnitOverrides(synthesisPlan.Units, cfg.UnitOverrides); err != nil {
		return nil, false, fmt.Errorf("apply unit overrides: %w", err)
	}
	synthesisPlan.Text = cfg.Text
	synthesisPlan.Language = language
	synthesisPlan.Phonemizer = phonemizer
	synthesisPlan.RequestedAliasPolicy = string(requestedAliasPolicy)
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	return synthesisPlan, stretchAdapt, nil
}
