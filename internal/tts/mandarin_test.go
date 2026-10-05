package tts

import (
	"path/filepath"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestBundledMandarinModelIsUsedForJapaneseDefault(t *testing.T) {
	defaultPath := filepath.Join("..", "..", "models", "frame-intonation-tcn-v10.json")
	model, err := resolveProsodyModelForLanguage(Config{ProsodyModelPath: defaultPath}, frontend.LanguageChinese)
	if err != nil || model == nil || model.ID != "tone-intonation-zh-v1" {
		t.Fatalf("Mandarin fallback = %#v, %v", model, err)
	}
	base := Config{Language: frontend.LanguageChinese, Reading: "ni3 hao3 jin1 tian1 hen3 hao3", MoraDurationMS: 120}
	rule, err := PredictProsody(base)
	if err != nil {
		t.Fatal(err)
	}
	base.ProsodyModelPath = defaultPath
	learned, err := PredictProsody(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(rule.MoraDurationsMS) != len(learned.MoraDurationsMS) || rule.FramePitchCurve == nil || learned.FramePitchCurve == nil {
		t.Fatal("Mandarin preview is incomplete")
	}
	changed := false
	for index := range rule.MoraDurationsMS {
		if rule.MoraDurationsMS[index] != learned.MoraDurationsMS[index] {
			t.Fatalf("Mandarin model changed duration at %d", index)
		}
	}
	for index := range rule.FramePitchCurve.Cents {
		if rule.FramePitchCurve.Cents[index] != learned.FramePitchCurve.Cents[index] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("bundled Mandarin model did not change the pitch contour")
	}
}

func TestMandarinToneCurveUsesCanonicalDirections(t *testing.T) {
	morae := []frontend.Mora{{Tone: 1}, {Tone: 2}, {Tone: 3}, {Tone: 4}}
	timings := []prosody.MoraTiming{
		{StartMS: 0, DurationMS: 100},
		{StartMS: 100, DurationMS: 100},
		{StartMS: 200, DurationMS: 100},
		{StartMS: 300, DurationMS: 100},
	}
	curve := mandarinToneCurve(morae, timings, 400)
	if curve == nil {
		t.Fatal("声調曲線が生成されなかった")
	}
	if curve.Cents[0] != curve.Cents[8] {
		t.Fatal("一声が平坦ではない")
	}
	if curve.Cents[11] >= curve.Cents[19] {
		t.Fatal("二声が上昇していない")
	}
	if curve.Cents[26] >= curve.Cents[29] {
		t.Fatal("非終端の三声が低く保たれていない")
	}
	if curve.Cents[31] <= curve.Cents[39] {
		t.Fatal("四声が下降していない")
	}
}

func TestMandarinLearnedCorrectionChangesOnlySelectedTone(t *testing.T) {
	morae := []frontend.Mora{{Tone: 1}, {Tone: 2}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	base := mandarinToneCurve(morae, timings, 200)
	model := &prosody.MandarinIntonationModel{
		FeatureNames: []string{"tone_1"}, Knots: []float64{0.25, 0.75},
		Weights: [][]float64{{80}, {80}}, Strength: 0.5, MaxCents: 100,
	}
	got := applyMandarinIntonation(base, model, morae, timings)
	if got.Cents[5] != base.Cents[5]+40 || got.Cents[15] != base.Cents[15] {
		t.Fatalf("learned correction affected wrong frames: got=%v base=%v", got.Cents, base.Cents)
	}
	if base.Cents[5] == got.Cents[5] {
		t.Fatal("base tone curve was changed in place")
	}
}

func TestMandarinThirdToneSandhiTurnsFirstToneUpward(t *testing.T) {
	morae := []frontend.Mora{{Tone: 3}, {Tone: 3}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	curve := mandarinToneCurve(morae, timings, 200)
	if curve.Cents[1] >= curve.Cents[9] {
		t.Fatalf("三声連続の先頭が二声化されていない: %.1f -> %.1f", curve.Cents[1], curve.Cents[9])
	}
	if curve.Cents[15] >= curve.Cents[19] {
		t.Fatalf("終端三声の上昇部がない: %.1f -> %.1f", curve.Cents[15], curve.Cents[19])
	}
}

func TestPredictProsodyReturnsMandarinToneCurveWithoutModel(t *testing.T) {
	preview, err := PredictProsody(Config{
		Language:       frontend.LanguageChinese,
		Reading:        "ni3 hao3",
		MoraDurationMS: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.FramePitchCurve == nil || len(preview.PitchPoints) != 2 {
		t.Fatalf("preview=%#v", preview)
	}
	if preview.PitchPoints[0] >= preview.FramePitchCurve.Cents[9] {
		t.Fatalf("三声変調がプレビューへ反映されていない: %#v", preview.FramePitchCurve.Cents)
	}
}

func TestMandarinToneCurveFallsBackWithoutNucleus(t *testing.T) {
	morae := []frontend.Mora{{Tone: 2}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 120}}
	curve := mandarinToneCurve(morae, timings, 120)
	if curve == nil || curve.Cents[0] != -20 {
		t.Fatalf("母音核不明時のフォールバックが開始点へ置かれていない: %#v", curve)
	}
	if curve.Cents[10] <= curve.Cents[0] {
		t.Fatalf("フォールバック曲線が上昇していない: %.1f", curve.Cents[10])
	}
}

func TestMandarinNeutralToneDependsOnPreviousTone(t *testing.T) {
	high := mandarinNeutralTonePoints(3)
	low := mandarinNeutralTonePoints(1)
	if high[len(high)-1].cents <= low[len(low)-1].cents {
		t.Fatalf("軽声のF0が前声調を反映していない: 3声後=%.0f 1声後=%.0f", high[len(high)-1].cents, low[len(low)-1].cents)
	}
	if got := mandarinNeutralTonePoints(5); len(got) == 0 || got[len(got)-1].cents >= high[len(high)-1].cents {
		t.Fatalf("軽声連続のF0が高すぎる: %#v", got)
	}

	morae := []frontend.Mora{{Tone: 3}, {Tone: 5}}
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	afterThird := mandarinToneCurve(morae, timings, 200)
	morae[0].Tone = 1
	afterFirst := mandarinToneCurve(morae, timings, 200)
	if afterThird.Cents[19] <= afterFirst.Cents[19] {
		t.Fatalf("曲線上の軽声が前声調を反映していない: %.1f <= %.1f", afterThird.Cents[19], afterFirst.Cents[19])
	}
}
