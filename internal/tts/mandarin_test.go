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

// E3: 母音核が無い音節は音節全体へ置く従来動作へ戻す。
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

// E3: 軽声(5)のF0は前の声調に追従する。
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
