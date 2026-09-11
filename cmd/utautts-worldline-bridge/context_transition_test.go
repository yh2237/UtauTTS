package main

import (
	"math"
	"testing"
	"utautts/internal/provider"
)

func TestContextTransitionPreservesMotionAndAnchors(t *testing.T) {
	for _, length := range []float64{140, 240, 600} {
		u := unit{OffsetMS: 13, ConsonantMS: 100, RequiredLengthMS: length, Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 60, TargetOnsetMS: 40, ProtectTransition: true}}
		a, ok := worldSpeechAnchors(u, 400)
		if !ok || !a.transitionProtected {
			t.Fatal(a, ok)
		}
		for tm := a.targetOnset - a.protected; tm <= a.targetFixed; tm += .5 {
			if math.Abs(a.sourceTime(tm)-(a.sourceOnset+tm-a.targetOnset)) > 1e-9 {
				t.Fatal("transition stretched", tm, a)
			}
		}
		if a.sourceTime(0) != 0 || a.sourceTime(length) != 400 || a.sourceTime(40) != 63 {
			t.Fatal("anchors moved", a)
		}
		last := -1.0
		for tm := 0.0; tm <= length; tm += .25 {
			v := a.sourceTime(tm)
			if v < last || v > 400 {
				t.Fatal("invalid mapping", tm, v)
			}
			last = v
		}
		u.RequiredLengthMS = 90
		b, ok := worldSpeechAnchors(u, 400)
		if !ok || b.transitionProtected {
			t.Fatal("short unit not falling back", b)
		}
		u.Speech.ProtectTransition = false
		c, _ := worldSpeechAnchors(u, 400)
		if b != c {
			t.Fatal("fallback changed baseline")
		}
	}
}
