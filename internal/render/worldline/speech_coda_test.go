package worldline

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestCodaMappingProtectsMeasuredReleaseWithoutChangingSpeechClock(t *testing.T) {
	for _, tc := range []struct {
		name     string
		profile  voicebank.SpeechProfile
		pre, end float64
	}{
		{"delta-release", voicebank.SpeechProfile{ActivityEndMS: 257, ActivityConfidence: 1, ReleaseTransientMS: 203, ReleaseTransientDurationMS: 7.5, ReleaseTransientConfidence: .31, TransientMS: 113, TransientDurationMS: 11, TransientConfidence: 1}, 117, 333},
		{"vccv-preutterance-release", voicebank.SpeechProfile{ActivityEndMS: 142, ActivityConfidence: 1, TransientMS: 117, TransientDurationMS: 5.7, TransientConfidence: .92}, 116, 317},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &plan.Plan{Language: frontend.LanguageEnglish, Morae: []frontend.Mora{{}}, Units: []plan.Unit{{Role: "ending", DurationMS: 69.3, NoteStartMS: 641.28, PreutteranceMS: tc.pre, CodaPhones: []string{"d"}, SpeechProfile: &tc.profile}}}
			item, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, tc.end, 0)
			if err != nil {
				t.Fatal(err)
			}
			if p.Units[0].DurationMS != 69.3 || p.Units[0].NoteStartMS != 641.28 || item.LengthMS != 89.29999999999995 {
				t.Fatal("speech clock changed", item)
			}
			if !item.Speech.ProtectStop || !p.Units[0].CodaReleaseSeparated {
				t.Fatal("release not protected", item.Speech)
			}
			a := item.Speech.Anchors
			if a[len(a)-1].SourceMS >= tc.end {
				t.Fatal("trailing silence not excluded", a)
			}
			found := false
			for i := 1; i < len(a); i++ {
				if a[i-1].SourceMS < item.Speech.SourceTransientMS && a[i].SourceMS > item.Speech.SourceTransientMS {
					found = true
					if math.Abs((a[i].SourceMS-a[i-1].SourceMS)-(a[i].TargetMS-a[i-1].TargetMS)) > .001 {
						t.Fatal("release stretched", a)
					}
				}
			}
			if !found {
				t.Fatal("no measured release interval", a)
			}
		})
	}
}

func TestCodaMappingDoesNotProtectUnreliableOrForeignTransient(t *testing.T) {
	profile := &voicebank.SpeechProfile{TransientMS: 100, TransientDurationMS: 8, TransientConfidence: .1, ActivityConfidence: 1, ActivityEndMS: 150}
	p := &plan.Plan{Language: frontend.LanguageEnglish, Morae: []frontend.Mora{{}}, Units: []plan.Unit{{Role: "ending", DurationMS: 70, NoteStartMS: 200, PreutteranceMS: 100, CodaPhones: []string{"d"}, SpeechProfile: profile}}}
	for _, lang := range []string{frontend.LanguageEnglish, frontend.LanguageChinese} {
		p.Language = lang
		item, err := placeSpeechUnit(p, 0, worldlineManifestUnit{}, 300, 0)
		if err != nil || item.Speech.ProtectStop || item.Speech.Anchors[len(item.Speech.Anchors)-1].SourceMS != 300 {
			t.Fatal("unreliable source changed", item, err)
		}
	}
}
