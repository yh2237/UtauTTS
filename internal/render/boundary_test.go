package render

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestSingleCVBoundaryProfilesProtectOnset(t *testing.T) {
	if got := singleCVBoundaryProfileForOnset("s"); got.widthMS != 8 || got.maxMix >= singleCVBoundaryMixSonorant {
		t.Fatalf("fricative profile = %+v", got)
	}
	if got := singleCVBoundaryProfileForOnset("m"); got.widthMS != 12 || got.maxMix != singleCVBoundaryMixSonorant {
		t.Fatalf("sonorant profile = %+v", got)
	}
	if got := singleCVBoundaryProfileForOnset(""); got.widthMS != 16 || got.maxMix != singleCVBoundaryMixVowel {
		t.Fatalf("vowel profile = %+v", got)
	}

	current := renderedUnit{startFrame: 100, fadeInFrames: 20}
	start, end := singleCVBoundaryWindow(current, 8, 200, 1000)
	if start != 112 || end != 120 {
		t.Fatalf("fricative window = [%d, %d), want [112, 120)", start, end)
	}
	start, end = singleCVBoundaryWindow(current, 16, 200, 1000)
	if start != 104 || end != 120 {
		t.Fatalf("vowel window = [%d, %d), want [104, 120)", start, end)
	}
}

func TestSingleCVBoundaryBridgeOnlyAppliesWhenItImproves(t *testing.T) {
	const sampleRate = 1000
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{
		{Vowel: "a"},
		{Vowel: "i"},
	}, Units: []plan.Unit{{Role: "mora", Position: 0}, {Role: "mora", Position: 1}}}
	previousWave := make([]float64, 120)
	mix := make([]float64, 220)
	weights := make([]float64, len(mix))
	for index := range previousWave {
		previousWave[index] = 0.2 * math.Sin(2*math.Pi*float64(index)/20)
	}
	for index := range mix {
		mix[index] = 0.2 * math.Sin(2*math.Pi*float64(index)/20)
		weights[index] = 1
	}
	mix[115] += 0.7
	untouched := append([]float64(nil), mix[:104]...)
	rendered := []renderedUnit{
		{index: 0, unit: plan.Unit{Role: "mora", Position: 0, DurationMS: 80}, timing: effectiveTiming{preutteranceMS: 20, consonantMS: 30}, wave: previousWave},
		{index: 1, unit: plan.Unit{Role: "mora", Position: 1}, startFrame: 100, fadeInFrames: 20},
	}
	applySingleCVBoundaryBridges(mix, weights, rendered, p, sampleRate, 0)
	if len(p.BoundaryBridges) != 1 || !p.Units[1].SpeechJoinApplied {
		t.Fatalf("boundary bridge was not applied: bridges=%d units=%+v", len(p.BoundaryBridges), p.Units)
	}
	if p.BoundaryBridges[0].StartMS != 104 || p.BoundaryBridges[0].EndMS != 120 {
		t.Fatalf("bridge range = [%f, %f), want [104, 120)", p.BoundaryBridges[0].StartMS, p.BoundaryBridges[0].EndMS)
	}
	for index, value := range untouched {
		if mix[index] != value {
			t.Fatalf("protected prefix changed at %d", index)
		}
	}

	baseline := transitionMeasure{peak: 1, deltaRMS: 1}
	if singleCVBoundaryImproves(baseline, transitionMeasure{peak: 1, deltaRMS: 1}) {
		t.Fatal("unchanged boundary was accepted")
	}
	if !singleCVBoundaryImproves(baseline, transitionMeasure{peak: 0.99, deltaRMS: 1.02}) {
		t.Fatal("small peak improvement was rejected")
	}
	if singleCVBoundaryImproves(baseline, transitionMeasure{peak: 1.01, deltaRMS: 0.9}) {
		t.Fatal("peak worsening was accepted")
	}
}
