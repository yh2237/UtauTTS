// Package g2pは言語別のテキスト正規化・読み変換・音素解析をまとめる。
// 合成方式（波形合成のエイリアスや参照TTS記号）には依存しない。
package g2p

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

type Language string

const (
	Japanese   Language = Language(linguistics.LanguageJapanese)
	English    Language = Language(linguistics.LanguageEnglish)
	Chinese    Language = Language(linguistics.LanguageChinese)
	Spanish    Language = Language(linguistics.LanguageSpanish)
	Portuguese Language = Language(linguistics.LanguagePortuguese)
)

// Inputは言語解析へ渡す入力。ReadingFallbackはKagome等で読めないときの補助解析（Open JTalk等）に使う。
type Input struct {
	Text            string
	Reading         string
	Phonemizer      string
	Dictionary      map[string]string
	ReadingFallback func(ctx context.Context, text string) (string, error)
}

// Analyzerは1つの言語に対するテキスト正規化とG2P解析を提供する。
type Analyzer interface {
	Language() string
	DefaultPhonemizer() string
	Phonemizers() []string
	Normalize(text string) string
	Analyze(ctx context.Context, input Input) (linguistics.Utterance, error)
}

// Registryは登録済みの言語解析器を管理する。
type Registry struct {
	mu        sync.RWMutex
	analyzers map[string]Analyzer
}

// NewRegistryは空のRegistryを返す。
func NewRegistry() *Registry {
	return &Registry{analyzers: map[string]Analyzer{}}
}

var defaultRegistry = NewRegistry()

// Registerは言語解析器をRegistryへ登録する。同じ言語を二重登録するとpanicする。
func (r *Registry) Register(analyzer Analyzer) {
	if analyzer == nil {
		panic("g2p: nil analyzer")
	}
	lang := strings.ToLower(strings.TrimSpace(analyzer.Language()))
	if lang == "" {
		panic("g2p: empty analyzer language")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.analyzers[lang]; exists {
		panic(fmt.Sprintf("g2p: analyzer already registered for language %q", lang))
	}
	r.analyzers[lang] = analyzer
}

// Getは指定言語のAnalyzerを返す。
func (r *Registry) Get(language string) (Analyzer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	analyzer, ok := r.analyzers[strings.ToLower(strings.TrimSpace(language))]
	return analyzer, ok
}

// Languagesは登録済み言語コードをソート済みで返す。
func (r *Registry) Languages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	langs := make([]string, 0, len(r.analyzers))
	for lang := range r.analyzers {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// ResolveLanguageは言語と発音形式の組み合わせを検証して正規化する。
func (r *Registry) ResolveLanguage(language, phonemizer string) (string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	phonemizer = strings.ToLower(strings.TrimSpace(phonemizer))

	r.mu.RLock()
	defer r.mu.RUnlock()

	if language == "" {
		if phonemizer != "" {
			for lang, analyzer := range r.analyzers {
				if slices.Contains(analyzer.Phonemizers(), phonemizer) {
					language = lang
					break
				}
			}
		}
		if language == "" {
			language = linguistics.LanguageJapanese
		}
	}
	analyzer, ok := r.analyzers[language]
	if !ok {
		return "", "", fmt.Errorf("unsupported language %q", language)
	}
	if phonemizer == "" {
		phonemizer = analyzer.DefaultPhonemizer()
	}
	if !slices.Contains(analyzer.Phonemizers(), phonemizer) {
		return "", "", fmt.Errorf("phonemizer %q does not support language %q", phonemizer, language)
	}
	return language, phonemizer, nil
}

// Normalizeはその言語の正規化規則で数字・通貨・記号等を語へ広げる。
func (r *Registry) Normalize(language, text string) string {
	analyzer, ok := r.Get(language)
	if !ok {
		return text
	}
	return analyzer.Normalize(text)
}

// Analyzeは登録済みの言語解析器で入力を共通中間表現へ変換する。
func (r *Registry) Analyze(ctx context.Context, language string, input Input) (linguistics.Utterance, error) {
	resolvedLang, resolvedPhonemizer, err := r.ResolveLanguage(language, input.Phonemizer)
	if err != nil {
		return linguistics.Utterance{}, err
	}
	input.Phonemizer = resolvedPhonemizer
	analyzer, _ := r.Get(resolvedLang)
	return analyzer.Analyze(ctx, input)
}

func Register(analyzer Analyzer) { defaultRegistry.Register(analyzer) }
func Get(language string) (Analyzer, bool) {
	return defaultRegistry.Get(language)
}
func Languages() []string { return defaultRegistry.Languages() }
func ResolveLanguage(language, phonemizer string) (string, string, error) {
	return defaultRegistry.ResolveLanguage(language, phonemizer)
}
func Normalize(language Language, text string) string {
	return defaultRegistry.Normalize(string(language), text)
}
func Analyze(ctx context.Context, language string, input Input) (linguistics.Utterance, error) {
	return defaultRegistry.Analyze(ctx, language, input)
}
