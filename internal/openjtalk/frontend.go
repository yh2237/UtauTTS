package openjtalk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"utautts/internal/prosody"
)

type Config struct {
	HelperPath     string
	DictionaryPath string
}

type Analysis struct {
	Version  int                    `json:"version"`
	Reading  string                 `json:"reading"`
	Morae    []string               `json:"morae"`
	Features []prosody.FeatureFrame `json:"features"`
}

func Analyze(text string, cfg Config) (*Analysis, error) {
	return AnalyzeContext(context.Background(), text, cfg)
}

const helperTimeout = 2 * time.Minute

func AnalyzeContext(ctx context.Context, text string, cfg Config) (*Analysis, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("Open JTalk frontend canceled: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, helperTimeout)
	defer cancel()
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("Open JTalk input is empty")
	}
	result, err := runFrontend(ctx, text, cfg)
	if err != nil {
		return nil, fmt.Errorf("Open JTalk frontend failed (text=%q): %w", previewText(text), err)
	}
	if result == nil || result.Version != 1 || result.Reading == "" || len(result.Morae) == 0 || len(result.Features) != len(result.Morae) {
		return nil, fmt.Errorf("invalid Open JTalk response")
	}
	return result, nil
}

func previewText(text string) string {
	const maxRunes = 120
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}
