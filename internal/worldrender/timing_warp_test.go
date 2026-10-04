package worldrender

import "testing"

func TestDecodeTimingWarpCountsCVVCTransitionAsConsonant(t *testing.T) {
	plan := []byte(`{"leading_margin_ms":20,"units":[
		{"position":0,"role":"mora","mora":"あ","note_start_ms":0,"duration_ms":120,"effective_preutterance_ms":0},
		{"position":1,"role":"transition","mora":"す","note_start_ms":120,"duration_ms":90,"effective_preutterance_ms":90},
		{"position":1,"role":"mora","mora":"す","note_start_ms":120,"duration_ms":120,"effective_preutterance_ms":72},
		{"position":2,"role":"mora","mora":"か","note_start_ms":240,"duration_ms":120,"effective_preutterance_ms":60}]}`)
	warp, err := decodeTimingWarp(plan, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(warp.Morae) != 3 {
		t.Fatalf("morae = %d, want 3", len(warp.Morae))
	}
	if got := warp.Morae[1].EffectivePreutteranceMS; got != 90 {
		t.Fatalf("CVVC consonant = %v, want the VC length 90", got)
	}
	if got := warp.Morae[2].EffectivePreutteranceMS; got != 60 {
		t.Fatalf("CV consonant = %v, want 60", got)
	}
}
