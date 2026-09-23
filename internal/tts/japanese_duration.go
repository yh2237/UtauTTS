package tts

import (
	"strings"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

// 日本語のモーラ長を韻律特徴へ控えめに連動させる係数。
// 話し言葉寄りの強い短縮は避け、読み上げとして自然な範囲に留める。
const (
	japaneseContextMinFactor        = 0.8
	japaneseContextMaxFactor        = 1.3
	japaneseAccentPhraseEndFactor   = 1.15 // アクセント句末はやや伸ばす
	japanesePhraseFinalFactor       = 1.2  // 文末・ポーズ前は伸ばす
	japaneseQuestionFinalFactor     = 1.25 // 疑問文の文末はさらに伸ばす
	japaneseParticleFactor          = 0.85 // 助詞・助動詞は短め
	japaneseContentWordStartFactor  = 1.05 // 自立語の語頭をごく僅かに伸ばす
	japaneseContextBoundaryPresence = 0.5  // 0/1特徴の真偽判定しきい値
)

// japaneseContextDurationFactorsは各モーラの長さ倍率を韻律特徴から求める。
// 特徴が無い、またはモーラ数と一致しない場合は恒等（全て1.0）を返す。
func japaneseContextDurationFactors(morae []frontend.Mora, features []prosody.FeatureFrame, question bool) []float64 {
	factors := make([]float64, len(morae))
	for i := range factors {
		factors[i] = 1
	}
	if len(morae) == 0 || len(features) != len(morae) || !hasJapaneseContextFeatures(features) {
		return factors
	}
	for i, mora := range morae {
		if mora.Pause {
			continue
		}
		feature := features[i]
		factor := 1.0
		particle := isParticleLikeFeature(feature)
		if particle {
			factor *= japaneseParticleFactor
		}
		switch {
		case isUtteranceFinalMora(morae, i):
			if question {
				factor *= japaneseQuestionFinalFactor
			} else {
				factor *= japanesePhraseFinalFactor
			}
		case isPhraseFinalMora(morae, i):
			factor *= japanesePhraseFinalFactor
		case feature["accent_phrase_end"] >= japaneseContextBoundaryPresence:
			factor *= japaneseAccentPhraseEndFactor
		}
		if !particle && feature["word_start"] >= japaneseContextBoundaryPresence {
			factor *= japaneseContentWordStartFactor
		}
		factors[i] = clampJapaneseContextFactor(factor)
	}
	return factors
}

// applyJapaneseContextDurationは既存の予測へ文脈係数を乗算する。
// 予測が無い場合は1埋めの配列を用意する。無効時は恒等。強度0は既定1.0、負値は恒等。
func applyJapaneseContextDuration(cfg Config, morae []frontend.Mora, features []prosody.FeatureFrame, predictions []prosody.Prediction, question bool) []prosody.Prediction {
	if !contextDurationEnabled(cfg) {
		return predictions
	}
	strength := contextDurationStrength(cfg)
	if strength <= 0 {
		return predictions
	}
	if len(morae) == 0 {
		return predictions
	}
	if len(predictions) == 0 {
		predictions = make([]prosody.Prediction, len(morae))
		for i := range predictions {
			predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		}
	}
	factors := japaneseContextDurationFactors(morae, features, question)
	for i := range morae {
		if i >= len(predictions) {
			break
		}
		if factors[i] <= 0 {
			continue
		}
		// 強度は中立1.0からの偏差へ掛け、クランプ済みの0.8〜1.3を保つ。
		predictions[i].DurationFactor *= 1 + (factors[i]-1)*strength
	}
	return predictions
}

// contextDurationEnabledはC1が有効かを返す。未指定(nil)は既定ON。
func contextDurationEnabled(cfg Config) bool {
	return cfg.ContextDuration == nil || *cfg.ContextDuration
}

// contextDurationStrengthは適用強度を返す。0は既定1.0、負値はそのまま返す。
func contextDurationStrength(cfg Config) float64 {
	if cfg.ContextDurationStrength == 0 {
		return 1
	}
	return cfg.ContextDurationStrength
}

// hasJapaneseContextFeaturesは長さ制御に使える韻律特徴が含まれるかを返す。
func hasJapaneseContextFeatures(features []prosody.FeatureFrame) bool {
	for _, feature := range features {
		for key, value := range feature {
			if value < japaneseContextBoundaryPresence {
				continue
			}
			if strings.HasPrefix(key, "accent_") || strings.HasPrefix(key, "word_") ||
				strings.HasPrefix(key, "pos=") || strings.HasPrefix(key, "pos_group1=") {
				return true
			}
		}
	}
	return false
}

// isParticleLikeFeatureはpos/pos_group1特徴から助詞・助動詞を判定する。
func isParticleLikeFeature(feature prosody.FeatureFrame) bool {
	for key, value := range feature {
		if value < japaneseContextBoundaryPresence {
			continue
		}
		if pos, ok := strings.CutPrefix(key, "pos="); ok {
			if pos == "助詞" || pos == "助動詞" {
				return true
			}
		}
		if group, ok := strings.CutPrefix(key, "pos_group1="); ok {
			if strings.Contains(group, "助詞") || strings.Contains(group, "助動詞") {
				return true
			}
		}
	}
	return false
}

// isPhraseFinalMoraは文末またはポーズ前のモーラかを返す。
func isPhraseFinalMora(morae []frontend.Mora, index int) bool {
	if index+1 >= len(morae) {
		return true
	}
	return morae[index+1].Pause
}

// isUtteranceFinalMoraは後続がポーズだけの最終発話モーラかを返す。
func isUtteranceFinalMora(morae []frontend.Mora, index int) bool {
	for i := index + 1; i < len(morae); i++ {
		if !morae[i].Pause {
			return false
		}
	}
	return true
}

func clampJapaneseContextFactor(factor float64) float64 {
	if factor < japaneseContextMinFactor {
		return japaneseContextMinFactor
	}
	if factor > japaneseContextMaxFactor {
		return japaneseContextMaxFactor
	}
	return factor
}
