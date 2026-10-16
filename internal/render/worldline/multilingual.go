package worldline

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
)

func multilingualScore(p *plan.Plan) bool {
	language := frontend.NormalizeLanguage(p.Language)
	return p.PhoneTimingSource == "multilingual-speech-score-v1" && language == frontend.LanguageChinese
}
