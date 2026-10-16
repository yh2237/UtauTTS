package tts

import "testing"

func TestEnglishStressAccentHasAContinuousRiseAndFall(t *testing.T) {
	if englishStressAccent(1, 0) >= 0 || englishStressAccent(1, 0.5) <= 40 || englishStressAccent(1, 1) > 1e-9 {
		t.Fatalf("accent contour is not rise-fall: start=%.2f middle=%.2f end=%.2f", englishStressAccent(1, 0), englishStressAccent(1, 0.5), englishStressAccent(1, 1))
	}
	if englishStressAccent(2, 0.5) >= englishStressAccent(1, 0.5) {
		t.Fatal("secondary stress is not weaker than primary stress")
	}
}
