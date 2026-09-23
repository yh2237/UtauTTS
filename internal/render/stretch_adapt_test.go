package render

import (
	"math"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func stretchAdaptUnit() plan.Unit {
	return plan.Unit{
		Role: "mora", Position: 0, DurationMS: 200,
		PreutteranceMS: 50, ConsonantMS: 100,
		SpeechProfile: &voicebank.SpeechProfile{Applied: true, TrimmedLengthMS: 200},
	}
}

func TestAdaptStretchTimingBoundsExcessiveStretch(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	if !got.StretchAdapted || got.StretchLimitReason == "" {
		t.Fatalf("adaptation was not reported: %+v", got)
	}
	if got.ConsonantMS <= before.ConsonantMS {
		t.Fatalf("fixed was not extended: %+v", got)
	}
	sourceTail := unit.SpeechProfile.TrimmedLengthMS - unit.ConsonantMS
	targetTail := got.PreutteranceMS + unit.DurationMS + 20 - got.ConsonantMS
	if targetTail/sourceTail > stretchAdaptMaxRatio+1e-9 {
		t.Fatalf("tail stretch %.3f exceeded the limit", targetTail/sourceTail)
	}
	// 総長は変えない。
	if got.PreutteranceMS != before.PreutteranceMS || got.Scale != before.Scale {
		t.Fatalf("total length changed: %+v", got)
	}
}

func TestAdaptStretchTimingLeavesNormalUnitsUnchanged(t *testing.T) {
	unit := stretchAdaptUnit()
	unit.SpeechProfile.TrimmedLengthMS = 400
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	if got.StretchAdapted || got.ConsonantMS != before.ConsonantMS {
		t.Fatalf("normal unit changed: %+v", got)
	}
}

func TestAdaptStretchTimingClampsConsonant(t *testing.T) {
	unit := stretchAdaptUnit()
	unit.SpeechProfile.TrimmedLengthMS = 101
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	targetTotal := before.PreutteranceMS + unit.DurationMS + 20
	minimumTail := math.Max(stretchAdaptMinTailMS, targetTotal*stretchAdaptMinTailRatio)
	if maximumFixed := targetTotal - minimumTail; got.ConsonantMS > maximumFixed+1e-9 {
		t.Fatalf("fixed %.3f exceeded the vowel-tail clamp %.3f", got.ConsonantMS, maximumFixed)
	}
}

func TestAdaptStretchTimingScalesByStrength(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	full := adaptStretchTiming(unit, before, 20, true, 1)
	half := adaptStretchTiming(unit, before, 20, true, 0.5)
	if !(half.ConsonantMS > before.ConsonantMS && half.ConsonantMS < full.ConsonantMS) {
		t.Fatalf("half strength = %.3f, full = %.3f", half.ConsonantMS, full.ConsonantMS)
	}
}

func TestAdaptStretchTimingZeroStrengthUsesDefault(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 0)
	full := adaptStretchTiming(unit, before, 20, true, 1)
	if got.ConsonantMS != full.ConsonantMS || got.StretchAdapted != full.StretchAdapted {
		t.Fatalf("zero strength = %+v, want %+v", got, full)
	}
}

func TestAdaptStretchTimingIdentityCases(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}

	disabled := adaptStretchTiming(unit, before, 20, false, 1)
	if disabled.StretchAdapted || disabled.ConsonantMS != before.ConsonantMS {
		t.Fatalf("disabled = %+v", disabled)
	}
	negative := adaptStretchTiming(unit, before, 20, true, -1)
	if negative.StretchAdapted || negative.ConsonantMS != before.ConsonantMS {
		t.Fatalf("negative strength = %+v", negative)
	}

	noProfile := unit
	noProfile.SpeechProfile = nil
	if got := adaptStretchTiming(noProfile, before, 20, true, 1); got.StretchAdapted {
		t.Fatalf("nil profile was adapted: %+v", got)
	}
	notApplied := unit
	notApplied.SpeechProfile = &voicebank.SpeechProfile{TrimmedLengthMS: 200}
	if got := adaptStretchTiming(notApplied, before, 20, true, 1); got.StretchAdapted {
		t.Fatalf("unapplied profile was adapted: %+v", got)
	}
	silent := unit
	silent.Silent = true
	if got := adaptStretchTiming(silent, before, 20, true, 1); got.StretchAdapted {
		t.Fatalf("silent unit was adapted: %+v", got)
	}
	ending := unit
	ending.Role = "ending"
	if got := adaptStretchTiming(ending, before, 20, true, 1); got.StretchAdapted {
		t.Fatalf("ending unit was adapted: %+v", got)
	}
}

func TestAdaptStretchTimingStrengthLimit(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{PreutteranceMS: 50, ConsonantMS: 100, OverlapMS: 20, Scale: 1}
	capped := adaptStretchTiming(unit, before, 20, true, stretchAdaptStrengthLimit)
	over := adaptStretchTiming(unit, before, 20, true, stretchAdaptStrengthLimit+5)
	if capped.ConsonantMS != over.ConsonantMS || capped.StretchAdapted != over.StretchAdapted {
		t.Fatalf("strength was not capped: %+v vs %+v", capped, over)
	}
}
