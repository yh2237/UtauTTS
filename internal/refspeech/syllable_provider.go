package refspeech

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// KokoroSyllableProviderは、音節を1モーラにして読む言語（スペイン語、ポルトガル語）の参照TTS。
// 英語と同じKokoro-82Mのモデルを、言語ごとの声で読む。
type KokoroSyllableProvider struct {
	kokoroVoice
	language string
}

// syllableVoicesは、言語ごとの参照TTSのID。
var syllableVoices = map[string]string{
	linguistics.LanguageSpanish:    kokoroSpanish,
	linguistics.LanguagePortuguese: kokoroPortuguese,
}

// NewKokoroSyllableProviderは、音節で読む言語の参照TTSを作る。
func NewKokoroSyllableProvider(service *Service, language string) *KokoroSyllableProvider {
	return &KokoroSyllableProvider{kokoroVoice: kokoroVoice{service: service, id: syllableVoices[language]}, language: language}
}

func (p *KokoroSyllableProvider) Language() string { return p.language }

// Piecesは、読み（「ˈo.la , kˈe tˈal ?」）を音節ごとの断片にする。
// 断片のPartはモーラの番号で、linguistics.ParseSyllableReadingのモーラと同じ並びになる。
// 語の間の空白と文末の記号はPartを-1にする。
func (p *KokoroSyllableProvider) Pieces(utterance linguistics.Utterance) ([]Piece, error) {
	_, parsed, err := linguistics.ParseSyllableReading(utterance.EffectiveReading(), p.language)
	if err != nil {
		return nil, fmt.Errorf("%s reference synthesis needs a reading: %w", p.language, err)
	}
	pieces := make([]Piece, len(parsed))
	for i, piece := range parsed {
		pieces[i] = Piece{Symbols: piece.Symbols, Part: piece.Part}
	}
	return pieces, nil
}

func (p *KokoroSyllableProvider) AlignMoraTokens(pieces []Piece, timing Timing, utterance linguistics.Utterance) [][]int {
	return partMoraTokens(pieces, timing, utterance.Morae)
}
