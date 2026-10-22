package refspeech

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// KokoroSpanishProviderはスペイン語の参照TTS（Kokoro-82Mのef_dora）。
type KokoroSpanishProvider struct {
	service *Service
}

// NewKokoroSpanishProviderはスペイン語の参照TTSを作る。
func NewKokoroSpanishProvider(service *Service) *KokoroSpanishProvider {
	return &KokoroSpanishProvider{service: service}
}

func (p *KokoroSpanishProvider) ID() string { return kokoroSpanish }

func (p *KokoroSpanishProvider) Language() string { return frontend.LanguageSpanish }

// Piecesは、スペイン語の読み（「ˈo.la , kˈe tˈal ?」）を音節ごとの断片にする。
// 断片のPartはモーラの番号で、frontend.ParseSpanishのモーラと同じ並びになる。
// 語の間の空白と文末の記号はPartを-1にする。
func (p *KokoroSpanishProvider) Pieces(text, reading string) ([]Piece, error) {
	fields := strings.Fields(reading)
	if len(fields) == 0 {
		return nil, fmt.Errorf("Spanish reference synthesis needs a reading")
	}
	last := len(fields) - 1
	for last >= 0 && frontend.IsSpanishPause(fields[last]) {
		last--
	}
	var pieces []Piece
	part := 0
	pausePending := false
	for i, field := range fields {
		if frontend.IsSpanishPause(field) {
			if i > last {
				pieces = append(pieces, Piece{Symbols: field, Part: -1})
				continue
			}
			// frontend.ParseSpanishと同じく、先頭と続いた記号は休止のモーラにしない。
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
		return nil, fmt.Errorf("Spanish reference synthesis needs a reading")
	}
	return pieces, nil
}

func (p *KokoroSpanishProvider) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.Read(p.ID(), pieces, speed, durations)
}

func (p *KokoroSpanishProvider) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.ReadProsody(p.ID(), pieces, speed, durations)
}
