package base

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestPitchCurveFactorInterpolatesInCents(t *testing.T) {
	curve := &PitchCurve{FrameMS: 10, Cents: []float64{0, 100}}
	want := math.Pow(2, 50.0/1200)
	if got := PitchCurveFactorAt(curve, 5); math.Abs(got-want) > 1e-12 {
		t.Fatalf("factor at midpoint = %.12f, want %.12f", got, want)
	}
	if got, end := PitchCurveFactorAt(curve, 100), math.Pow(2, 100.0/1200); math.Abs(got-end) > 1e-12 {
		t.Fatalf("factor after end = %.12f, want %.12f", got, end)
	}
}

func TestOpenUtauEnvelopeUsesNextPhoneTailTiming(t *testing.T) {
	units := []plan.Unit{
		{NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	timings, phraseStart := OpenUtauPhoneTimingsWithCoda(units, CVVCTimingSequential, false)
	if timings[0].TailIntrude != 40 || timings[0].TailOverlap != 10 || !timings[1].Overlapped || phraseStart != -30 {
		t.Fatalf("timing = %+v %+v phraseStart=%.1f", timings[0], timings[1], phraseStart)
	}
	envelope := OpenUtauEnvelopeFromTiming(units[0], timings[0])
	wantX := []float64{-30, -25, 0, 60, 70}
	for index, want := range wantX {
		if envelope[index].XMS != want {
			t.Fatalf("envelope point %d x = %.1f, want %.1f", index, envelope[index].XMS, want)
		}
	}
}

func TestOpenUtauSequentialCVVCTimingChainsTransitionAndMainPhone(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{Position: 1, Role: "transition", NoteStartMS: 100, DurationMS: 30, PreutteranceMS: 50, OverlapMS: 20},
		{Position: 1, Role: "mora", NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	timings, phraseStart := OpenUtauPhoneTimingsWithCoda(units, CVVCTimingSequential, false)
	if timings[0].TailIntrude != 50 || timings[0].TailOverlap != 20 {
		t.Fatalf("previous mora tail timing = %+v", timings[0])
	}
	if timings[1].TailIntrude != 20 || timings[1].TailOverlap != 5 {
		t.Fatalf("transition tail timing = %+v", timings[1])
	}
	if timings[2].Preutter != 20 || timings[2].Overlap != 5 || !timings[2].Overlapped {
		t.Fatalf("main timing = %+v", timings[2])
	}
	if phraseStart != -30 {
		t.Fatalf("phrase start = %.1f", phraseStart)
	}
}

func TestCVVCPreBoundaryEnvelopeEndsAtFollowingMoraBoundary(t *testing.T) {
	points := []EnvelopePoint{
		{XMS: -80, Y: 0}, {XMS: -50, Y: 1}, {XMS: 0, Y: 1},
		{XMS: 10, Y: 1}, {XMS: 30, Y: 0},
	}
	got := CVVCPreBoundaryEnvelope(points, OpenUtauPhoneTiming{TailOverlap: 20})
	if got[2].XMS != -20 || got[3].XMS != -20 || got[4].XMS != 0 {
		t.Fatalf("unexpected pre-boundary envelope: %+v", got)
	}
	if points[4].XMS != 30 {
		t.Fatal("input envelope was mutated")
	}
}

func TestNormalizeTimingLeavesOrdinaryBankAlone(t *testing.T) {
	unit := plan.Unit{DurationMS: 140, PreutteranceMS: 60, OverlapMS: 20, ConsonantMS: 100}
	got := NormalizeTiming(unit, 20)
	if got.PreutteranceMS != 60 || got.OverlapMS != 20 || got.ConsonantMS != 100 || got.Scale != 1 {
		t.Fatalf("ordinary timing changed: %#v", got)
	}
}

func TestSourceCacheReusesMonoAndNormalizedAudio(t *testing.T) {
	path := t.TempDir() + "/stereo.wav"
	data := []int16{1000, -1000, 2000, -2000, 3000, -3000, 4000, -4000}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 2, Data: data}); err != nil {
		t.Fatal(err)
	}

	cache := SourceCache{}
	first, err := cache.LoadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.LoadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("mono source was converted more than once")
	}
	if first.Channels != 1 || len(first.Data) != len(data)/2 {
		t.Fatalf("unexpected mono source: %#v", first)
	}

	native, err := cache.LoadNormalized(path, 16000)
	if err != nil {
		t.Fatal(err)
	}
	resampled, err := cache.LoadNormalized(path, 8000)
	if err != nil {
		t.Fatal(err)
	}
	resampledAgain, err := cache.LoadNormalized(path, 8000)
	if err != nil {
		t.Fatal(err)
	}
	if native != first || resampled != resampledAgain {
		t.Fatal("normalized source cache did not reuse its entries")
	}
	if resampled.SampleRate != 8000 || len(resampled.Data) != len(first.Data)/2 {
		t.Fatalf("unexpected resampled source: %#v", resampled)
	}
}

func TestMeasureWorldPitchesHandlesMultipleUnits(t *testing.T) {
	ClearWAVCache()
	defer ClearWAVCache()
	directory := t.TempDir()
	units := make([]plan.Unit, 4)
	for index, hz := range []float64{180, 200, 220, 240} {
		path := filepath.Join(directory, fmt.Sprintf("tone-%d.wav", index))
		data := make([]int16, 6400)
		for frame := range data {
			data[frame] = int16(6000 * math.Sin(2*math.Pi*hz*float64(frame)/16000))
		}
		if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
		units[index] = plan.Unit{Position: index, Source: path, DurationMS: 150}
	}
	values, sampleRate, err := MeasureWorldPitches(&plan.Plan{Units: units}, &SourceCache{})
	if err != nil {
		t.Fatal(err)
	}
	if sampleRate != 16000 || len(values) != len(units) {
		t.Fatalf("sample rate=%d pitches=%v", sampleRate, values)
	}
	for index, value := range values {
		if value <= 0 {
			t.Fatalf("pitch %d was not measured: %v", index, values)
		}
	}
}

func TestStabilizeWorldPitches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []float64
		want  []float64
		tol   []float64
	}{
		{"corrects harmonic jump", []float64{296, 446, 298}, []float64{296, 297.333333, 298}, []float64{2, 2, 2}},
		{"keeps lower short phrase anchor", []float64{296, 446}, []float64{296, 297.333333}, []float64{0.01, 2}},
		{"keeps ordinary movement", []float64{280, 296, 315}, []float64{280, 296, 315}, []float64{0, 0, 0}},
		{"corrects subharmonic drop", []float64{207, 69, 205}, []float64{207, 207, 205}, []float64{0, 1, 0}},
		{"corrects near half pitch", []float64{207, 115, 205}, []float64{207, 230, 205}, []float64{0, 1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := StabilizeWorldPitches(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("pitches = %v, want %d values", got, len(tc.want))
			}
			for index, want := range tc.want {
				if math.IsNaN(got[index]) || math.Abs(got[index]-want) > tc.tol[index] {
					t.Fatalf("pitch[%d] = %.3f, want %.3f (±%g) in %#v", index, got[index], want, tc.tol[index], got)
				}
			}
		})
	}
}

func TestStabilizeSingleCVPitches(t *testing.T) {
	planFor := func(vowels ...string) *plan.Plan {
		units := make([]plan.Unit, len(vowels))
		morae := make([]frontend.Mora, len(vowels))
		for index, vowel := range vowels {
			units[index] = plan.Unit{Role: "mora", Position: index}
			morae[index] = frontend.Mora{Vowel: vowel}
		}
		return &plan.Plan{SingleCV: true, Units: units, Morae: morae}
	}
	for _, tc := range []struct {
		name   string
		vowels []string
		input  []float64
		want   []float64
		tol    []float64
	}{
		{"corrects moderate outlier", []string{"a", "i", "u"}, []float64{207, 155, 208}, []float64{207, 206.667, 208}, []float64{0, 2, 0}},
		{"corrects untrackable low outlier", []string{"a", "a", "a"}, []float64{207, 86, 208}, []float64{207, 207.5, 208}, []float64{0, 2, 0}},
		{"leaves ambiguous short phrase", []string{"a", "i"}, []float64{207, 155}, []float64{207, 155}, []float64{0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := StabilizeSingleCVPitches(planFor(tc.vowels...), tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("pitches = %v, want %d values", got, len(tc.want))
			}
			for index, want := range tc.want {
				if math.IsNaN(got[index]) || math.Abs(got[index]-want) > tc.tol[index] {
					t.Fatalf("pitch[%d] = %.3f, want %.3f (±%g) in %#v", index, got[index], want, tc.tol[index], got)
				}
			}
		})
	}
}
