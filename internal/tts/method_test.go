package tts

import (
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// TestValidateMethodは、方式の指定を確かめる。
func TestValidateMethod(t *testing.T) {
	for _, method := range []string{MethodWaveformSynthesis, MethodReferenceMatching} {
		if err := ValidateMethod(method); err != nil {
			t.Fatalf("ValidateMethod(%q) = %v", method, err)
		}
	}
	for _, method := range []string{"", "native", "reference", "test-concat"} {
		if err := ValidateMethod(method); err == nil {
			t.Fatalf("ValidateMethod(%q) accepted an unsupported method", method)
		}
	}
}

func TestMethodCapabilityGating(t *testing.T) {
	for _, cfg := range []Config{
		{Method: MethodWaveformSynthesis, Language: linguistics.LanguageChinese, Text: "你好"},
		{Method: MethodWaveformSynthesis, Language: linguistics.LanguageSpanish, Reading: "ˈo.la"},
		{Method: MethodWaveformSynthesis, Language: linguistics.LanguagePortuguese, Reading: "ˈo.lɐ"},
	} {
		if _, err := PredictProsody(cfg); err == nil || !strings.Contains(err.Error(), "does not support method") {
			t.Errorf("PredictProsody(%+v) = %v, want waveform capability error", cfg, err)
		}
	}

	if _, err := Synthesize(Config{
		Method:    MethodWaveformSynthesis,
		Language:  linguistics.LanguageChinese,
		Text:      "你好",
		Voicebank: &voicebank.Bank{Root: t.TempDir()},
	}); err == nil || !strings.Contains(err.Error(), "does not support method") {
		t.Fatalf("Synthesize zh waveform error = %v", err)
	}

	preview, err := PredictProsody(Config{
		Method:     MethodWaveformSynthesis,
		Language:   linguistics.LanguageEnglish,
		Phonemizer: linguistics.PhonemizerEnglishDelta,
		Reading:    "HH AH0 L OW1",
	})
	if err != nil {
		t.Fatalf("PredictProsody en waveform = %v", err)
	}
	if len(preview.Morae) != 2 {
		t.Fatalf("en morae = %d, want 2", len(preview.Morae))
	}
}
