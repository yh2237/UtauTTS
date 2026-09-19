package prosody

import (
	"math"
	"testing"

	"utautts/internal/frontend"
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

func TestManualPitchCurveRejectsPauseAndMoraMismatch(t *testing.T) {
	morae := []frontend.Mora{{Text: "あ"}, {Pause: true}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
	for _, point := range []ManualPitchPoint{{Position: 1, Cents: 10}, {Position: 0, Mora: "い", Cents: 10}} {
		_, err := (&ManualPitchFile{Version: 1, Points: []ManualPitchPoint{point}}).Curve(morae, timings, 200)
		if err == nil {
			t.Fatalf("accepted invalid point %+v", point)
		}
	}
}

func TestManualPitchCurveRejectsNegativePosition(t *testing.T) {
	morae := []frontend.Mora{{Text: "あ"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}}
	file := &ManualPitchFile{Version: 1, Points: []ManualPitchPoint{{Position: -1, Cents: 10}}}
	if _, err := file.Curve(morae, timings, 200); err == nil {
		t.Fatal("curve accepted a negative position")
	}
}

func TestManualPitchValidateRejectsNegativePositionAndMissingVersion(t *testing.T) {
	for _, file := range []*ManualPitchFile{
		{Version: 1, Points: []ManualPitchPoint{{Position: -1, Cents: 10}}},
		{Points: []ManualPitchPoint{{Position: 0, Cents: 10}}},
	} {
		if err := file.Validate(); err == nil {
			t.Fatalf("Validate accepted %+v", file)
		}
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

func TestManualPitchFramesModeRejectsMixedPoints(t *testing.T) {
	file := &ManualPitchFile{Version: 1, Mode: "frames",
		Points: []ManualPitchPoint{{Position: 0, Cents: 10}}, Frames: []float64{0}}
	if err := file.Validate(); err == nil {
		t.Fatal("frames mode accepted mora points")
	}
	oversized := &ManualPitchFile{Version: 1, Mode: "frames", Frames: make([]float64, 20001)}
	if err := oversized.Validate(); err == nil {
		t.Fatal("oversized frames were accepted")
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
