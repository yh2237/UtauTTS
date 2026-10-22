package refspeech

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// KokoroSyllableProviderは、音節を1モーラにして読む言語（スペイン語、ポルトガル語）の参照TTS。
// 英語と同じKokoro-82Mのモデルを、言語ごとの声で読む。
type KokoroSyllableProvider struct {
	kokoroVoice
	language string
}

// syllableVoicesは、言語ごとの参照TTSのID。
var syllableVoices = map[string]string{
	frontend.LanguageSpanish:    kokoroSpanish,
	frontend.LanguagePortuguese: kokoroPortuguese,
}

// NewKokoroSyllableProviderは、音節で読む言語の参照TTSを作る。
func NewKokoroSyllableProvider(service *Service, language string) *KokoroSyllableProvider {
	return &KokoroSyllableProvider{kokoroVoice: kokoroVoice{service: service, id: syllableVoices[language]}, language: language}
}

func (p *KokoroSyllableProvider) Language() string { return p.language }

// Piecesは、読み（「ˈo.la , kˈe tˈal ?」）を音節ごとの断片にする。
// 断片のPartはモーラの番号で、frontend.ParseSyllableLanguageのモーラと同じ並びになる。
// 語の間の空白と文末の記号はPartを-1にする。
func (p *KokoroSyllableProvider) Pieces(text, reading string) ([]Piece, error) {
	fields := strings.Fields(reading)
	if len(fields) == 0 {
		return nil, fmt.Errorf("%s reference synthesis needs a reading", p.language)
	}
	last := len(fields) - 1
	for last >= 0 && frontend.IsSyllablePause(fields[last]) {
		last--
	}
	var pieces []Piece
	part := 0
	pausePending := false
	for i, field := range fields {
		if frontend.IsSyllablePause(field) {
			if i > last {
				pieces = append(pieces, Piece{Symbols: field, Part: -1})
				continue
			}
			// frontend.ParseSyllableLanguageと同じく、先頭と続いた記号は休止のモーラにしない。
			if part > 0 && !pausePending {
				pieces = append(pieces, Piece{Symbols: field, Part: part})
				part++
				pausePending = true
			}
			continue
		}
		if len(pieces) > 0 {
			pieces = append(pieces, Piece{Symbols: " ", Part: -1})
		}
		for _, syllable := range strings.Split(field, ".") {
			pieces = append(pieces, Piece{Symbols: syllable, Part: part})
			part++
		}
		pausePending = false
	}
	if part == 0 {
		return nil, fmt.Errorf("%s reference synthesis needs a reading", p.language)
	}
	return pieces, nil
}
