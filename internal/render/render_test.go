package render

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestRenderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := renderMutable(nil, Config{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRenderRejectsInvalidConfiguration(t *testing.T) {
	t.Run("nonfinite pitch", testRenderRejectsNonFinitePitchCurve)
	t.Run("nonfinite timing", testRenderRejectsNonFiniteTimingConfiguration)
	t.Run("unsafe pitch range", testRenderRejectsUnsafePitchCurveRangeAndFrame)
}

func TestConstrainPitchCurveDoesNotMutateAndLimitsSlope(t *testing.T) {
	source := &PitchCurve{FrameMS: 10, Cents: []float64{0, 80, -80, 80, 0}}
	result := ConstrainPitchCurve(source, 20, 4)
	if source.Cents[1] != 80 || result == source {
		t.Fatal("pitch curve smoothing mutated its input")
	}
	for index := 1; index < len(result.Cents); index++ {
		if math.Abs(result.Cents[index]-result.Cents[index-1]) > 4.000001 {
			t.Fatalf("step %d is too steep: %f -> %f", index, result.Cents[index-1], result.Cents[index])
		}
	}
}

func testRenderRejectsNonFinitePitchCurve(t *testing.T) {
	for _, curve := range []*PitchCurve{
		{FrameMS: math.NaN(), Cents: []float64{0}},
		{FrameMS: 5, Cents: []float64{math.Inf(1)}},
	} {
		_, err := renderMutable(&plan.Plan{Units: []plan.Unit{{}}}, Config{ApplyPitch: true, PitchCurve: curve})
		if err == nil {
			t.Fatalf("accepted non-finite curve: %+v", curve)
		}
	}
}

func testRenderRejectsNonFiniteTimingConfiguration(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := renderMutable(&plan.Plan{Units: []plan.Unit{{}}}, Config{ReleaseMS: value})
		if err == nil {
			t.Fatalf("accepted non-finite release_ms %v", value)
		}
	}
}

func testRenderRejectsUnsafePitchCurveRangeAndFrame(t *testing.T) {
	for _, curve := range []*PitchCurve{
		{FrameMS: 0.01, Cents: []float64{0}},
		{FrameMS: 5, Cents: []float64{4801}},
		{FrameMS: 5, Cents: []float64{-4801}},
	} {
		_, err := renderMutable(&plan.Plan{Units: []plan.Unit{{}}}, Config{ApplyPitch: true, PitchCurve: curve})
		if err == nil {
			t.Fatalf("accepted unsafe curve: %+v", curve)
		}
	}
}

func TestRenderWithReportKeepsSelectionPlanImmutable(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 400)
	for index := range data {
		data[index] = 9000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	input := &plan.Plan{DurationMS: 140, Units: []plan.Unit{{
		Position: 0, Alias: "a", Source: path, NoteStartMS: 0, DurationMS: 140,
		PreutteranceMS: 80, OverlapMS: 20, ConsonantMS: 100,
	}}}
	before := plan.Clone(input)

	result, err := RenderWithReport(input, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.Audio == nil || len(result.Report.Units) != 1 {
		t.Fatalf("render result = %#v", result)
	}
	if !reflect.DeepEqual(input, before) {
		t.Fatalf("renderer mutated input plan: before=%#v after=%#v", before, input)
	}
}

func TestReportFromPlanExportsSpeechDiagnostics(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{DurationMS: 120}}}
	working := plan.Clone(p)
	working.Units[0].SpeechRetimeApplied = true
	working.Units[0].SpeechJoinApplied = true
	working.Units[0].EffectiveConsonantMS = 80
	report := reportFromPlan(working)
	exported := plan.Clone(p)
	report.ApplyTo(exported)
	if p.Units[0].SpeechRetimeApplied || p.Units[0].SpeechJoinApplied {
		t.Fatal("canonical plan mutated")
	}
	if !exported.Units[0].SpeechRetimeApplied || !exported.Units[0].SpeechJoinApplied || exported.Units[0].EffectiveConsonantMS != 80 {
		t.Fatal("lost applied diagnostics")
	}
}
