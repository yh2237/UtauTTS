package worldline

import (
	"testing"

	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func TestTimingWarpJobCountsCVVCTransitionAsConsonant(t *testing.T) {
	synthesisPlan := &plan.Plan{Language: "ja", LeadingMarginMS: 20, Units: []plan.Unit{
		{Position: 0, Role: "mora", Mora: "あ", NoteStartMS: 0, DurationMS: 120},
		{Position: 1, Role: "transition", Mora: "す", NoteStartMS: 120, DurationMS: 90, EffectivePreutteranceMS: 90},
		{Position: 1, Role: "mora", Mora: "す", NoteStartMS: 120, DurationMS: 120, EffectivePreutteranceMS: 72},
		{Position: 2, Role: "mora", Mora: "か", NoteStartMS: 240, DurationMS: 120, EffectivePreutteranceMS: 60},
		{Position: 3, Role: "mora", Mora: "", Silent: true, NoteStartMS: 360, DurationMS: 180},
	}}
	warp := timingWarpJob(synthesisPlan, base.Config{})
	if warp == nil || warp.Strength != 1 || warp.LeadingMarginMS != 20 || len(warp.Morae) != 3 {
		t.Fatalf("warp = %+v", warp)
	}
	if got := warp.Morae[1].ConsonantMS; got != 90 {
		t.Fatalf("CVVC consonant = %v, want the VC length 90", got)
	}
	if got := warp.Morae[2].ConsonantMS; got != 60 {
		t.Fatalf("CV consonant = %v, want 60", got)
	}
	off := false
	cfg := base.Config{}
	cfg.ProviderOptions.Worldline.TimingWarp = &off
	if timingWarpJob(synthesisPlan, cfg) != nil {
		t.Fatal("disabled timing warp produced a job")
	}
	if timingWarpJob(&plan.Plan{Language: "en", Units: synthesisPlan.Units}, base.Config{}) != nil {
		t.Fatal("English plan produced a timing warp job")
	}
}
