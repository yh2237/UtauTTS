package refspeech

import (
	"fmt"
	"strings"
)

// KokoroChineseProviderは中国語の参照TTS（Kokoro-82M-v1.1-zh）。
type KokoroChineseProvider struct {
	service *Service
}

// NewKokoroChineseProviderは中国語の参照TTSを作る。
func NewKokoroChineseProvider(service *Service) *KokoroChineseProvider {
	return &KokoroChineseProvider{service: service}
}

func (p *KokoroChineseProvider) ID() string { return kokoroChinese }

func (p *KokoroChineseProvider) Language() string { return "zh" }

// Piecesは、ピンインの読み（「ni3 hao3 | ...」）を注音の音素列にする。
// 休止は読点にする。1つの音節が1つの断片になる。
func (p *KokoroChineseProvider) Pieces(text, reading string) ([]Piece, error) {
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

func (p *KokoroChineseProvider) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.Read(p.ID(), pieces, speed, durations)
}

func (p *KokoroChineseProvider) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.ReadProsody(p.ID(), pieces, speed, durations)
}
