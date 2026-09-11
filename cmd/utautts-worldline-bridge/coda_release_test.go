package main

import (
	"math"
	"testing"
	"utautts/internal/provider"
)

func TestCodaReleaseReachesBurstBeforeAudibleEnd(t *testing.T) {
	// cupの破裂音は583msの原音内の約408msにある。
	u := unit{ConsonantMS: 536, ConsonantVelocity: 100, RequiredLengthMS: 600, SkipMS: 68, LengthMS: 232,
		Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 250, TargetOnsetMS: 250, CodaRelease: true}}
	a, ok := worldSpeechAnchors(u, 583)
	if !ok {
		t.Fatal("valid coda rejected")
	}
	release := a.targetOnset + (408-a.sourceOnset)*(a.targetEnd-a.targetOnset)/(a.sourceEnd-a.sourceOnset)
	if release >= u.SkipMS+u.LengthMS || math.Abs(mapWorldSourceTime(u, 583, release-u.SkipMS)-408) > 1e-8 {
		t.Fatal("release outside audible output", release)
	}
	old := u
	old.Speech = nil
	if mapWorldSourceTime(old, 583, u.LengthMS) >= 408 {
		t.Fatal("fixture no longer reproduces truncation")
	}
	previous := -1.0
	for ms := 0.0; ms <= 300; ms += .25 {
		v := a.sourceTime(ms)
		if v < previous || v > 583 {
			t.Fatal("invalid mapping", ms, v)
		}
		previous = v
	}
	if a.sourceTime(250) != 250 || a.sourceTime(300) != 583 {
		t.Fatal("anchors moved")
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1)} {
		if _, ok := worldSpeechAnchors(u, bad); ok {
			t.Fatal("nonfinite duration accepted")
		}
	}
	u.LengthMS = 185
	if _, ok := worldSpeechAnchors(u, 583); ok {
		t.Fatal("insufficient coda duration accepted")
	}
	u.SkipMS = 0
	u.LengthMS = 60
	u.Speech.SourceOnsetMS = 0
	u.Speech.TargetOnsetMS = 0
	z, ok := worldSpeechAnchors(u, 200)
	if !ok || z.sourceTime(0) != 0 || z.sourceTime(60) != 200 {
		t.Fatal("zero-preutterance CC mapping", z, ok)
	}
}

func TestCodaReleasePreservesStopTransient(t *testing.T) {
	u := unit{SkipMS: 70, LengthMS: 60, Speech: &provider.WorldSpeechTiming{
		SourceOnsetMS: 100, TargetOnsetMS: 100, CodaRelease: true, ProtectStop: true,
	}}
	a, ok := worldSpeechAnchors(u, 300)
	if !ok || a.protected != 26 || a.sourceTime(120) != 120 || a.sourceTime(130) != 300 {
		t.Fatalf("anchors=%+v ok=%v", a, ok)
	}
}
