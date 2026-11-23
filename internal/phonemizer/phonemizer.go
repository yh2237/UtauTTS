// Package phonemizerは言語解析の共通中間表現からUTAU波形合成用のエイリアス候補を作る。
package phonemizer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// Phonemizerは言語ごとの発音形式からUTAU波形合成のエイリアス候補を生成する。
type Phonemizer interface {
	ID() string
	Language() string
	Aliases(utterance linguistics.Utterance, presamp voicebank.PresampConfig) ([]voicebank.AliasHints, error)
}

var (
	registryMu    sync.RWMutex
	registryByID  = map[string]Phonemizer{}
	defaultByLang = map[string]Phonemizer{}
)

// Registerは波形合成用の発音変換器を登録する。重複IDや空IDではpanicする。
func Register(p Phonemizer) {
	if p == nil {
		panic("phonemizer: nil phonemizer")
	}
	id := strings.ToLower(strings.TrimSpace(p.ID()))
	if id == "" {
		panic("phonemizer: empty id")
	}
	lang := linguistics.NormalizeLanguage(p.Language())
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registryByID[id]; exists {
		panic(fmt.Sprintf("phonemizer: duplicate id %q", id))
	}
	registryByID[id] = p
	if _, exists := defaultByLang[lang]; !exists {
		defaultByLang[lang] = p
	}
}

// Lookupは指定IDの波形合成用発音変換器を返す。
func Lookup(id string) (Phonemizer, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registryByID[strings.ToLower(strings.TrimSpace(id))]
	return p, ok
}

// Resolveは指定IDの波形合成用発音変換器を返し、未登録ならエラーを返す。
func Resolve(id string) (Phonemizer, error) {
	p, ok := Lookup(id)
	if !ok {
		return nil, fmt.Errorf("unsupported waveform phonemizer %q", id)
	}
	return p, nil
}

// DefaultForLanguageは指定言語に対応する既定の波形合成用発音変換器を返す。
func DefaultForLanguage(language string) (Phonemizer, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := defaultByLang[linguistics.NormalizeLanguage(language)]
	return p, ok
}

// SupportsLanguageは指定言語が波形合成用の発音変換器を持つかを返す。
func SupportsLanguage(language string) bool {
	_, ok := DefaultForLanguage(language)
	return ok
}
