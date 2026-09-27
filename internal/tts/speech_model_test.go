package tts

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

// 数式検証用データ。配布モデルではない。
func speechModelFixture() *prosody.SpeechModel {
	return &prosody.SpeechModel{Version: 1, FeatureVersion: 1, ID: "test-only", Language: "en", Corpus: "unit-test", License: "unit-test", TrainingDataKind: "natural",
		PhoneCounts: map[string]int{"aa": 5}, Duration: map[string]float64{"bias": math.Log(1.5)}}
}

func TestLearnedSpeechScoreCoverageManualDurationAndPreviewCache(t *testing.T) {
	m := []frontend.Mora{{Language: "en", Phones: []frontend.Phone{{Symbol: "AA", Role: "nucleus"}, {Symbol: "D", Role: "coda"}}}}
	base := speechPhoneDurations(m, 120)
	cfg := Config{SpeechModel: speechModelFixture(), MoraDurationMS: 120}
	got := speechDurationsForConfig(cfg, m)
	if math.Abs(got[0][0]-base[0][0]*1.5) > 1e-9 || got[0][1] != base[0][1] {
		t.Fatalf("coverage/fallback: %v vs %v", got, base)
	}
	cfg.ProsodyPitchOnly = true
	if got := speechDurationsForConfig(cfg, m); got[0][0] != base[0][0] {
		t.Fatal("pitch-only applied learned duration")
	}
	cfg.ProsodyPitchOnly = false
	cfg.Language, cfg.Phonemizer, cfg.Reading = "en", "en-arpasing", "AA1"
	model := cfg.SpeechModel
	cfg.SpeechModelPath = filepath.Join(t.TempDir(), "model.json")
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.SpeechModelPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.SpeechModel = nil
	first, err := PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	model.Duration["bias"] = math.Log(.5)
	data, err = json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.SpeechModelPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	second, err := PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.MoraDurationsMS[0] <= second.MoraDurationsMS[0] {
		t.Fatal("stale preview prediction reused")
	}
	cfg.MoraDurationsMS = []float64{300}
	manual, err := PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if manual.MoraDurationsMS[0] != 300 {
		t.Fatal("manual duration lost")
	}
	if configureSpeechModel(&cfg, "ja") == nil {
		t.Fatal("English model accepted for Japanese")
	}
}

func TestLearnedSpeechPitchAndEnergyRemainBounded(t *testing.T) {
	m := []frontend.Mora{{Language: "en", Phones: []frontend.Phone{{Symbol: "AA", Role: "nucleus"}}}}
	cfg := Config{SpeechModel: speechModelFixture(), MoraDurationMS: 120, ApplyPitch: true, IntonationStrength: 1}
	cfg.SpeechModel.Pitch = []map[string]float64{{"bias": 50}, {"bias": 60}, {"bias": 70}}
	cfg.SpeechModel.PitchPhoneCounts = map[string]int{"aa": 5}
	cfg.SpeechModel.Energy = map[string]float64{"bias": 100}
	curve, _ := (englishProfile{}).AutomaticPitchCurve(cfg, nil, m, []prosody.MoraTiming{{StartMS: 0, DurationMS: 180}}, 180)
	if curve == nil || curve.Cents[9] < 50 || curve.Cents[9] > 70 {
		t.Fatalf("learned contour absent: %v", curve)
	}
	predictions := applySpeechScore(cfg, m, nil)
	if predictions[0].EnergyFactor > 1.300001 || predictions[0].EnergyFactor < .7 {
		t.Fatal("energy unbounded")
	}
}
