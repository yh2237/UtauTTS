package plan

import "utautts/internal/frontend"

// B5: ポーズ長の文脈化。句読点の種類と発話末かどうかで休止長を変える。
// 補正は控えめにし、強度で中立1.0から調整できるようにする。
const (
	// pauseContextMinFactorとpauseContextMaxFactorは補正後の倍率のクランプ範囲。
	pauseContextMinFactor = 0.4
	pauseContextMaxFactor = 2.5
	// pauseContextMaxStrengthは補正強度の上限。
	pauseContextMaxStrength = 2.0
	// pauseContextFinalFactorは発話末ポーズへ掛ける追加倍率。
	pauseContextFinalFactor = 1.1
	// 句読点種別ごとの基準倍率。中立は1.0。
	pauseContextCommaFactor    = 0.7
	pauseContextPeriodFactor   = 1.0
	pauseContextQuestionFactor = 1.15
	pauseContextEllipsisFactor = 1.3
	pauseContextNeutralFactor  = 1.0
)

// pauseContextFactorはポーズ長へ掛ける文脈倍率を返す。無効時や負の強度は恒等。
func pauseContextFactor(morae []frontend.Mora, position int, cfg Config) float64 {
	if !cfg.PauseContext {
		return 1
	}
	strength := cfg.PauseContextStrength
	if strength == 0 {
		strength = 1
	}
	if strength < 0 {
		return 1
	}
	if strength > pauseContextMaxStrength {
		strength = pauseContextMaxStrength
	}
	kind := frontend.PauseKindOther
	if position >= 0 && position < len(morae) {
		kind = morae[position].PauseKind
	}
	factor := pauseKindFactor(kind)
	if isUtteranceFinalPause(morae, position) {
		factor *= pauseContextFinalFactor
	}
	// 中立1.0からの偏差に強度を掛ける。
	factor = 1 + (factor-1)*strength
	return clampPauseContextFactor(factor)
}

// pauseKindFactorはポーズ種別の基準倍率を返す。
func pauseKindFactor(kind string) float64 {
	switch kind {
	case frontend.PauseKindComma:
		return pauseContextCommaFactor
	case frontend.PauseKindPeriod:
		return pauseContextPeriodFactor
	case frontend.PauseKindQuestion:
		return pauseContextQuestionFactor
	case frontend.PauseKindEllipsis:
		return pauseContextEllipsisFactor
	default:
		return pauseContextNeutralFactor
	}
}

// isUtteranceFinalPauseは後続に発話モーラが無いポーズかを返す。
func isUtteranceFinalPause(morae []frontend.Mora, position int) bool {
	for index := position + 1; index < len(morae); index++ {
		if !morae[index].Pause {
			return false
		}
	}
	return true
}

func clampPauseContextFactor(factor float64) float64 {
	if factor < pauseContextMinFactor {
		return pauseContextMinFactor
	}
	if factor > pauseContextMaxFactor {
		return pauseContextMaxFactor
	}
	return factor
}
