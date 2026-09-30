package tts

func pauseContextEnabled(cfg Config) bool {
	return cfg.PauseContext == nil || *cfg.PauseContext
}

func pauseContextStrength(cfg Config) float64 {
	if cfg.PauseContextStrength == 0 {
		return 1
	}
	return cfg.PauseContextStrength
}
