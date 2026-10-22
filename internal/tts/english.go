package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// parseEnglishPronunciationは、発音形式ごとに英語の読みとモーラを返す。
func parseEnglishPronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	switch phonemizer {
	case frontend.PhonemizerEnglish:
		return frontend.ParseEnglishARPAsing(cfg.Text, cfg.Reading, cfg.Dictionary)
	case frontend.PhonemizerEnglishDelta:
		var presamp frontend.PresampConfig
		if cfg.Voicebank != nil {
			presamp = cfg.Voicebank.Presamp.FrontendConfig()
		}
		return frontend.ParseEnglishDeltaWithConfig(cfg.Text, cfg.Reading, cfg.Dictionary, presamp)
	case frontend.PhonemizerEnglishVCCV:
		return frontend.ParseEnglishVCCV(cfg.Text, cfg.Reading, cfg.Dictionary)
	case frontend.PhonemizerEnglishCV:
		return frontend.ParseEnglishCV(cfg.Text, cfg.Reading, cfg.Dictionary)
	default:
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageEnglish)
	}
}
