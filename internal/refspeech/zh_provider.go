package refspeech

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// KokoroChineseProviderは中国語の参照TTS（Kokoro-82M-v1.1-zh）。
type KokoroChineseProvider struct {
	kokoroVoice
}

// NewKokoroChineseProviderは中国語の参照TTSを作る。
func NewKokoroChineseProvider(service *Service) *KokoroChineseProvider {
	return &KokoroChineseProvider{kokoroVoice{service: service, id: kokoroChinese}}
}

func (p *KokoroChineseProvider) Language() string { return linguistics.LanguageChinese }

// Piecesは、ピンインの読み（「ni3 hao3 | ...」）を注音の音素列にする。
// 休止は読点にする。1つの音節が1つの断片になる。
func (p *KokoroChineseProvider) Pieces(utterance linguistics.Utterance) ([]Piece, error) {
	reading := utterance.EffectiveReading()
	if strings.TrimSpace(reading) == "" {
		return nil, fmt.Errorf("Chinese reference synthesis needs a Pinyin reading")
	}
	fields := strings.Fields(reading)
	pieces := make([]Piece, 0, len(fields))
	part := 0
	for _, field := range fields {
		if field == "|" {
			pieces = append(pieces, Piece{Symbols: ",", Part: -1})
			continue
		}
		symbol, ok := pinyinToZhuyin(field)
		if !ok {
			return nil, fmt.Errorf("cannot read Pinyin syllable %q", field)
		}
		pieces = append(pieces, Piece{Symbols: symbol, Part: part})
		part++
	}
	if len(pieces) == 0 {
		return nil, fmt.Errorf("Chinese reference synthesis needs a Pinyin reading")
	}
	return pieces, nil
}

func (p *KokoroChineseProvider) AlignMoraTokens(pieces []Piece, timing Timing, utterance linguistics.Utterance) [][]int {
	return simpleMoraTokens(pieces, timing, utterance.Morae)
}
