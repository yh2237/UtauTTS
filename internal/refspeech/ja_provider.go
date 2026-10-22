package refspeech

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// KokoroJapaneseProviderは日本語の参照TTS（Kokoro-82Mのjf_alpha）。英語と同じモデルで読む。
type KokoroJapaneseProvider struct {
	service *Service
	// wordStartは、読みのモーラごとの語の頭（OpenJTalkの解析から）。nilなら語の区切りを置かない。
	wordStart []bool
}

// NewKokoroJapaneseProviderは日本語の参照TTSを作る。
func NewKokoroJapaneseProvider(service *Service, wordStart []bool) *KokoroJapaneseProvider {
	return &KokoroJapaneseProvider{service: service, wordStart: wordStart}
}

func (p *KokoroJapaneseProvider) ID() string { return kokoroJapanese }

func (p *KokoroJapaneseProvider) Language() string { return frontend.LanguageJapanese }

// Piecesは、かなの読みをモーラごとの断片にする。断片のPartはモーラの番号（休止のモーラを含む）。
// 語の間の空白はPartを-1にする。
func (p *KokoroJapaneseProvider) Pieces(text, reading string) ([]Piece, error) {
	morae, err := frontend.ParseKana(reading)
	if err != nil {
		return nil, err
	}
	symbols, spaceBefore := frontend.KokoroJapaneseSymbols(morae, p.wordStart)
	pieces := make([]Piece, 0, len(morae)*2)
	for i, symbol := range symbols {
		if strings.TrimSpace(symbol) == "" {
			return nil, fmt.Errorf("cannot read Japanese mora %q", morae[i].Text)
		}
		if spaceBefore[i] {
			pieces = append(pieces, Piece{Symbols: " ", Part: -1})
		}
		pieces = append(pieces, Piece{Symbols: symbol, Part: i})
	}
	if len(pieces) == 0 {
		return nil, fmt.Errorf("Japanese reference synthesis needs a reading")
	}
	return pieces, nil
}

func (p *KokoroJapaneseProvider) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.Read(p.ID(), pieces, speed, durations)
}

func (p *KokoroJapaneseProvider) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.ReadProsody(p.ID(), pieces, speed, durations)
}
