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
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	if !got.stretchAdapted || got.stretchLimitReason == "" {
		t.Fatalf("adaptation was not reported: %+v", got)
	}
	if got.consonantMS <= before.consonantMS {
		t.Fatalf("fixed was not extended: %+v", got)
	}
	sourceTail := unit.SpeechProfile.TrimmedLengthMS - unit.ConsonantMS
	targetTail := got.preutteranceMS + unit.DurationMS + 20 - got.consonantMS
	if targetTail/sourceTail > stretchAdaptMaxRatio+1e-9 {
		t.Fatalf("tail stretch %.3f exceeded the limit", targetTail/sourceTail)
	}
	// 総長は変えない。
	if got.preutteranceMS != before.preutteranceMS || got.scale != before.scale {
		t.Fatalf("total length changed: %+v", got)
	}
}

func TestAdaptStretchTimingLeavesNormalUnitsUnchanged(t *testing.T) {
	unit := stretchAdaptUnit()
	unit.SpeechProfile.TrimmedLengthMS = 400
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	if got.stretchAdapted || got.consonantMS != before.consonantMS {
		t.Fatalf("normal unit changed: %+v", got)
	}
}

func TestAdaptStretchTimingClampsConsonant(t *testing.T) {
	unit := stretchAdaptUnit()
	unit.SpeechProfile.TrimmedLengthMS = 101
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 1)
	targetTotal := before.preutteranceMS + unit.DurationMS + 20
	minimumTail := math.Max(stretchAdaptMinTailMS, targetTotal*stretchAdaptMinTailRatio)
	if maximumFixed := targetTotal - minimumTail; got.consonantMS > maximumFixed+1e-9 {
		t.Fatalf("fixed %.3f exceeded the vowel-tail clamp %.3f", got.consonantMS, maximumFixed)
	}
}

func TestAdaptStretchTimingScalesByStrength(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	full := adaptStretchTiming(unit, before, 20, true, 1)
	half := adaptStretchTiming(unit, before, 20, true, 0.5)
	if !(half.consonantMS > before.consonantMS && half.consonantMS < full.consonantMS) {
		t.Fatalf("half strength = %.3f, full = %.3f", half.consonantMS, full.consonantMS)
	}
}

func TestAdaptStretchTimingZeroStrengthUsesDefault(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	got := adaptStretchTiming(unit, before, 20, true, 0)
	full := adaptStretchTiming(unit, before, 20, true, 1)
	if got.consonantMS != full.consonantMS || got.stretchAdapted != full.stretchAdapted {
		t.Fatalf("zero strength = %+v, want %+v", got, full)
	}
}

func TestAdaptStretchTimingIdentityCases(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}

	disabled := adaptStretchTiming(unit, before, 20, false, 1)
	if disabled.stretchAdapted || disabled.consonantMS != before.consonantMS {
		t.Fatalf("disabled = %+v", disabled)
	}
	negative := adaptStretchTiming(unit, before, 20, true, -1)
	if negative.stretchAdapted || negative.consonantMS != before.consonantMS {
		t.Fatalf("negative strength = %+v", negative)
	}

	noProfile := unit
	noProfile.SpeechProfile = nil
	if got := adaptStretchTiming(noProfile, before, 20, true, 1); got.stretchAdapted {
		t.Fatalf("nil profile was adapted: %+v", got)
	}
	notApplied := unit
	notApplied.SpeechProfile = &voicebank.SpeechProfile{TrimmedLengthMS: 200}
	if got := adaptStretchTiming(notApplied, before, 20, true, 1); got.stretchAdapted {
		t.Fatalf("unapplied profile was adapted: %+v", got)
	}
	silent := unit
	silent.Silent = true
	if got := adaptStretchTiming(silent, before, 20, true, 1); got.stretchAdapted {
		t.Fatalf("silent unit was adapted: %+v", got)
	}
	ending := unit
	ending.Role = "ending"
	if got := adaptStretchTiming(ending, before, 20, true, 1); got.stretchAdapted {
		t.Fatalf("ending unit was adapted: %+v", got)
	}
}

func TestAdaptStretchTimingStrengthLimit(t *testing.T) {
	unit := stretchAdaptUnit()
	before := effectiveTiming{preutteranceMS: 50, consonantMS: 100, overlapMS: 20, scale: 1}
	capped := adaptStretchTiming(unit, before, 20, true, stretchAdaptStrengthLimit)
	over := adaptStretchTiming(unit, before, 20, true, stretchAdaptStrengthLimit+5)
	if capped.consonantMS != over.consonantMS || capped.stretchAdapted != over.stretchAdapted {
		t.Fatalf("strength was not capped: %+v vs %+v", capped, over)
	}
}
