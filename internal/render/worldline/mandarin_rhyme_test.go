package worldline

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestMandarinDiphthongGetsItsOwnTargetInterval(t *testing.T) {
	fixed := 100.0
	p := &plan.Plan{Language: frontend.LanguageChinese, Morae: []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "h", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}}}}, Units: []plan.Unit{{Role: "mora", DurationMS: 200, PreutteranceMS: 70, TargetOnsetMS: 50, SourceFixedMS: &fixed}}, PhoneTimings: []plan.PhoneTiming{{Position: 0, Role: "onset", DurationMS: 50}, {Position: 0, Role: "nucleus", StartMS: 50, DurationMS: 105}, {Position: 0, Role: "offglide", StartMS: 155, DurationMS: 45}}}
	item, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, 400, 0)
	if err != nil {
		t.Fatal(err)
	}
	a := item.Speech.Anchors
	if len(a) != 4 || a[2].TargetMS != 155 || a[3].TargetMS != 200 || a[2].SourceMS != 310 || *p.Units[0].SourceFixedMS != 100 {
		t.Fatal(a)
	}
	if p.Units[0].SpeechMapping != "mandarin-rhyme-prior-v1" || p.Units[0].DurationMS != 200 {
		t.Fatal(p.Units[0])
	}
}

func TestMandarinMedialAndNasalStayOrdered(t *testing.T) {
	u := plan.Unit{Position: 0, PreutteranceMS: 70, NoteStartMS: 500}
	p := &plan.Plan{PhoneTimings: []plan.PhoneTiming{{Position: 0, Role: "medial", StartMS: 540, DurationMS: 24}, {Position: 0, Role: "nucleus", StartMS: 564, DurationMS: 96}, {Position: 0, Role: "coda", StartMS: 660, DurationMS: 60}}}
	a, compound := mandarinRhymeAnchors(p, u, 300, 220)
	if !compound || len(a) != 2 || a[0].SourceMS != 101 || a[0].TargetMS != 64 || a[1].SourceMS != 225 || a[1].TargetMS != 160 {
		t.Fatal(a)
	}
	if a, ok := mandarinRhymeAnchors(p, u, 80, 220); len(a) != 0 || ok {
		t.Fatal(a, ok)
	}
}
