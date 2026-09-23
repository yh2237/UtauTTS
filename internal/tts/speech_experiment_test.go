package tts

import (
	"math"
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestSpeechExperimentSeparatesTimingAndPitch(t *testing.T) {
	// E3で中国語の基線も母音核へ整列するため、中国語の抑揚案は基線と一致する。
	for _, tc := range []struct {
		cfg          Config
		pitchDiffers bool
	}{
		{Config{Language: "en", Phonemizer: "en-delta", Reading: "AH0 N AH1 DH ER0 | K AH1 P | SP", ApplyPitch: true, IntonationStrength: 1, Renderer: "utautts-world-phrase"}, true},
		{Config{Language: "zh", Reading: "ni3 hao3 | ma1 ma2 ma3 ma4 ma5", Renderer: "utautts-world-phrase"}, false},
	} {
		cfg := tc.cfg
		base, err := PredictProsody(cfg)
		if err != nil {
			t.Fatal(err)
		}
		cfg.SpeechProsodyExperiment = "pitch"
		changed, err := PredictProsody(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(base.MoraDurationsMS, changed.MoraDurationsMS) {
			t.Fatal("pitch experiment changed durations")
		}
		if tc.pitchDiffers && reflect.DeepEqual(base.FramePitchCurve, changed.FramePitchCurve) {
			t.Fatal("pitch experiment did not change contour")
		}
		if !tc.pitchDiffers && !reflect.DeepEqual(base.FramePitchCurve, changed.FramePitchCurve) {
			t.Fatal("中国語の抑揚案が母音核整列の基線と一致しない")
		}
		cfg.SpeechProsodyExperiment = "timing"
		changed, err = PredictProsody(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(base.MoraDurationsMS, changed.MoraDurationsMS) {
			t.Fatal("timing experiment did not redistribute time")
		}
		before, after := 0.0, 0.0
		for i, m := range base.Morae {
			if m.Pause {
				if math.Abs(before-after) > 1e-8 || changed.MoraDurationsMS[i] != base.MoraDurationsMS[i] {
					t.Fatal("phrase budget/pause changed")
				}
				before, after = 0, 0
				continue
			}
			before += base.MoraDurationsMS[i]
			after += changed.MoraDurationsMS[i]
			if changed.MoraDurationsMS[i] <= 0 {
				t.Fatal("nonpositive duration")
			}
		}
		if math.Abs(before-after) > 1e-8 {
			t.Fatal("final phrase budget changed")
		}
		cfg.MoraDurationsMS = append([]float64(nil), base.MoraDurationsMS...)
		cfg.SpeechProsodyExperiment = "both"
		manual, err := PredictProsody(cfg)
		if err != nil || !reflect.DeepEqual(base.MoraDurationsMS, manual.MoraDurationsMS) {
			t.Fatal("manual durations changed", err)
		}
	}
}

func TestSpeechExperimentPartialManualBudgetAndValidation(t *testing.T) {
	cfg := Config{Language: "en", Phonemizer: "en-vccv", Reading: "HH AH0 L OW1 | SP", Renderer: "utautts-world-phrase", MoraDurationsMS: []float64{160, 0}}
	base, err := PredictProsody(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SpeechProsodyExperiment = "timing"
	got, err := PredictProsody(cfg)
	if err != nil || !reflect.DeepEqual(got.MoraDurationsMS, base.MoraDurationsMS) {
		t.Fatal("single free duration changed budget", err)
	}
	for _, bad := range []Config{{SpeechProsodyExperiment: "other"}, {Language: "ja", SpeechProsodyExperiment: "both"}, {Language: "zh", Renderer: "waveform", SpeechProsodyExperiment: "pitch"}} {
		if validateSpeechExperiment(bad) == nil {
			t.Fatal("unsupported experiment accepted", bad)
		}
	}
}

// E3: 声調を頭子音と語尾子音ではなく母音核の区間へ置く。
func TestMandarinToneCurveAlignsToNucleus(t *testing.T) {
	morae := []frontend.Mora{{Tone: 4, Phones: []frontend.Phone{
		{Symbol: "n", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "n", Role: "coda"},
	}}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 200}}
	curve := mandarinToneCurve(morae, timings, 200)
	if curve.Cents[2] != 145 {
		t.Fatalf("頭子音区間で声調が動いた: %.1f", curve.Cents[2])
	}
	if curve.Cents[17] != -145 {
		t.Fatalf("語尾子音区間で声調が動いた: %.1f", curve.Cents[17])
	}
	if curve.Cents[10] == 145 || curve.Cents[10] == -145 {
		t.Fatalf("母音核で声調が動いていない: %.1f", curve.Cents[10])
	}
}
