package refspeech

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// Providerは参照TTS。言語ごとにテキストを音素列にし、その読みを返す。
// 参照モデルを差し替えると、音素列の作り方と読みの声が変わる。
type Provider interface {
	// IDは参照モデルの名前。
	ID() string
	// Languageは参照モデルが読む言語。
	Language() string
	// Piecesはテキストを音素列にする。
	Pieces(text, reading string) ([]Piece, error)
	// Readは音素列を読み、参照の音声とF0を返す。
	Read(pieces []Piece, speed float32, durations []int) (Reference, error)
	// ReadProsodyは波形を作らず、長さとF0だけを返す。
	ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error)
}

// KokoroProviderは英語の参照TTS（Kokoro-82M）。
type KokoroProvider struct {
	service    *Service
	lexicon    *Lexicon
	dictionary map[string]string
}

// NewKokoroProviderは英語の参照TTSを作る。serviceはKokoroの読みと合成に使う。
func NewKokoroProvider(service *Service, lexicon *Lexicon, dictionary map[string]string) *KokoroProvider {
	return &KokoroProvider{service: service, lexicon: lexicon, dictionary: dictionary}
}

func (p *KokoroProvider) ID() string { return "kokoro-en" }

func (p *KokoroProvider) Language() string { return "en" }

// Piecesは英語のテキストをKokoroの音素列にする。
func (p *KokoroProvider) Pieces(text, reading string) ([]Piece, error) {
	automatic, _, err := frontend.ParseEnglishARPAsing(text, "", p.dictionary)
	if err != nil {
		automatic = ""
	}
	return KokoroPieces(p.lexicon, text, reading, automatic, p.dictionary), nil
}

func (p *KokoroProvider) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.Read(pieces, speed, durations)
}

func (p *KokoroProvider) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if p.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return p.service.ReadProsody(pieces, speed, durations)
}
