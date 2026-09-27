package worldline

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestSpeechCodaProtectsStrongOnsetInsteadOfWeakTail(t *testing.T) {
	p := &plan.Plan{Language: frontend.LanguageEnglish, Units: []plan.Unit{{
		Role: "ending", CodaPhones: []string{"d"}, NoteStartMS: 641.28, DurationMS: 69.3,
		PreutteranceMS: 117.333,
		SpeechProfile: &voicebank.SpeechProfile{
			TransientMS: 112.812, TransientDurationMS: 11.111, TransientConfidence: 1,
			ReleaseTransientMS: 203.401, ReleaseTransientDurationMS: 7.483, ReleaseTransientConfidence: .313,
		},
	}}}
	u, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, 333.668, 55)
	if err != nil {
		t.Fatal(err)
	}
	if u.Speech == nil || !u.Speech.ProtectStop || u.Speech.SourceTransientMS != 112.812 {
		t.Fatalf("wrong protected transient: %#v", u.Speech)
	}
	anchors := u.Speech.Anchors
	for i := 1; i < len(anchors); i++ {
		if anchors[i-1].SourceMS == 108.812 {
			if d := anchors[i].TargetMS - anchors[i-1].TargetMS; d < 21 || d > 22 {
				t.Fatalf("burst compressed to %.3f ms", d)
			}
			return
		}
	}
	t.Fatal("missing protected burst interval")
}

func TestSpeechCodaKeepsComparableReleaseCandidate(t *testing.T) {
	p := &plan.Plan{Language: frontend.LanguageEnglish}
	u := plan.Unit{Role: "ending", CodaPhones: []string{"d"}, SpeechProfile: &voicebank.SpeechProfile{
		TransientMS: 110, TransientDurationMS: 11, TransientConfidence: .9,
		ReleaseTransientMS: 200, ReleaseTransientDurationMS: 8, ReleaseTransientConfidence: .8,
	}}
	position, _, ok := speechCodaTransient(p, u, 300)
	if !ok || position != 200 {
		t.Fatalf("release candidate = %v, %v", position, ok)
	}
}

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
