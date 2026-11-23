package phonemizer

import (
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

type japanesePhonemizer struct{}

func init() {
	Register(japanesePhonemizer{})
}

func (japanesePhonemizer) ID() string       { return linguistics.PhonemizerJapanese }
func (japanesePhonemizer) Language() string { return linguistics.LanguageJapanese }

func (japanesePhonemizer) Aliases(_ linguistics.Utterance, _ voicebank.PresampConfig) ([]voicebank.AliasHints, error) {
	// 日本語はvoicebank側のVCV/CVVC/CV標準規則だけで解決できるため追加ヒントは不要。
	return nil, nil
}
