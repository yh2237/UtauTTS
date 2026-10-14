package prosody

import (
	"math"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func TestFrameContourRegression(t *testing.T) {
	model := &Model{FramePitch: &FramePitchModel{
		FeatureNames: []string{"vowel=a", "mora_progress", "frame_position", "question_distance"},
		InputWeights: [][]float64{{0.5, 0.2, -0.3, 0.4}, {-0.1, 0.6, 0.2, -0.3}},
		InputBias:    []float64{0.1, -0.2}, OutputWeight: []float64{70, -40}, OutputBias: 10,
		FrameMS: 10, LowCents: -250, HighCents: 250,
		Layers: []SequencePitchLayer{{
			Dilation: 1, Bias: []float64{0.03, -0.02},
			Weights: [][][]float64{
				{{0.1, 0.2, -0.1}, {0.2, -0.1, 0.3}},
				{{-0.2, 0.1, 0.2}, {0.1, 0.3, -0.1}},
			},
		}},
	}}
	morae := []frontend.Mora{{Text: "あ", Vowel: "a"}, {Pause: true}, {Text: "い", Vowel: "i"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 40}, {StartMS: 40, DurationMS: 20}, {StartMS: 60, DurationMS: 40}}
	// 個別フレーム確保での出力を保持し、配列の共有や計算順の変更を検出する。
	wants := [][]float64{
		{20.54933311102396, 15.835072502551437, 12.900907768496111, 0, 0, 0, 7.877850055522217, -1.3978292465694153, -3.768294677863146, -5.149915581876975, -20.044734287353357},
		{26.026468487274236, 22.740470447152894, 16.73888033592585, 0, 0, 0, 0.40490801332848037, -12.126426475425873, -18.808472035797884, -24.47975294441897, -44.01498512564778},
	}
	for index, question := range []bool{false, true} {
		curve := model.PredictFrameContour(morae, nil, timings, 100, question)
		if curve == nil || curve.FrameMS != 10 || len(curve.Cents) != len(wants[index]) {
			t.Fatalf("invalid contour: %+v", curve)
		}
		again := model.PredictFrameContour(morae, nil, timings, 100, question)
		if !reflect.DeepEqual(curve, again) || !reflect.DeepEqual(model.FramePitch.InputBias, []float64{0.1, -0.2}) {
			t.Fatal("prediction mutated model or a previous contour")
		}
		for i, value := range curve.Cents {
			if math.IsNaN(value) || math.Abs(value-wants[index][i]) > 1e-9 {
				t.Fatalf("question=%v frame %d=%v, want %v", question, i, value, wants[index][i])
			}
		}
	}
}
