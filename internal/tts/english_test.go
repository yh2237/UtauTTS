package tts

import "testing"

func TestEnglishPreviewUsesStressAndRespectsOverrides(t *testing.T) {
	cfg := Config{Language: "en", Reading: "AE0 | AE1", MoraDurationMS: 100, ApplyPitch: true, IntonationStrength: 1}
	preview, err := PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if preview.MoraDurationsMS[1] <= preview.MoraDurationsMS[0] || preview.FramePitchCurve == nil {
		t.Fatalf("preview=%+v", preview)
	}
	cfg.MoraDurationsMS = []float64{80, 90}
	cfg.ApplyPitch = false
	preview, err = PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if preview.MoraDurationsMS[0] != 80 || preview.MoraDurationsMS[1] != 90 || preview.FramePitchCurve != nil {
		t.Fatalf("overrides=%+v", preview)
	}
}

func TestEnglishStressAccentHasAContinuousRiseAndFall(t *testing.T) {
	if englishStressAccent(1, 0) >= 0 || englishStressAccent(1, 0.5) <= 40 || englishStressAccent(1, 1) > 1e-9 {
		t.Fatalf("accent contour is not rise-fall: start=%.2f middle=%.2f end=%.2f", englishStressAccent(1, 0), englishStressAccent(1, 0.5), englishStressAccent(1, 1))
	}
	if englishStressAccent(2, 0.5) >= englishStressAccent(1, 0.5) {
		t.Fatal("secondary stress is not weaker than primary stress")
	}
}
