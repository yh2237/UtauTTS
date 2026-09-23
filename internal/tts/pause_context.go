package tts

// pauseContextEnabledはポーズ長の文脈化(B5)が有効かを返す。未指定(nil)は既定ON。
func pauseContextEnabled(cfg Config) bool {
	return cfg.PauseContext == nil || *cfg.PauseContext
}

// pauseContextStrengthはポーズ長補正の強度を返す。0は既定1.0、負値はそのまま返す。
func pauseContextStrength(cfg Config) float64 {
	if cfg.PauseContextStrength == 0 {
		return 1
	}
	return cfg.PauseContextStrength
}
