package worldline

import (
	"math"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func TestTimingWarpJobBuildsPhraseTimeline(t *testing.T) {
	synthesisPlan := &plan.Plan{Language: "ja", LeadingMarginMS: 20, Units: []plan.Unit{
		{Position: 0, Role: "mora", Mora: "あ", NoteStartMS: 0, DurationMS: 120},
		{Position: 1, Role: "transition", Mora: "す", NoteStartMS: 120, DurationMS: 90, EffectivePreutteranceMS: 90},
		{Position: 1, Role: "mora", Mora: "す", NoteStartMS: 120, DurationMS: 120, EffectivePreutteranceMS: 72},
		{Position: 2, Role: "mora", Mora: "か", NoteStartMS: 240, DurationMS: 120, EffectivePreutteranceMS: 60},
		{Position: 3, Role: "mora", Mora: "", Silent: true, NoteStartMS: 360, DurationMS: 180},
	}}
	warp := timingWarpJob(synthesisPlan, base.Config{}, 40)
	if warp == nil || warp.Strength != 1 || warp.Language != "ja" {
		t.Fatalf("warp = %+v", warp)
	}
	// sil, a, s, u, k, a, sil
	if len(warp.Spans) != 7 || warp.Spans[0].Label != "sil" || warp.Spans[1].Label != "a" || warp.Spans[6].Label != "sil" {
		t.Fatalf("spans = %+v", warp.Spans)
	}
	// 遷移ユニットのVC長90msを子音長に使い、sは先行発声から始まる。
	if s := warp.Spans[2]; s.Label != "s" || math.Abs(s.Start-0.05) > 1e-9 || math.Abs(s.End-0.14) > 1e-9 {
		t.Fatalf("CVVC transition span = %+v, want 0.05..0.14", s)
	}
	if a := warp.Spans[1]; math.Abs(a.End-0.05) > 1e-9 {
		t.Fatalf("previous vowel was not cut at the consonant onset: %+v", a)
	}
	if k := warp.Spans[4]; k.Label != "k" || math.Abs(k.Start-0.20) > 1e-9 || math.Abs(k.End-0.26) > 1e-9 {
		t.Fatalf("CV consonant span = %+v, want 0.20..0.26", k)
	}
	if len(warp.Starts) != 3 || math.Abs(warp.Starts[0]-0.02) > 1e-9 || math.Abs(warp.Starts[2]-0.26) > 1e-9 {
		t.Fatalf("starts = %v", warp.Starts)
	}
	if len(warp.Ends) != 1 || math.Abs(warp.Ends[0]-0.38) > 1e-9 {
		t.Fatalf("ends = %v", warp.Ends)
	}
	off := false
	cfg := base.Config{}
	cfg.ProviderOptions.Worldline.TimingWarp = &off
	if timingWarpJob(synthesisPlan, cfg, 40) != nil {
		t.Fatal("disabled timing warp produced a job")
	}
}

func TestTimingWarpJobUsesPlanSpansForEnglish(t *testing.T) {
	english := &plan.Plan{Language: "en", LeadingMarginMS: 50, Units: []plan.Unit{
		{Position: 0, Role: "mora", Mora: "w3", NoteStartMS: 0, DurationMS: 120, EffectivePreutteranceMS: 40},
	}, PhoneTimings: []plan.PhoneTiming{
		{Position: 0, Symbol: "w", Role: "onset", StartMS: 0, DurationMS: 30},
		{Position: 0, Symbol: "er", Role: "nucleus", StartMS: 30, DurationMS: 50},
		{Position: 0, Symbol: "l", Role: "coda", StartMS: 80, DurationMS: 20},
	}}
	warp := timingWarpJob(english, base.Config{}, 15)
	// sil, w, er, l, モーラ末尾を埋めるl
	if warp == nil || warp.Language != "en" || len(warp.Spans) != 5 {
		t.Fatalf("English warp = %+v", warp)
	}
	if w := warp.Spans[1]; w.Label != "w" || math.Abs(w.Start-0.05) > 1e-9 || math.Abs(w.End-0.08) > 1e-9 {
		t.Fatalf("English w span = %+v, want 0.05..0.08", w)
	}
	if last := warp.Spans[3]; last.Label != "l" || math.Abs(last.End-0.15) > 1e-9 {
		t.Fatalf("English coda span = %+v, want end 0.15", last)
	}
	if len(warp.Ends) != 1 || math.Abs(warp.Ends[0]-0.17) > 1e-9 {
		t.Fatalf("English ends = %v", warp.Ends)
	}
}
