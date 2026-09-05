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
