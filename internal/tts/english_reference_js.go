//go:build js

package tts

import (
	"errors"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// Web版はKokoroとWavLMのモデルを持たないため、参照照合をしない。
var errReferenceUnavailable = errors.New("reference synthesis is not available in the web version")

func synthesizeReference(Config, *voicebank.Bank, string, string, string, []frontend.Mora) (*Result, error) {
	return nil, errReferenceUnavailable
}

func predictReference(Config, string, string, []frontend.Mora) (*ProsodyPreview, error) {
	return nil, errReferenceUnavailable
}

func PrepareReferenceIndex(string, *voicebank.Bank, string) {}

func ReferenceIndexProgress() struct{} { return struct{}{} }
