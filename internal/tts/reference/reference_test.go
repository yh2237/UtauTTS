package reference

import (
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/tts"
)

// TestReferenceProvidersは、モデルの有無で一覧が変わることを確かめる。
func TestReferenceProviders(t *testing.T) {
	if providers := ReferenceProviders(""); providers != nil {
		t.Fatalf("providers for an empty folder: %v", providers)
	}
	if providers := ReferenceProviders(t.TempDir()); providers != nil {
		t.Fatalf("providers for a folder without models: %v", providers)
	}
}

func TestJapaneseReferenceMatchingIsUnavailable(t *testing.T) {
	if _, err := tts.PredictProsody(tts.Config{
		Method:   tts.MethodReferenceMatching,
		Language: linguistics.LanguageJapanese,
		Reading:  "コンニチハ",
	}); err == nil || !strings.Contains(err.Error(), `reference synthesis is not available for "ja"`) {
		t.Fatalf("PredictProsody ja reference error = %v", err)
	}
}
