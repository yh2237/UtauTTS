package tts

// stretchAdaptMinMoraMSは伸縮の音源適応(C3a)が効果を持ち得る目標モーラ長の下限。
// これより短い設定では伸縮比が上限に届かず適用0ユニットのまま、
// 全モーラのWAV解析コストだけが増えるため有効化しない。
const stretchAdaptMinMoraMS = 200.0

// stretchAdaptEnabledは日本語の伸縮の音源適応(C3a)が有効かを返す。
// 明示falseは常に無効。未指定(nil)は既定ONだが、基準モーラ長または
// 位置別モーラ長の最大値がstretchAdaptMinMoraMS以上になり得るときだけ有効化する。
func stretchAdaptEnabled(cfg Config) bool {
	if cfg.StretchAdapt != nil && !*cfg.StretchAdapt {
		return false
	}
	return stretchAdaptTargetReachesMinMoraMS(cfg)
}

// stretchAdaptTargetReachesMinMoraMSは目標モーラ長がしきい値以上になり得るかを返す。
func stretchAdaptTargetReachesMinMoraMS(cfg Config) bool {
	if cfg.MoraDurationMS >= stretchAdaptMinMoraMS {
		return true
	}
	for _, duration := range cfg.MoraDurationsMS {
		if duration >= stretchAdaptMinMoraMS {
			return true
		}
	}
	return false
}

// stretchAdaptStrengthは伸縮補正の強度を返す。0は既定1.0、負値はそのまま返す。
func stretchAdaptStrength(cfg Config) float64 {
	if cfg.StretchAdaptStrength == 0 {
		return 1
	}
	return cfg.StretchAdaptStrength
}
