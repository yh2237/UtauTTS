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

// E1の弱形はConfigの*boolで切り替わり、未指定は既定ON。
func TestEnglishWeakFormConfigControlsReading(t *testing.T) {
	enabled := true
	cfg := Config{Language: "en", Phonemizer: "en-arpasing", Text: "bread and butter", EnglishWeakForm: &enabled}
	preview, err := Analyze(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reading != "B R EH1 D | AH0 N | B AH1 T ER0" {
		t.Fatalf("enabled reading = %s", preview.Reading)
	}
	disabled := false
	cfg.EnglishWeakForm = &disabled
	preview, err = Analyze(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reading != "B R EH1 D | AH0 N D | B AH1 T ER0" {
		t.Fatalf("disabled reading = %s", preview.Reading)
	}
}
