package tts

import "testing"

// TestReferenceProvidersは、モデルの有無で一覧が変わることを確かめる。
func TestReferenceProviders(t *testing.T) {
	if providers := ReferenceProviders(""); providers != nil {
		t.Fatalf("providers for an empty folder: %v", providers)
	}
	if providers := ReferenceProviders(t.TempDir()); providers != nil {
		t.Fatalf("providers for a folder without models: %v", providers)
	}
}
