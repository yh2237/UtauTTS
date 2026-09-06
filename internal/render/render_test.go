package render

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/pitch"
	"utautts/internal/plan"
)

func TestRenderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Render(nil, Config{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestWorldlineRFaithfulRendererIsRegistered(t *testing.T) {
	const backend = "openutau-worldline-r-faithful"
	if !IsKnownRenderer(backend) {
		t.Fatalf("WORLDLINE-R faithful renderer %q is not registered", backend)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestRenderIsDeterministicAndUsesAbsolutePlacement(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 300)
	for i := range data {
		data[i] = 10000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 200, Units: []plan.Unit{{
		Alias: "あ", Source: path, NoteStartMS: 100, DurationMS: 100,
		PreutteranceMS: 100, OverlapMS: 0,
	}}}
	first, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("render is not deterministic")
	}
	if len(first.Data) != 220 {
		t.Fatalf("frames = %d, want 220", len(first.Data))
	}
	if first.Data[0] != 0 || first.Data[100] == 0 || first.Data[len(first.Data)-1] != 0 {
		t.Fatalf("unexpected envelope: first=%d middle=%d last=%d", first.Data[0], first.Data[100], first.Data[len(first.Data)-1])
	}
}

func TestMinimumHandoffIsComplementaryWhenPreutteranceEqualsOverlap(t *testing.T) {
	timings := []effectiveTiming{{}, {preutteranceMS: 8, overlapMS: 8}}
	p := &plan.Plan{Units: []plan.Unit{{Position: 0}, {Position: 1, NoteStartMS: 100}}}
	const sampleRate = 1000
	start := 92
	for frame := start; frame <= start+6; frame++ {
		previous := handoffGain(frame, 0, p, timings, sampleRate)
		next := envelope(frame-start, 100, msToFrames(fadeInDurationMS(timings[1]), sampleRate), 0)
		if math.Abs(previous+next-1) > 1e-9 {
			t.Fatalf("non-complementary handoff at %d: previous=%f next=%f", frame, previous, next)
		}
	}
}

func TestCVVCHandoffCoversMoraToTransitionAndTransitionToMora(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 100},
		{Position: 1, Role: "transition", NoteStartMS: 100, DurationMS: 30, PreutteranceMS: 30, OverlapMS: 0},
		{Position: 1, Role: "mora", NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	if !unitsShareHandoff(units[0], units[1]) || !unitsShareHandoff(units[1], units[2]) {
		t.Fatal("CVVC units did not form both handoff boundaries")
	}
	if unitsShareHandoff(units[0], plan.Unit{Position: 2, Role: "mora"}) {
		t.Fatal("non-adjacent CVVC units were treated as one handoff")
	}
}

func TestRenderCVVCPlanWithTransitionUnit(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 400)
	for i := range data {
		data[i] = int16(6000 * math.Sin(2*math.Pi*120*float64(i)/1000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 200, Units: []plan.Unit{
		{Position: 0, Role: "mora", Alias: "あ", Source: path, NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 20},
		{Position: 1, Role: "transition", Alias: "a k", Source: path, NoteStartMS: 100, DurationMS: 30, PreutteranceMS: 50, OverlapMS: 20, ConsonantMS: 20},
		{Position: 1, Role: "mora", Alias: "か", Source: path, NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}}
	pcm, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm.Data) < 220 {
		t.Fatalf("CVVC render frames = %d, want at least 220", len(pcm.Data))
	}
}

func TestFadeInDurationKeepsConfiguredLongCrossfade(t *testing.T) {
	if got := fadeInDurationMS(effectiveTiming{preutteranceMS: 60, overlapMS: 20}); got != 40 {
		t.Fatalf("fade-in duration=%f, want 40", got)
	}
}

func TestRenderRejectsUnknownBackend(t *testing.T) {
	_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{Backend: "missing"})
	if err == nil {
		t.Fatal("unknown backend was accepted")
	}
}

func TestPitchCurveFactorInterpolatesInCents(t *testing.T) {
	curve := &PitchCurve{FrameMS: 10, Cents: []float64{0, 100}}
	want := math.Pow(2, 50.0/1200)
	if got := pitchCurveFactorAt(curve, 5); math.Abs(got-want) > 1e-12 {
		t.Fatalf("factor at midpoint = %.12f, want %.12f", got, want)
	}
	if got, end := pitchCurveFactorAt(curve, 100), math.Pow(2, 100.0/1200); math.Abs(got-end) > 1e-12 {
		t.Fatalf("factor after end = %.12f, want %.12f", got, end)
	}
}

func TestEffectiveUnitPitchFactorRequiresPitchProcessing(t *testing.T) {
	unit := plan.Unit{PitchFactor: 1.25}
	if got := effectiveUnitPitchFactor(unit, false); got != 1 {
		t.Fatalf("disabled pitch processing kept unit factor %.2f", got)
	}
	if got := effectiveUnitPitchFactor(unit, true); got != 1.25 {
		t.Fatalf("enabled pitch processing changed unit factor to %.2f", got)
	}
}

func TestSmoothAndLimitPitchCurveDoesNotMutateAndLimitsSlope(t *testing.T) {
	source := &PitchCurve{FrameMS: 10, Cents: []float64{0, 80, -80, 80, 0}}
	result := smoothAndLimitPitchCurve(source, 20, 4)
	if source.Cents[1] != 80 || result == source {
		t.Fatal("pitch curve smoothing mutated its input")
	}
	for index := 1; index < len(result.Cents); index++ {
		if math.Abs(result.Cents[index]-result.Cents[index-1]) > 4.000001 {
			t.Fatalf("step %d is too steep: %f -> %f", index, result.Cents[index-1], result.Cents[index])
		}
	}
}

func TestOpenUtauEnvelopeUsesNextPhoneTailTiming(t *testing.T) {
	units := []plan.Unit{
		{NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	timings, phraseStart := openUtauPhoneTimings(units, CVVCTimingLegacy)
	if timings[0].tailIntrude != 40 || timings[0].tailOverlap != 10 || !timings[1].overlapped || phraseStart != -30 {
		t.Fatalf("timing = %+v %+v phraseStart=%.1f", timings[0], timings[1], phraseStart)
	}
	envelope := openUtauEnvelopeFromTiming(units[0], timings[0])
	wantX := []float64{-30, -25, 0, 60, 70}
	for index, want := range wantX {
		if envelope[index].XMS != want {
			t.Fatalf("envelope point %d x = %.1f, want %.1f", index, envelope[index].XMS, want)
		}
	}
}

func TestLimitLeadingPreutterance(t *testing.T) {
	if got := limitLeadingPreutterance(120, 0); got != 120 {
		t.Fatalf("automatic leading preutterance = %.1f, want 120", got)
	}
	if got := limitLeadingPreutterance(120, 45); got != 45 {
		t.Fatalf("limited leading preutterance = %.1f, want 45", got)
	}
	if got := limitLeadingPreutterance(80, 120); got != 80 {
		t.Fatalf("short leading preutterance = %.1f, want 80", got)
	}
}

func TestOpenUtauPhoneTimingsKeepMoraTimingAcrossCVVCTransition(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{Position: 1, Role: "transition", NoteStartMS: 100, DurationMS: 30, PreutteranceMS: 50, OverlapMS: 20},
		{Position: 1, Role: "mora", NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	timings, phraseStart := openUtauPhoneTimings(units, CVVCTimingLegacy)
	if timings[1].preutter != 50 || timings[1].overlap != 20 {
		t.Fatalf("transition timing = %+v", timings[1])
	}
	if timings[2].preutter != 40 || timings[2].overlap != 10 || !timings[2].overlapped {
		t.Fatalf("main timing = %+v", timings[2])
	}
	if timings[0].tailIntrude != 50 || timings[0].tailOverlap != 20 || phraseStart != -30 {
		t.Fatalf("mora tail timing = %+v phraseStart=%.1f", timings[0], phraseStart)
	}
}

func TestOpenUtauSequentialCVVCTimingChainsTransitionAndMainPhone(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{Position: 1, Role: "transition", NoteStartMS: 100, DurationMS: 30, PreutteranceMS: 50, OverlapMS: 20},
		{Position: 1, Role: "mora", NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	timings, phraseStart := openUtauPhoneTimings(units, CVVCTimingSequential)
	if timings[0].tailIntrude != 50 || timings[0].tailOverlap != 20 {
		t.Fatalf("previous mora tail timing = %+v", timings[0])
	}
	if timings[1].tailIntrude != 20 || timings[1].tailOverlap != 5 {
		t.Fatalf("transition tail timing = %+v", timings[1])
	}
	if timings[2].preutter != 20 || timings[2].overlap != 5 || !timings[2].overlapped {
		t.Fatalf("main timing = %+v", timings[2])
	}
	if phraseStart != -30 {
		t.Fatalf("phrase start = %.1f", phraseStart)
	}
}

func TestOpenUtauSequentialTimingDoesNotChangeNonCVVCSequence(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 100, PreutteranceMS: 30, OverlapMS: 5},
		{Position: 1, Role: "mora", NoteStartMS: 100, DurationMS: 100, PreutteranceMS: 40, OverlapMS: 10},
	}
	legacy, legacyStart := openUtauPhoneTimings(units, CVVCTimingLegacy)
	sequential, sequentialStart := openUtauPhoneTimings(units, CVVCTimingSequential)
	if !reflect.DeepEqual(legacy, sequential) || legacyStart != sequentialStart {
		t.Fatalf("non-CVVC timing changed: legacy=%+v sequential=%+v", legacy, sequential)
	}
}

func TestWorldlineRejectsUnknownCVVCTimingBeforeResolvingAssets(t *testing.T) {
	_, err := renderWorldlineEngine(&plan.Plan{Units: []plan.Unit{{Role: "mora"}}}, Config{CVVCTiming: "unknown"}, "worldline-r-faithful")
	if err == nil || !strings.Contains(err.Error(), "unknown CVVC timing mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCVVCPreBoundaryEnvelopeEndsAtFollowingMoraBoundary(t *testing.T) {
	points := []worldlineEnvelopePoint{
		{XMS: -80, Y: 0}, {XMS: -50, Y: 1}, {XMS: 0, Y: 1},
		{XMS: 10, Y: 1}, {XMS: 30, Y: 0},
	}
	got := cvvcPreBoundaryEnvelope(points, openUtauPhoneTiming{tailOverlap: 20})
	if got[2].XMS != -20 || got[3].XMS != -20 || got[4].XMS != 0 {
		t.Fatalf("unexpected pre-boundary envelope: %+v", got)
	}
	if points[4].XMS != 30 {
		t.Fatal("input envelope was mutated")
	}
}

func TestWaveformRendererAcceptsFramePitchCurve(t *testing.T) {
	_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{Backend: "waveform", ApplyPitch: true, PitchCurve: &PitchCurve{FrameMS: 5, Cents: []float64{0}}})
	if err == nil {
		t.Fatal("empty plan unit was accepted")
	}
	if strings.Contains(err.Error(), "frame pitch curve is not supported by waveform renderer") {
		t.Fatalf("waveform still rejects frame pitch curves: %v", err)
	}
}

func TestResampleForPitchCurveFlatMatchesConstantResample(t *testing.T) {
	source := make([]float64, 256)
	for i := range source {
		source[i] = math.Sin(float64(i) * 0.1)
	}
	flat := &PitchCurve{FrameMS: 10, Cents: []float64{100, 100}}
	factor := 1.2 * math.Pow(2, 100.0/1200)
	varying := resampleForPitchCurve(source, 1.2, flat, 0, 256)
	if got := len(varying); got != max(16, int(math.Round(256/factor))) {
		t.Fatalf("length %d, want %d", got, int(math.Round(256/factor)))
	}
	for k := range varying {
		position := math.Min(float64(len(source)-1), float64(k)*factor)
		left := int(math.Floor(position))
		fraction := position - float64(left)
		want := source[left] + (source[min(left+1, len(source)-1)]-source[left])*fraction
		if math.Abs(varying[k]-want) > 1e-9 {
			t.Fatalf("sample %d = %f, want %f", k, varying[k], want)
		}
	}
}

func TestResampleForPitchCurveStretchesUnevenly(t *testing.T) {
	source := make([]float64, 512)
	for i := range source {
		source[i] = float64(i) / 511
	}
	curve := &PitchCurve{FrameMS: 10, Cents: []float64{0, 100}}
	result := resampleForPitchCurve(source, 1, curve, 0, 256)
	flat := resampleForPitch(source, math.Pow(2, 100.0/1200))
	if len(result) <= len(flat) {
		t.Fatalf("raising pitch should stretch beyond the flat resample, got %d vs %d", len(result), len(flat))
	}
	if result[0] != 0 || result[len(result)-1] != 1 {
		t.Fatalf("ramp endpoints not preserved: %f .. %f", result[0], result[len(result)-1])
	}
	lagged := false
	for index := 1; index < len(flat); index++ {
		if result[index] < flat[index]-1e-9 {
			lagged = true
			break
		}
	}
	if !lagged {
		t.Fatal("curved resample never lags the flat resample in the stretched region")
	}
}

func TestRenderRejectsNonFinitePitchCurve(t *testing.T) {
	for _, curve := range []*PitchCurve{
		{FrameMS: math.NaN(), Cents: []float64{0}},
		{FrameMS: 5, Cents: []float64{math.Inf(1)}},
	} {
		_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{Backend: "openutau-worldline-r-faithful", ApplyPitch: true, PitchCurve: curve})
		if err == nil {
			t.Fatalf("accepted non-finite curve: %+v", curve)
		}
	}
}

func TestRenderRejectsNonFiniteTimingConfiguration(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{ReleaseMS: value})
		if err == nil {
			t.Fatalf("accepted non-finite release_ms %v", value)
		}
	}
}

func TestRenderReleaseZeroIsHonoredWhenExplicit(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 300)
	for i := range data {
		data[i] = 10000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 200, Units: []plan.Unit{{
		Alias: "あ", Source: path, NoteStartMS: 100, DurationMS: 100,
		PreutteranceMS: 100, OverlapMS: 0,
	}}}
	implicit, err := Render(p, Config{ReleaseMS: 0})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := Render(p, Config{ReleaseMS: 0, ReleaseSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(implicit.Data) != 220 {
		t.Fatalf("unset release frames = %d, want 220", len(implicit.Data))
	}
	if len(explicit.Data) != 200 {
		t.Fatalf("explicit zero release frames = %d, want 200", len(explicit.Data))
	}
}

func TestWaveformRendererPreservesFirstUnitPreutterance(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 180)
	for index := 0; index < 40; index++ {
		data[index] = 10000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 100, Units: []plan.Unit{{
		Alias: "に", Source: path, NoteStartMS: 0, DurationMS: 100,
		PreutteranceMS: 40, OverlapMS: 0, ConsonantMS: 40,
	}}}
	pcm, err := Render(p, Config{Backend: "waveform", ReleaseMS: 0, ReleaseSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm.Data) != 140 {
		t.Fatalf("output frames = %d, want 140 including preutterance", len(pcm.Data))
	}
	peak := int16(0)
	for _, sample := range pcm.Data[:40] {
		if sample > peak {
			peak = sample
		}
	}
	if peak < 1000 {
		t.Fatalf("first-unit preutterance was lost: peak=%d", peak)
	}
}

func TestRenderRejectsUnsafePitchCurveRangeAndFrame(t *testing.T) {
	for _, curve := range []*PitchCurve{
		{FrameMS: 0.01, Cents: []float64{0}},
		{FrameMS: 5, Cents: []float64{4801}},
		{FrameMS: 5, Cents: []float64{-4801}},
	} {
		_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{Backend: "openutau-worldline-r-faithful", ApplyPitch: true, PitchCurve: curve})
		if err == nil {
			t.Fatalf("accepted unsafe curve: %+v", curve)
		}
	}
}

func TestBoundaryBridgeRequiresWaveformRenderer(t *testing.T) {
	_, err := Render(&plan.Plan{Units: []plan.Unit{{}}}, Config{Backend: "openutau-worldline-r-faithful", BoundaryBridgeMS: 20})
	if err == nil {
		t.Fatal("boundary bridge was accepted by non-waveform renderer")
	}
}

func TestRenderAllowsSilentClosureUnit(t *testing.T) {
	path := t.TempDir() + "/unit.wav"
	data := make([]int16, 200)
	for index := range data {
		data[index] = 8000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 265, Units: []plan.Unit{
		{Position: 0, Alias: "あ", Source: path, NoteStartMS: 0, DurationMS: 100},
		{Position: 1, Alias: "<closure>", Silent: true, NoteStartMS: 100, DurationMS: 65},
		{Position: 2, Alias: "か", Source: path, NoteStartMS: 165, DurationMS: 100},
	}}
	pcm, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if pcm.Data[140] != 0 {
		t.Fatalf("closure midpoint=%d, want silence", pcm.Data[140])
	}
}

func TestWorldlineF0CurveInterpolatesInLogFrequency(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0Curve(p, []float64{200, 400}, []float64{1, 1}, 220, 11)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[10]-400) > 0.01 {
		t.Fatalf("curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
	if math.Abs(curve[5]-math.Sqrt(200*400)) > 0.1 {
		t.Fatalf("log midpoint = %.2f", curve[5])
	}
}

func TestWorldlineF0CurveOffsetIncludesPhraseLeading(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0CurveAtOffset(p, []float64{200, 400}, []float64{1, 1}, 220, 13, 10, -20)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[2]-200) > 0.01 {
		t.Fatalf("leading frames = %.2f, %.2f; want 200Hz", curve[0], curve[2])
	}
	if math.Abs(curve[12]-400) > 0.01 {
		t.Fatalf("second unit at shifted frame = %.2f, want 400Hz", curve[12])
	}
}

func TestWorldlineF0CurveAppliesLearnedPitchFactors(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0Curve(p, []float64{200, 200}, []float64{1.03, 0.97}, 200, 11)
	if math.Abs(curve[0]-206) > 0.01 || math.Abs(curve[10]-194) > 0.01 {
		t.Fatalf("factored curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
}

func TestNormalizeTimingCompressesLongVCVAndKeepsVowelTail(t *testing.T) {
	unit := plan.Unit{DurationMS: 140, PreutteranceMS: 360, OverlapMS: 120, ConsonantMS: 439}
	got := normalizeTiming(unit, 20)
	if math.Abs(got.preutteranceMS-105) > 0.001 {
		t.Fatalf("preutterance = %.3f, want 105", got.preutteranceMS)
	}
	if math.Abs(got.overlapMS-35) > 0.001 {
		t.Fatalf("overlap = %.3f, want 35", got.overlapMS)
	}
	if got.consonantMS >= got.preutteranceMS+unit.DurationMS+20-(20+49) {
		t.Fatalf("consonant %.3f leaves no guaranteed vowel tail", got.consonantMS)
	}
}

func TestNormalizeTimingLeavesOrdinaryBankAlone(t *testing.T) {
	unit := plan.Unit{DurationMS: 140, PreutteranceMS: 60, OverlapMS: 20, ConsonantMS: 100}
	got := normalizeTiming(unit, 20)
	if got.preutteranceMS != 60 || got.overlapMS != 20 || got.consonantMS != 100 || got.scale != 1 {
		t.Fatalf("ordinary timing changed: %#v", got)
	}
}

func TestRetimeCompressedPrefixRetainsVowelTail(t *testing.T) {
	source := make([]float64, 600)
	for i := 439; i < len(source); i++ {
		source[i] = 1
	}
	got, err := retimeWithCompressedPrefixUsing(source, 265, 439, 128, 1000, wsolaStretch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 265 {
		t.Fatalf("length = %d", len(got))
	}
	voiced := 0
	for _, value := range got[128:] {
		if value > 0.5 {
			voiced++
		}
	}
	if voiced < 100 {
		t.Fatalf("vowel tail only has %d frames", voiced)
	}
	maxDelta := 0.0
	maxDeltaAt := 0
	for i := 1; i < len(got); i++ {
		if delta := math.Abs(got[i] - got[i-1]); delta > maxDelta {
			maxDelta, maxDeltaAt = delta, i
		}
	}
	if maxDelta > 0.25 {
		t.Fatalf("compressed-prefix join clicks: max delta %.3f at %d", maxDelta, maxDeltaAt)
	}
}

func TestRenderLongVCVUsesWeightedCrossfade(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/vcv.wav"
	data := make([]int16, 800)
	for i := range data {
		data[i] = 10000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 280, Units: []plan.Unit{
		{Alias: "a", Source: path, NoteStartMS: 0, DurationMS: 140, PreutteranceMS: 360, OverlapMS: 120, ConsonantMS: 439},
		{Alias: "b", Source: path, NoteStartMS: 140, DurationMS: 140, PreutteranceMS: 360, OverlapMS: 120, ConsonantMS: 439},
	}}
	pcm, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	peak := int16(0)
	for _, value := range pcm.Data {
		if value > peak {
			peak = value
		}
	}
	if peak > 10010 {
		t.Fatalf("overlap was additively mixed: peak=%d", peak)
	}
	if p.Units[1].EffectivePreutteranceMS != 105 || p.Units[1].EffectiveOverlapMS != 35 {
		t.Fatalf("effective timing not recorded: %#v", p.Units[1])
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
	exportPlan := plan.Clone(input)
	result.Report.ApplyTo(exportPlan)
	if exportPlan.LeadingMarginMS == 0 || exportPlan.Units[0].EffectivePreutteranceMS == 0 {
		t.Fatalf("report did not retain renderer diagnostics: %#v", result.Report)
	}
}

func TestConnectedUnitsUseComplementaryHandoff(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/unit.wav"
	data := make([]int16, 400)
	for i := range data {
		data[i] = 10000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 280, Units: []plan.Unit{
		{Position: 0, Alias: "a", Source: path, NoteStartMS: 0, DurationMS: 140, PreutteranceMS: 60, OverlapMS: 20, ConsonantMS: 100},
		{Position: 1, Alias: "b", Source: path, NoteStartMS: 140, DurationMS: 140, PreutteranceMS: 60, OverlapMS: 20, ConsonantMS: 100},
	}}
	pcm, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	peak := int16(0)
	for _, value := range pcm.Data {
		if value > peak {
			peak = value
		}
	}
	if peak > 10010 {
		t.Fatalf("connected units were layered: peak=%d", peak)
	}
	for frame := 85; frame < 120; frame++ {
		if pcm.Data[frame] < 9500 {
			t.Fatalf("handoff dipped at frame %d: %d", frame, pcm.Data[frame])
		}
	}
}

func TestAnalyzeIntonationMeasuresAndLimitsCorrection(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	for index, hz := range []float64{200, 220, 240} {
		paths[index] = dir + "/tone" + string(rune('0'+index)) + ".wav"
		data := make([]int16, 8000)
		for i := range data {
			data[i] = int16(6000 * math.Sin(2*math.Pi*hz*float64(i)/16000))
		}
		if err := audio.WriteWav(paths[index], &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	p := &plan.Plan{Units: []plan.Unit{
		{Position: 0, Source: paths[0]}, {Position: 1, Source: paths[1]}, {Position: 2, Source: paths[2]},
	}}
	timings := []effectiveTiming{{scale: 1}, {scale: 1}, {scale: 1}}
	factors := analyzeIntonation(p, timings, &sourceCache{}, 1)
	if len(factors) != 3 {
		t.Fatalf("factor count = %d", len(factors))
	}
	for i, factor := range factors {
		if factor < 0.92 || factor > 1.08 {
			t.Fatalf("factor %d out of bounds: %f", i, factor)
		}
		if p.Units[i].SourceF0Hz == 0 || p.Units[i].TargetF0Hz == 0 {
			t.Fatalf("missing F0 audit at %d: %#v", i, p.Units[i])
		}
	}
	if p.Units[2].TargetF0Hz >= p.Units[1].TargetF0Hz {
		t.Fatalf("phrase does not fall: %#v", p.Units)
	}
}

func TestAnalyzeIntonationSkipsCVVCTransitionInMoraContour(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	for index, hz := range []float64{200, 220, 240} {
		paths[index] = dir + "/cvvc-tone" + string(rune('0'+index)) + ".wav"
		data := make([]int16, 8000)
		for i := range data {
			data[i] = int16(6000 * math.Sin(2*math.Pi*hz*float64(i)/16000))
		}
		if err := audio.WriteWav(paths[index], &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	p := &plan.Plan{Units: []plan.Unit{
		{Position: 0, Role: "mora", Source: paths[0]},
		{Position: 1, Role: "transition", Source: paths[1]},
		{Position: 1, Role: "mora", Source: paths[1]},
		{Position: 2, Role: "mora", Source: paths[2]},
	}}
	factors := analyzeIntonation(p, make([]effectiveTiming, len(p.Units)), &sourceCache{}, 1)
	if factors[1] != 1 || p.Units[1].SourceF0Hz != 0 || p.Units[1].TargetF0Hz != 0 {
		t.Fatalf("transition received intonation: factor=%v unit=%#v", factors[1], p.Units[1])
	}
	if p.Units[2].SourceF0Hz == 0 || p.Units[2].TargetF0Hz == 0 || p.Units[3].TargetF0Hz == 0 {
		t.Fatalf("mora contour omitted a main unit: %#v", p.Units)
	}
}

func TestSourceCacheReusesMonoAndNormalizedAudio(t *testing.T) {
	path := t.TempDir() + "/stereo.wav"
	data := []int16{1000, -1000, 2000, -2000, 3000, -3000, 4000, -4000}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 2, Data: data}); err != nil {
		t.Fatal(err)
	}

	cache := sourceCache{}
	first, err := cache.loadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.loadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("mono source was converted more than once")
	}
	if first.Channels != 1 || len(first.Data) != len(data)/2 {
		t.Fatalf("unexpected mono source: %#v", first)
	}

	native, err := cache.loadNormalized(path, 16000)
	if err != nil {
		t.Fatal(err)
	}
	resampled, err := cache.loadNormalized(path, 8000)
	if err != nil {
		t.Fatal(err)
	}
	resampledAgain, err := cache.loadNormalized(path, 8000)
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

func TestUnitPitchCacheIsReusedAndClearedWithWAVCache(t *testing.T) {
	ClearWAVCache()
	defer ClearWAVCache()
	path := t.TempDir() + "/tone.wav"
	data := make([]int16, 4000)
	for index := range data {
		data[index] = int16(6000 * math.Sin(2*math.Pi*220*float64(index)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	cache := newSourceCache()
	mono, err := cache.loadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	unit := plan.Unit{Source: path}
	first, err := estimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	second, err := estimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first <= 0 {
		t.Fatalf("cached pitch = %.3f, first = %.3f", second, first)
	}
	if len(globalUnitPitchCache.entries) != 1 {
		t.Fatalf("pitch cache entries = %d, want 1", len(globalUnitPitchCache.entries))
	}
	ClearWAVCache()
	if len(globalUnitPitchCache.entries) != 0 {
		t.Fatal("pitch cache was not cleared with the WAV cache")
	}
}

func TestAnalyzeIntonationAuditIncludesLearnedPitchFactor(t *testing.T) {
	path := t.TempDir() + "/tone.wav"
	data := make([]int16, 8000)
	for i := range data {
		data[i] = int16(6000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{Units: []plan.Unit{{Position: 0, Source: path, PitchFactor: 1.03}}}
	factors := analyzeIntonation(p, []effectiveTiming{{scale: 1}}, &sourceCache{}, 1)
	if math.Abs(p.Units[0].TargetF0Hz-p.Units[0].SourceF0Hz*factors[0]*1.03) > 0.1 {
		t.Fatalf("target F0=%f source=%f", p.Units[0].TargetF0Hz, p.Units[0].SourceF0Hz)
	}
}

func TestRenderPitchFactorRequiresExplicitMode(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tone.wav"
	data := make([]int16, 8000)
	for i := range data {
		data[i] = int16(6000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 300, Units: []plan.Unit{{Position: 0, Source: path, DurationMS: 300, PitchFactor: 1.05, EnergyFactor: 1}}}
	pcm, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	baseline := pitch.EstimateMedian(pcmFloats(pcm.Data[1600:4000]), pcm.SampleRate)
	if math.Abs(baseline-200) > 5 {
		t.Fatalf("default F0 = %.2f, want about 200", baseline)
	}
	pcm, err = Render(p, Config{ReleaseMS: 20, ApplyPitch: true})
	if err != nil {
		t.Fatal(err)
	}
	shifted := pitch.EstimateMedian(pcmFloats(pcm.Data[1600:4000]), pcm.SampleRate)
	if math.Abs(shifted-210) > 5 {
		t.Fatalf("explicitly shifted F0 = %.2f, want about 210", shifted)
	}
}

func TestStretchPreservesPrefixAndLength(t *testing.T) {
	source := make([]float64, 200)
	for i := range source {
		source[i] = float64(i) / 200
	}
	got, err := stretchPreservingPrefixUsing(source, 350, 50, 1000, wsolaStretch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 350 {
		t.Fatalf("length = %d", len(got))
	}
	if !reflect.DeepEqual(got[:50], source[:50]) {
		t.Fatal("protected prefix changed")
	}
}

func TestStretchWSOLAAnchoredUsesOneContinuousOutput(t *testing.T) {
	source := make([]float64, 1000)
	for index := range source {
		source[index] = 0.5 * math.Sin(2*math.Pi*float64(index)/50)
	}
	first := StretchWSOLAAnchored(source, 300, 1000, []int{0, 300, 700, 1000}, []int{0, 100, 200, 300})
	second := StretchWSOLAAnchored(source, 300, 1000, []int{0, 300, 700, 1000}, []int{0, 100, 200, 300})
	if len(first) != 300 {
		t.Fatalf("length = %d, want 300", len(first))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("anchored stretch is not deterministic")
	}
	energy := 0.0
	for _, value := range first {
		energy += value * value
	}
	if energy < 1 {
		t.Fatalf("output energy = %f", energy)
	}
}

func TestBridgeEnvelopeIsBoundedAndFadesAtEdges(t *testing.T) {
	if got := bridgeEnvelope(0, 9); got != 0 {
		t.Fatalf("start envelope = %f, want 0", got)
	}
	if got := bridgeEnvelope(8, 9); got != 0 {
		t.Fatalf("end envelope = %f, want 0", got)
	}
	peak := 0.0
	for frame := 0; frame < 9; frame++ {
		value := bridgeEnvelope(frame, 9)
		if value < 0 || value > 1 {
			t.Fatalf("envelope[%d] = %f, want [0, 1]", frame, value)
		}
		peak = math.Max(peak, value)
	}
	if peak < 0.99 {
		t.Fatalf("envelope peak = %f, want near 1", peak)
	}
}

func TestBestAlignedVowelSegmentFindsPhaseShift(t *testing.T) {
	unit := renderedUnit{
		unit:   plan.Unit{DurationMS: 80},
		timing: effectiveTiming{preutteranceMS: 20, consonantMS: 40},
		wave:   make([]float64, 120),
	}
	for index := range unit.wave {
		unit.wave[index] = math.Sin(0.013 * float64(index*index))
	}
	target := append([]float64(nil), unit.wave[75:95]...)
	got, lag, correlation := bestAlignedVowelSegment(unit, target, 20, 1000)
	if len(got) != 20 || lag != -5 || correlation < 0.999 {
		t.Fatalf("aligned segment len=%d lag=%d correlation=%f", len(got), lag, correlation)
	}
}

func TestStabilizeWorldlinePitchesCorrectsHarmonicJump(t *testing.T) {
	got := stabilizeWorldlinePitches([]float64{296, 446, 298})
	if math.Abs(got[1]-297.333333) > 2 {
		t.Fatalf("stabilized harmonic pitch = %.2f, want near 297", got[1])
	}
}

func TestStabilizeWorldlinePitchesKeepsLowerShortPhraseAnchor(t *testing.T) {
	got := stabilizeWorldlinePitches([]float64{296, 446})
	if math.Abs(got[0]-296) > 0.01 || math.Abs(got[1]-297.333333) > 2 {
		t.Fatalf("short phrase pitches = %#v, want near [296, 297]", got)
	}
}

func TestStabilizeWorldlinePitchesKeepsOrdinaryMovement(t *testing.T) {
	input := []float64{280, 296, 315}
	got := stabilizeWorldlinePitches(input)
	for index := range input {
		if got[index] != input[index] {
			t.Fatalf("ordinary pitch[%d] changed from %.2f to %.2f", index, input[index], got[index])
		}
	}
}

func TestChooseBoundaryRepairKeepsNormalOrImprovesPeak(t *testing.T) {
	const sampleRate = 1000
	mix := make([]float64, 220)
	weights := make([]float64, len(mix))
	previousWave := make([]float64, 120)
	for index := range mix {
		mix[index] = 0.2 * math.Sin(2*math.Pi*float64(index)/20)
		weights[index] = 1
	}
	for index := range previousWave {
		previousWave[index] = 0.2 * math.Sin(2*math.Pi*float64(index)/20)
	}
	// 境界のインパルスが減らなければ通常接続へ戻ることを確認する。
	mix[110] += 0.8
	previous := renderedUnit{
		unit: plan.Unit{DurationMS: 80}, timing: effectiveTiming{preutteranceMS: 20}, wave: previousWave,
	}
	current := renderedUnit{startFrame: 100, fadeInFrames: 20}
	choice := chooseBoundaryRepair(mix, weights, previous, current, 20, sampleRate)
	if !choice.applied {
		t.Fatal("clear transient did not select an improving repair")
	}
	if choice.selected.peak >= choice.baseline.peak {
		t.Fatalf("selected peak=%f baseline=%f", choice.selected.peak, choice.baseline.peak)
	}
}

func TestWaveformBoundaryBridgeIsOptionalAndAudited(t *testing.T) {
	dir := t.TempDir()
	paths := []string{dir + "/left.wav", dir + "/right.wav"}
	for fileIndex, frequency := range []float64{180, 260} {
		data := make([]int16, 6400)
		for frame := range data {
			data[frame] = int16(7000 * math.Sin(2*math.Pi*frequency*float64(frame)/16000))
		}
		if err := audio.WriteWav(paths[fileIndex], &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	base := &plan.Plan{DurationMS: 200, Units: []plan.Unit{
		{Position: 0, Mora: "a", Source: paths[0], NoteStartMS: 0, DurationMS: 100, ConsonantMS: 10, PreutteranceMS: 40},
		{Position: 1, Mora: "i", Source: paths[1], NoteStartMS: 100, DurationMS: 100, ConsonantMS: 10, PreutteranceMS: 40},
	}}
	if _, err := Render(base, Config{ReleaseMS: 20}); err != nil {
		t.Fatal(err)
	}
	if len(base.BoundaryBridges) != 0 || base.BoundaryBridgeMS != 0 {
		t.Fatalf("disabled bridge changed plan: %#v", base)
	}

	experiment := &plan.Plan{DurationMS: 200, Units: append([]plan.Unit(nil), base.Units...)}
	if _, err := Render(experiment, Config{ReleaseMS: 20, BoundaryBridgeMS: 20, BoundaryBridgeThreshold: 100}); err != nil {
		t.Fatal(err)
	}
	if len(experiment.BoundaryRepairDecisions) != 1 {
		t.Fatalf("repair decision count = %d, want 1", len(experiment.BoundaryRepairDecisions))
	}
	decision := experiment.BoundaryRepairDecisions[0]
	if decision.SelectedKind != "normal" && decision.SelectedKind != "phase-aligned-vowel-tail" {
		t.Fatalf("repair decision = %#v", decision)
	}
	if decision.Applied && len(experiment.BoundaryBridges) != 1 {
		t.Fatalf("applied decision has %d bridge records", len(experiment.BoundaryBridges))
	}
}
