package tts

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

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
