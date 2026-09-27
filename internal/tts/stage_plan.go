package tts

import (
	"fmt"
	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/voicebank"
)

// 原音選択後に長さを確定し、最後にユーザーのunit指定を適用する。
func buildSynthesisPlan(cfg Config, profile languageProfile, bank *voicebank.Bank, reading, language, phonemizer string, morae []frontend.Mora, selections []voicebank.Selection, predictions []prosody.Prediction, requestedAliasPolicy voicebank.AliasPolicy, joinModel *connection.JoinModel) (*plan.Plan, bool, error) {
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
		}
		for i, factor := range cfg.PitchFactors {
			if factor <= 0 {
				return nil, false, fmt.Errorf("pitch factors: value %d is %.4f, want positive", i, factor)
			}
			predictions[i].PitchFactor = factor
		}
	}
	// C3aは日本語のモーラだけを対象にし、長いモーラ長で効果があるときだけ有効化する。
	stretchAdapt := stretchAdaptEnabled(cfg) && profile.SupportsStretchAdapt()
	synthesisPlan, err := plan.Build(bank, reading, morae, selections, plan.Config{
		SpeechTiming:         cfg.SpeechTiming,
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
	if cfg.SpeechModel != nil {
		synthesisPlan.SpeechModelID = cfg.SpeechModel.ID
	}
	synthesisPlan.WordBoundaryEnvelope = cfg.WordBoundaryEnvelope
	synthesisPlan.Text = cfg.Text
	synthesisPlan.Language = language
	synthesisPlan.Phonemizer = phonemizer
	synthesisPlan.RequestedAliasPolicy = string(requestedAliasPolicy)
	if joinModel != nil {
		synthesisPlan.JoinCostMode = "learned"
		synthesisPlan.JoinModelID = joinModel.ID
	}
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	return synthesisPlan, stretchAdapt, nil
}
