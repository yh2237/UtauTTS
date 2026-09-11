package tts

import (
	"math"
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestSpeechExperimentSeparatesTimingAndPitch(t *testing.T) {
	for _, cfg := range []Config{
		{Language: "en", Phonemizer: "en-delta", Reading: "AH0 N AH1 DH ER0 | K AH1 P | SP", ApplyPitch: true, IntonationStrength: 1, Renderer: "utautts-world-phrase"},
		{Language: "zh", Reading: "ni3 hao3 | ma1 ma2 ma3 ma4 ma5", Renderer: "utautts-world-phrase"},
	} {
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
		if reflect.DeepEqual(base.FramePitchCurve, changed.FramePitchCurve) {
			t.Fatal("pitch experiment did not change contour")
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

func TestMandarinExperimentalCurveAnchorsAtVowelStart(t *testing.T) {
	morae := []frontend.Mora{{Tone: 2, Phones: []frontend.Phone{{Symbol: "n", Role: "onset"}, {Symbol: "i", Role: "nucleus"}}}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 120}}
	legacy := mandarinToneCurve(morae, timings, 120)
	aligned := mandarinToneCurveAligned(morae, timings, 120, true)
	if legacy.Cents[3] != legacy.Cents[0] || aligned.Cents[3] >= aligned.Cents[0] {
		t.Fatal("onset alignment not applied")
	}
	if aligned.Cents[12] != 145 || aligned.Cents[12] <= aligned.Cents[3] {
		t.Fatal("tone endpoint/direction lost")
	}
}
