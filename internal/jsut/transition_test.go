package jsut

import (
	"math"
	"testing"

	"utautts/internal/audio"
)

func TestBuildTransitionExamplesKeepsNaturalResidual(t *testing.T) {
	const rate = 16000
	pcm := &audio.PCM{SampleRate: rate, Channels: 1, Data: make([]int16, rate/5)}
	for index := range pcm.Data {
		frequency := 180.0
		if index >= len(pcm.Data)/2 {
			frequency = 260
		}
		pcm.Data[index] = int16(9000 * math.Sin(2*math.Pi*frequency*float64(index)/rate))
	}
	phones := []Phone{{Index: 0, Symbol: "a", StartMS: 0, EndMS: 100, DurationMS: 100}, {Index: 1, Symbol: "k", StartMS: 100, EndMS: 200, DurationMS: 100}}
	label := 1.0
	alignment := NewAlignment("test", "あか", "test.wav", phones, []Boundary{{Index: 0, LeftPhone: "a", RightPhone: "k", BoundaryTimeMS: 100, JoinType: "phone", Trainable: true, Label: &label}})
	examples, err := BuildTransitionExamples(alignment, pcm, 35, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 1 || len(examples[0].Frames) < 5 || examples[0].LeftPhone != "a" || examples[0].RightPhone != "k" {
		t.Fatalf("examples: %+v", examples)
	}
	if len(examples[0].Frames[0].SpectrumResidualDB) != 10 {
		t.Fatalf("residual bands: %+v", examples[0].Frames[0])
	}
}

func TestTransitionPriorReducesRepeatedResidual(t *testing.T) {
	frame := TransitionFrame{RMSResidualDB: 2, SpectrumResidualDB: []float64{1, -1}}
	examples := make([]TransitionExample, 12)
	for index := range examples {
		examples[index] = TransitionExample{LeftPhone: "a", RightPhone: "k", Frames: []TransitionFrame{frame, frame, frame}}
	}
	prior, err := BuildTransitionPrior(examples[:10], 3, "test")
	if err != nil {
		t.Fatal(err)
	}
	metrics := EvaluateTransitionPrior(prior, examples[10:])
	if metrics.Frames != 6 || metrics.PriorSpectrumMAE >= metrics.BaselineSpectrumMAE || metrics.PriorRMSMAE >= metrics.BaselineRMSMAE {
		t.Fatalf("metrics: %+v", metrics)
	}
}
