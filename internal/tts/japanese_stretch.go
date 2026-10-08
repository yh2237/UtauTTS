package tts

import "utautts/internal/settings"

// 短いモーラでは伸縮上限に届かず、WAV解析のコストだけが増えるため補正しない。
const stretchAdaptMinMoraMS = 200.0

func stretchAdaptEnabled(cfg Config) bool {
	if cfg.StretchAdapt == nil && !settings.Bool("stretch_adapt") || cfg.StretchAdapt != nil && !*cfg.StretchAdapt {
		return false
	}
	return stretchAdaptTargetReachesMinMoraMS(cfg)
}

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

func stretchAdaptStrength(cfg Config) float64 {
	if cfg.StretchAdaptStrength == 0 {
		return settings.Number("stretch_adapt_strength")
	}
	return cfg.StretchAdaptStrength
}
