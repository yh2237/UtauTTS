package worldline

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestSpeechPlacementKeepsOtoAndNasalClockSeparate(t *testing.T) {
	fixed := 190.0
	p := &plan.Plan{Language: frontend.LanguageChinese, Morae: []frontend.Mora{{Phones: []frontend.Phone{{Role: "onset"}, {Role: "nucleus"}, {Role: "coda"}}}}, PhoneTimings: []plan.PhoneTiming{{Position: 0, Role: "coda", DurationMS: 60}}, Units: []plan.Unit{{Role: "mora", Position: 0, DurationMS: 220, PreutteranceMS: 160, SourceFixedMS: &fixed, TargetOnsetMS: 40, CodaPhones: []string{"n"}}}}
	u, err := placeSpeechUnit(p, 0, worldlineManifestUnit{ConsonantMS: 45}, 400, 0)
	if err != nil {
		t.Fatal(err)
	}
	if u.ConsonantMS != 190 || fixed != 190 || u.Speech == nil {
		t.Fatal(u)
	}
	a := u.Speech.Anchors
	if len(a) != 4 || a[1].SourceMS != 160 || a[1].TargetMS != 40 || a[2].TargetMS != 160 || a[3].TargetMS != 220 {
		t.Fatal(a)
	}
	clone := plan.Clone(p)
	clone.Units[0].SpeechSourceAnchorsMS[0] = 99
	if p.Units[0].SpeechSourceAnchorsMS[0] != 0 {
		t.Fatal("clone shares anchor slice")
	}
}
