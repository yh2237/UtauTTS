package tts

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/prosody"
	"utautts/internal/render"
)

func TestBlendPitchCurvesWeightsSecondCurve(t *testing.T) {
	a := &render.PitchCurve{FrameMS: 10, Cents: []float64{100, 0, -100}}
	b := &render.PitchCurve{FrameMS: 10, Cents: []float64{0, 100}}
	got := blendPitchCurves(a, b, .25)
	want := []float64{75, 25, -100}
	for index := range want {
		if math.Abs(got.Cents[index]-want[index]) > 1e-9 {
			t.Fatalf("blend = %v, want %v", got.Cents, want)
		}
	}
	if a.Cents[0] != 100 {
		t.Fatal("blend modified its input")
	}
	if blendPitchCurves(a, &render.PitchCurve{FrameMS: 5, Cents: []float64{0}}, .5) != a {
		t.Fatal("curves with different frame lengths must not be mixed")
	}
}

// 自然スケールのF0ヘッドは、既定の強さ（4）で輪郭をそのまま、半分の強さで半分にする。
func TestScaleUnifiedPitchCurveNaturalScale(t *testing.T) {
	model, err := prosody.LoadModel(filepath.Join("..", "..", "models", "intonation-ja-v11.json"))
	if err != nil {
		t.Fatal(err)
	}
	if model.F0Head == nil || model.F0Head.F0Scale() != 0 {
		t.Fatal("bundled v11 must carry a natural-scale F0 head")
	}
	contour := &render.PitchCurve{FrameMS: 10, Cents: []float64{80, -40}}
	if got := scaleUnifiedPitchCurve(contour, model.F0Head, 4); got.Cents[0] != 80 || got.Cents[1] != -40 {
		t.Fatalf("default strength changed the contour: %v", got.Cents)
	}
	if got := scaleUnifiedPitchCurve(contour, model.F0Head, 2); got.Cents[0] != 40 || got.Cents[1] != -20 {
		t.Fatalf("half strength = %v", got.Cents)
	}
	if scaleUnifiedPitchCurve(contour, model.F0Head, 0) != nil {
		t.Fatal("zero strength must disable the automatic contour")
	}
}
