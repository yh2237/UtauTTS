package refspeech

import (
	"fmt"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/g2p/en"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// Providerは参照TTS。言語ごとに発話中間表現を音素列にし、その読みとモーラ対応を返す。
// 参照モデルを差し替えると、音素列の作り方と読みの声が変わる。
type Provider interface {
	ID() string
	Language() string
	Pieces(utterance linguistics.Utterance) ([]Piece, error)
	// Readは音素列を読み、参照の音声とF0を返す。
	Read(pieces []Piece, speed float32, durations []int) (Reference, error)
	// ReadProsodyは波形を作らず、長さとF0だけを返す。
	ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error)
	// AlignMoraTokensは参照TTSの入力記号列をモーラごとの番号へ割り当てる。
	AlignMoraTokens(pieces []Piece, timing Timing, utterance linguistics.Utterance) [][]int
}

// ProviderConfigは言語別Providerを生成するときの依存をまとめる。
type ProviderConfig struct {
	Service     *Service
	Lexicon     *Lexicon
	LoadLexicon func() (*Lexicon, error)
	Dictionary  map[string]string
}

// ProviderFactoryは言語に対応するProviderを作る。
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

var (
	providerMu       sync.RWMutex
	providerRegistry = map[string]ProviderFactory{}
)

// RegisterProviderは言語に対応する参照TTSファクトリを登録する。
func RegisterProvider(language string, factory ProviderFactory) {
	if strings.TrimSpace(language) == "" {
		panic("refspeech: empty provider language")
	}
	if factory == nil {
		panic("refspeech: nil provider factory")
	}
	normalized := linguistics.NormalizeLanguage(language)
	providerMu.Lock()
	defer providerMu.Unlock()
	if _, exists := providerRegistry[normalized]; exists {
		panic(fmt.Sprintf("refspeech: provider already registered for language %q", normalized))
	}
	providerRegistry[normalized] = factory
}

// SupportsProviderLanguageは指定言語の参照TTSが登録されているかを返す。
func SupportsProviderLanguage(language string) bool {
	if strings.TrimSpace(language) == "" {
		return false
	}
	providerMu.RLock()
	defer providerMu.RUnlock()
	_, ok := providerRegistry[linguistics.NormalizeLanguage(language)]
	return ok
}

// NewProviderは指定言語に対応する参照TTSを生成する。
func NewProvider(language string, cfg ProviderConfig) (Provider, error) {
	if strings.TrimSpace(language) == "" {
		return nil, fmt.Errorf("reference synthesis is not available for %q", language)
	}
	normalized := linguistics.NormalizeLanguage(language)
	providerMu.RLock()
	factory, ok := providerRegistry[normalized]
	providerMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("reference synthesis is not available for %q", language)
	}
	return factory(cfg)
}

func init() {
	RegisterProvider(linguistics.LanguageEnglish, func(cfg ProviderConfig) (Provider, error) {
		lexicon := cfg.Lexicon
		if lexicon == nil && cfg.LoadLexicon != nil {
			var err error
			lexicon, err = cfg.LoadLexicon()
			if err != nil {
				return nil, err
			}
		}
		return NewKokoroProvider(cfg.Service, lexicon, cfg.Dictionary), nil
	})
	RegisterProvider(linguistics.LanguageChinese, func(cfg ProviderConfig) (Provider, error) {
		return NewKokoroChineseProvider(cfg.Service), nil
	})
	RegisterProvider(linguistics.LanguageSpanish, func(cfg ProviderConfig) (Provider, error) {
		return NewKokoroSyllableProvider(cfg.Service, linguistics.LanguageSpanish), nil
	})
	RegisterProvider(linguistics.LanguagePortuguese, func(cfg ProviderConfig) (Provider, error) {
		return NewKokoroSyllableProvider(cfg.Service, linguistics.LanguagePortuguese), nil
	})
}

// kokoroVoiceは、参照TTSのIDに対応するKokoroの声で音素列を読む。言語ごとの参照TTSが埋め込む。
type kokoroVoice struct {
	service *Service
	id      string
}

func (v kokoroVoice) ID() string { return v.id }

func (v kokoroVoice) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if v.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return v.service.Read(v.id, pieces, speed, durations)
}

func (v kokoroVoice) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	if v.service == nil {
		return Reference{}, fmt.Errorf("kokoro provider has no service")
	}
	return v.service.ReadProsody(v.id, pieces, speed, durations)
}

// KokoroProviderは英語の参照TTS（Kokoro-82M）。
type KokoroProvider struct {
	kokoroVoice
	lexicon    *Lexicon
	dictionary map[string]string
}

// NewKokoroProviderは英語の参照TTSを作る。serviceはKokoroの読みと合成に使う。
func NewKokoroProvider(service *Service, lexicon *Lexicon, dictionary map[string]string) *KokoroProvider {
	return &KokoroProvider{kokoroVoice: kokoroVoice{service: service, id: kokoroEnglish}, lexicon: lexicon, dictionary: dictionary}
}

func (p *KokoroProvider) Language() string { return linguistics.LanguageEnglish }

func (p *KokoroProvider) Pieces(utterance linguistics.Utterance) ([]Piece, error) {
	automatic := utterance.Automatic
	if automatic == "" && strings.TrimSpace(utterance.Text) != "" {
		if parsed, err := en.Parse(linguistics.PhonemizerEnglish, utterance.Text, "", p.dictionary); err == nil {
			automatic = parsed.Reading
		}
	}
	return KokoroPieces(p.lexicon, utterance.Text, utterance.EffectiveReading(), automatic, p.dictionary), nil
}

func (p *KokoroProvider) AlignMoraTokens(pieces []Piece, timing Timing, utterance linguistics.Utterance) [][]int {
	return englishMoraTokens(pieces, timing, utterance.EffectiveReading(), utterance.Morae)
}
