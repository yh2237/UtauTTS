package prosody

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func TestManualPitchCurveInterpolatesMoraCenters(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Mode: "offset", Points: []ManualPitchPoint{
		{Position: 0, Mora: "あ", Cents: 0},
		{Position: 2, Mora: "う", Cents: 100},
	}}
	morae := []frontend.Mora{{Text: "あ"}, {Text: "い"}, {Text: "う"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}, {StartMS: 200, DurationMS: 100}}
	curve, err := file.Curve(morae, timings, 300)
	if err != nil {
		t.Fatal(err)
	}
	if curve.FrameMS != 10 || len(curve.Cents) != 31 {
		t.Fatalf("curve metadata = %+v", curve)
	}
	if math.Abs(curve.Cents[5]) > 0.01 || math.Abs(curve.Cents[15]) > 0.01 || math.Abs(curve.Cents[20]-50) > 0.01 || math.Abs(curve.Cents[25]-100) > 0.01 {
		t.Fatalf("curve values = %.2f, %.2f, %.2f, %.2f", curve.Cents[5], curve.Cents[15], curve.Cents[20], curve.Cents[25])
	}
}

func TestManualPitchCurveKeepsUnspecifiedMoraeAtZero(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Points: []ManualPitchPoint{{Position: 1, Cents: 80}}}
	morae := []frontend.Mora{{Text: "あ"}, {Text: "い"}, {Text: "う"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}, {StartMS: 200, DurationMS: 100}}
	curve, err := file.Curve(morae, timings, 300)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(curve.Cents[5]) > 0.01 || math.Abs(curve.Cents[15]-80) > 0.01 || math.Abs(curve.Cents[25]) > 0.01 {
		t.Fatalf("sparse curve centers = %.2f, %.2f, %.2f", curve.Cents[5], curve.Cents[15], curve.Cents[25])
	}
}

func TestManualPitchFramesModeResamplesToDuration(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Mode: "frames", Frames: []float64{0, 100, 0}}
	morae := []frontend.Mora{{Text: "あ"}, {Text: "い"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	if err := file.Validate(); err != nil {
		t.Fatal(err)
	}
	curve, err := file.Curve(morae, timings, 200)
	if err != nil {
		t.Fatal(err)
	}
	if curve.FrameMS != 10 || len(curve.Cents) != 21 {
		t.Fatalf("frame curve metadata = %+v", curve)
	}
	if math.Abs(curve.Cents[0]) > 0.01 || math.Abs(curve.Cents[10]-100) > 0.01 || math.Abs(curve.Cents[20]) > 0.01 {
		t.Fatalf("frame curve values = %.2f, %.2f, %.2f", curve.Cents[0], curve.Cents[10], curve.Cents[20])
	}
}

func TestManualPitchFramesKeepTheirSampleTimes(t *testing.T) {
	for _, duration := range []float64{100, 200} {
		file := &ManualPitchFile{Version: 1, Mode: "frames", FrameMS: 10, Frames: []float64{0, 100, 0}}
		if err := file.Validate(); err != nil {
			t.Fatal(err)
		}
		curve, err := file.Curve(nil, nil, duration)
		if err != nil {
			t.Fatal(err)
		}
		for index, got := range curve.Cents {
			want := 0.0
			if index == 1 {
				want = 100
			}
			if got != want {
				t.Fatalf("duration=%g frame=%d got=%g want=%g", duration, index, got, want)
			}
		}
	}
	file := &ManualPitchFile{Version: 1, Mode: "frames", FrameMS: 20, Frames: []float64{100}}
	curve, err := file.Curve(nil, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if curve.Cents[0] != 100 || curve.Cents[1] != 50 || curve.Cents[2] != 0 {
		t.Fatalf("fixed frame interpolation = %v", curve.Cents)
	}
}

func TestManualPitchFramesAddMoraOffsets(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Mode: "frames", FrameMS: 10, Frames: []float64{0, 25, 0},
		Points: []ManualPitchPoint{{Position: 0, Cents: 100}, {Position: 2, Cents: 200}}}
	morae := []frontend.Mora{{Text: "あ"}, {Pause: true}, {Text: "い"}}
	timings := []MoraTiming{{DurationMS: 100}, {StartMS: 100, DurationMS: 50}, {StartMS: 150, DurationMS: 100}}
	if err := file.Validate(); err != nil {
		t.Fatal(err)
	}
	curve, err := file.Curve(morae, timings, 250)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range map[int]float64{0: 100, 1: 125, 5: 100, 20: 200, 25: 200} {
		if got := curve.Cents[index]; math.Abs(got-want) > 1e-9 {
			t.Fatalf("frame=%d got=%g want=%g", index, got, want)
		}
	}
	file.Points[0].Position = 1
	if _, err := file.Curve(morae, timings, 250); err == nil {
		t.Fatal("combined pitch accepted a point on a pause")
	}
}

func TestManualPitchFrameStepRejectsInvalidValues(t *testing.T) {
	for _, step := range []float64{-1, math.Inf(1), math.NaN()} {
		file := &ManualPitchFile{Version: 1, Mode: "frames", FrameMS: step, Frames: []float64{0}}
		if err := file.Validate(); err == nil {
			t.Fatalf("frame_ms=%g was accepted", step)
		}
	}
}

func TestManualPitchValidateDefaultsModeToOffset(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Points: []ManualPitchPoint{{Position: 0, Cents: 10}}}
	if err := file.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if file.Mode != "offset" {
		t.Fatalf("mode = %q, want offset", file.Mode)
	}
}

func TestManualPitchValidationRejectsInvalidInputs(t *testing.T) {
	morae := []frontend.Mora{{Text: "あ"}, {Pause: true}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	for _, file := range []*ManualPitchFile{
		{Version: 1, Points: []ManualPitchPoint{{Position: 1, Cents: 10}}},
		{Version: 1, Points: []ManualPitchPoint{{Position: 0, Mora: "い", Cents: 10}}},
		{Version: 1, Points: []ManualPitchPoint{{Position: -1, Cents: 10}}},
	} {
		if _, err := file.Curve(morae, timings, 200); err == nil {
			t.Fatalf("Curve accepted invalid input %+v", file)
		}
	}
	for _, file := range []*ManualPitchFile{
		{Version: 1, Points: []ManualPitchPoint{{Position: -1, Cents: 10}}},
		{Points: []ManualPitchPoint{{Position: 0, Cents: 10}}},
		{Version: 1, Mode: "frames", Frames: make([]float64, 20001)},
	} {
		if err := file.Validate(); err == nil {
			t.Fatalf("Validate accepted %+v", file)
		}
	}
}

func TestManualPitchAcceptsCorrectionBeyondDisplayRange(t *testing.T) {
	// 自動予測値が大きいと、表示上の±800centに合わせる補正値はその分大きくなる。
	if err := (&ManualPitchFile{Version: 1, Points: []ManualPitchPoint{{Position: 0, Cents: 1500}}}).Validate(); err != nil {
		t.Fatalf("offset Validate: %v", err)
	}
	if err := (&ManualPitchFile{Version: 1, Mode: "frames", Frames: []float64{1500}}).Validate(); err != nil {
		t.Fatalf("frames Validate: %v", err)
	}
	if err := (&ManualPitchFile{Version: 1, Points: []ManualPitchPoint{{Position: 0, Cents: ManualPitchCentsLimit + 1}}}).Validate(); err == nil {
		t.Fatal("Validate accepted a value above the limit")
	}
	if err := (&ManualPitchFile{Version: 1, Mode: "frames", Frames: []float64{ManualPitchCentsLimit + 1}}).Validate(); err == nil {
		t.Fatal("Validate accepted a frame above the limit")
	}
}
