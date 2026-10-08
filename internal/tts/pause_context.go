package tts

import "utautts/internal/settings"

func pauseContextEnabled(cfg Config) bool {
	if cfg.PauseContext == nil {
		return settings.Bool("pause_context")
	}
	return *cfg.PauseContext
}

func pauseContextStrength(cfg Config) float64 {
	if cfg.PauseContextStrength == 0 {
		return settings.Number("pause_context_strength")
	}
	return cfg.PauseContextStrength
}
