//go:build js

package tts

import (
	"errors"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// Web版はKokoroとHuBERTのモデルを持たないため、英語を合成しない。
var errEnglishUnavailable = errors.New("English synthesis is not available in the web version")

func synthesizeEnglishReference(Config, *voicebank.Bank, string, string, string, []frontend.Mora) (*Result, error) {
	return nil, errEnglishUnavailable
}

func predictEnglishReference(Config, string, []frontend.Mora) (*ProsodyPreview, error) {
	return nil, errEnglishUnavailable
}

func PrepareEnglishIndex(string, string, string) {}

func EnglishIndexProgress() struct{} { return struct{}{} }
