package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"utautts/internal/provider"
)

func TestWorldSpeechMappingAnchorsAndStop(t *testing.T) {
	u := unit{OffsetMS: 13, ConsonantMS: 100, RequiredLengthMS: 240, ConsonantVelocity: 100,
		Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 60, TargetOnsetMS: 40, ProtectStop: true}}
	a, ok := worldSpeechAnchors(u, 400)
	if !ok {
		t.Fatal("valid anchors rejected")
	}
	for _, pair := range [][2]float64{{0, 0}, {40, 63}, {a.targetFixed, 103}, {240, 400}, {35, 58}} {
		if got := mapWorldSourceTime(u, 400, pair[0]); math.Abs(got-pair[1]) > 1e-9 {
			t.Fatalf("map(%g)=%g want %g", pair[0], got, pair[1])
		}
	}
	previous := -1.0
	for tm := 0.0; tm <= 260; tm += .25 {
		v := mapWorldSourceTime(u, 400, tm)
		if v < previous || v > 400 {
			t.Fatal("nonmonotone/out of bounds", tm, v)
		}
		previous = v
	}
	u.SkipMS = 15
	if got := mapWorldSourceTime(u, 400, 25); got != 63 {
		t.Fatal("skip shifted vowel onset", got)
	}
	for _, change := range []func(*unit){
		func(u *unit) { u.Speech.SourceOnsetMS = 0 },
		func(u *unit) { u.RequiredLengthMS = 50 },
		func(u *unit) { u.Speech.TargetOnsetMS = math.NaN() },
		func(u *unit) { u.ConsonantMS = 395 },
	} {
		bad := u
		speech := *u.Speech
		bad.Speech = &speech
		change(&bad)
		if _, ok := worldSpeechAnchors(bad, 400); ok {
			t.Fatal("invalid anchors accepted", bad)
		}
		plain := bad
		plain.Speech = nil
		if got, want := mapWorldSourceTime(bad, 400, 80), mapWorldSourceTime(plain, 400, 80); got != want {
			t.Fatal("fallback differs", got, want)
		}
	}
}

func TestWorldSpeechMappingHonorsTargetFixed(t *testing.T) {
	u := unit{OffsetMS: 0, ConsonantMS: 100, RequiredLengthMS: 240, Speech: &provider.WorldSpeechTiming{
		SourceOnsetMS: 60, TargetOnsetMS: 40, TargetFixedMS: 96}}
	a, ok := worldSpeechAnchors(u, 400)
	if !ok {
		t.Fatal("valid anchors rejected")
	}
	if math.Abs(a.targetFixed-96) > 1e-9 {
		t.Fatalf("target fixed = %.3f, want 96", a.targetFixed)
	}
}

func TestWorldSpeechPreserveStopOnlyDoesNotRetargetFeatures(t *testing.T) {
	u := unit{OffsetMS: 3, ConsonantMS: 70, RequiredLengthMS: 180,
		Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 50, TargetOnsetMS: 50, PreserveStopOnly: true}}
	if _, ok := worldSpeechAnchors(u, 300); ok {
		t.Fatal("preserve-only stop unexpectedly enabled feature retiming")
	}
	plain := u
	plain.Speech = nil
	if got, want := mapWorldSourceTime(u, 300, 90), mapWorldSourceTime(plain, 300, 90); got != want {
		t.Fatalf("preserve-only stop changed source mapping: %f != %f", got, want)
	}
}

func speechTestFeatures() worldFeatures {
	f := worldFeatures{Frames: 7, FFTSize: 2, F0: []float64{180, 190, 200, 220, 210, 190, 180}, Spectrum: make([]float64, 14), Aperiodicity: make([]float64, 14)}
	for i := range f.Spectrum {
		f.Spectrum[i] = 1
		f.Aperiodicity[i] = .1
	}
	f.Spectrum[6], f.Spectrum[7] = 1.8, 1.8
	return f
}

func TestWorldSpeechJoinPreservesPitchAndRejectsUnsafeWindows(t *testing.T) {
	f := speechTestFeatures()
	pitch := append([]float64(nil), f.F0...)
	if !smoothWorldVowel(&f, 1, 5) {
		t.Fatal("spike not repaired")
	}
	if f.Spectrum[6] >= 1.8 || !reflect.DeepEqual(pitch, f.F0) || f.Frames != 7 {
		t.Fatal("pitch/length/spike", f)
	}
	if f.Spectrum[2] != 1 || f.Spectrum[10] != 1 {
		t.Fatal("endpoints changed")
	}
	for _, change := range []func(*worldFeatures){
		func(f *worldFeatures) {
			for i := range f.Spectrum {
				f.Spectrum[i] = 1
			}
		},
		func(f *worldFeatures) { f.F0[3] = 0 },
		func(f *worldFeatures) { f.Spectrum[6] = 100 },
	} {
		f := speechTestFeatures()
		change(&f)
		before := append([]float64(nil), f.Spectrum...)
		if smoothWorldVowel(&f, 1, 5) || !reflect.DeepEqual(before, f.Spectrum) {
			t.Fatal("unsafe/no-op window changed")
		}
	}
}

func TestWorldSpeechJoinEligibilityAndReport(t *testing.T) {
	in := manifest{Units: []unit{{PositionMS: 0, LengthMS: 40}, {PositionMS: 20, LengthMS: 60, Speech: &provider.WorldSpeechTiming{UnitIndex: 2, TargetOnsetMS: 10, VowelJoin: true}}}}
	f := speechTestFeatures()
	if !applyWorldSpeechJoins(in, &f)[2].JoinApplied {
		t.Fatal("missing applied report")
	}
	in.Units = append(in.Units, unit{PositionMS: 20, LengthMS: 50})
	f = speechTestFeatures()
	if len(applyWorldSpeechJoins(in, &f)) != 0 {
		t.Fatal("smoothed across a third unit")
	}
	in.Units = in.Units[:2]
	in.Units[1].Speech.TargetJoinMS = math.NaN()
	f = speechTestFeatures()
	if len(applyWorldSpeechJoins(in, &f)) != 0 {
		t.Fatal("NaN join anchor was accepted")
	}
}

func TestWorldSpeechJoinUsesExplicitAnchor(t *testing.T) {
	in := manifest{Units: []unit{
		{PositionMS: 0, LengthMS: 100},
		{PositionMS: 20, LengthMS: 80, Speech: &provider.WorldSpeechTiming{UnitIndex: 1, TargetOnsetMS: 10, TargetJoinMS: 20, VowelJoin: true}},
	}}
	f := speechTestFeatures()
	if !applyWorldSpeechJoins(in, &f)[1].JoinApplied {
		t.Fatal("explicit anchor did not select the repair window")
	}
}

func TestWorldSpeechWireAndUnsupportedEngine(t *testing.T) {
	speech := &provider.WorldSpeechTiming{UnitIndex: 4, SourceOnsetMS: 30, TargetOnsetMS: 20, TargetFixedMS: 55, TargetJoinMS: 30, ProtectStop: true, VowelJoin: true}
	job := provider.UnitRendererJob{Version: provider.UnitRendererJobVersion, Contract: "unit-renderer", ContractVersion: 1,
		Options: provider.UnitRendererOptions{Worldline: &provider.WorldlineOptions{Engine: "utautts-world-phrase", Units: []provider.WorldlineUnit{{Speech: speech}}}}}
	data, _ := json.Marshal(job)
	in, err := decodeProviderJob(data, "out.wav")
	if err != nil || !reflect.DeepEqual(in.Units[0].Speech, speech) {
		t.Fatal("speech wire", in, err)
	}
}

func TestWorldSpeechMixDoesNotChangeCachedFeatures(t *testing.T) {
	source := speechTestFeatures()
	before, _ := json.Marshal(source)
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{features: source, duration: 70}}}
	u := unit{LengthMS: 100, RequiredLengthMS: 100, ConsonantMS: 30, ConsonantVelocity: 100, Volume: 100, Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 10, TargetOnsetMS: 10}}
	in := manifest{Units: []unit{u}, F0Curve: []float64{220, 230, 240, 250, 260, 270, 280}}
	_ = mixWorldFeatures(in, prepared, 2, 1)
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("cached features mutated")
	}
	in.Units[0].Speech = nil
	baseline := mixWorldFeatures(in, prepared, 2, 1)
	if !reflect.DeepEqual(baseline.F0, in.F0Curve) {
		t.Fatal("target F0 changed")
	}
}
