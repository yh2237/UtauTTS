package worldline

import (
	"math"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"testing"
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

func TestSpeechPlacementCloneKeepsAnchors(t *testing.T) {
	p := &plan.Plan{Language: frontend.LanguageEnglish, Units: []plan.Unit{{Role: "ending", CodaPhones: []string{"l"}, NoteStartMS: 100, DurationMS: 80, PreutteranceMS: 120}}}
	if _, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, 300, 0); err != nil {
		t.Fatal(err)
	}
	clone := plan.Clone(p)
	clone.Units[0].SpeechSourceAnchorsMS[0] = 99
	if p.Units[0].SpeechSourceAnchorsMS[0] == 99 {
		t.Fatal("clone shares anchor slice")
	}
}

func TestEnglishStopEndingKeepsClosureBeforeBurst(t *testing.T) {
	p := &plan.Plan{Language: frontend.LanguageEnglish, Units: []plan.Unit{{
		Role: "ending", CodaPhones: []string{"t"}, NoteStartMS: 88, DurationMS: 184, PreutteranceMS: 191,
		SpeechProfile: &voicebank.SpeechProfile{CodaBurstMS: 289, CodaBurstConfidence: 1},
	}}}
	u, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, 536, 55)
	if err != nil {
		t.Fatal(err)
	}
	a := u.Speech.Anchors
	if u.PositionMS != 88+55 || len(a) < 3 || a[0].SourceMS != 189 || a[0].TargetMS != 0 {
		t.Fatalf("position %.1f anchors %+v", u.PositionMS, a)
	}
	closure := 184 * .55
	for i := 1; i < len(a); i++ {
		if a[i-1].SourceMS < 289 && a[i].SourceMS >= 289 {
			if burst := a[i-1].TargetMS + (289-a[i-1].SourceMS)*(a[i].TargetMS-a[i-1].TargetMS)/(a[i].SourceMS-a[i-1].SourceMS); math.Abs(burst-closure) > 1 {
				t.Fatalf("burst at %.1f, want %.1f: %+v", burst, closure, a)
			}
			return
		}
	}
	t.Fatalf("burst not mapped: %+v", a)
}
