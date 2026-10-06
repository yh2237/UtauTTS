package worldline

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestResolveWorldlineSpeechStrategy(t *testing.T) {
	profile := &voicebank.SpeechProfile{TransientMS: 40, TransientDurationMS: 10, TransientConfidence: 1, Applied: true}
	releaseProfile := &voicebank.SpeechProfile{ReleaseTransientMS: 30, ReleaseTransientConfidence: 1, Applied: true}
	japaneseStop := func(vcv bool) *plan.Plan {
		aliasKind := "CV"
		if vcv {
			aliasKind = "VCV"
		}
		return &plan.Plan{Language: "ja", Morae: []frontend.Mora{{Vowel: "a"}, {Consonant: "k", Vowel: "a"}},
			Units: []plan.Unit{{Position: 0, Role: "mora"}, {Position: 1, Role: "mora", AliasKind: aliasKind, SpeechProfile: profile}}}
	}
	cases := []struct {
		name      string
		plan      *plan.Plan
		legacyMix bool
		index     int
		coda      bool
		singleCV  bool
		vcv       bool
		want      worldlineSpeechStrategy
	}{
		{"japanese CV protected stop", japaneseStop(false), false, 1, false, false, false,
			worldlineSpeechStrategy{stopProtected: true, preserveStopOnly: true}},
		{"japanese VCV protected stop", japaneseStop(true), false, 1, false, false, true,
			worldlineSpeechStrategy{stopProtected: true, preserveStopOnly: true}},
		{"legacy japanese E2B stop", japaneseStop(true), true, 1, false, false, true,
			worldlineSpeechStrategy{stopProtected: true, legacyE2BStop: true}},
		{"stretch adapted mora", &plan.Plan{Language: "ja", Morae: []frontend.Mora{{Vowel: "a"}},
			Units: []plan.Unit{{Position: 0, Role: "mora", StretchAdapted: true}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{stretch: true}},
		{"coda release ending", &plan.Plan{Language: "ja", Units: []plan.Unit{
			{Role: "ending", CodaPhones: []string{"d"}, SpeechProfile: releaseProfile}}}, false, 0, true, false, false,
			worldlineSpeechStrategy{codaRelease: true, stopProtected: true}},
		{"multilingual english", &plan.Plan{Language: "en", PhoneTimingSource: "multilingual-speech-score-v1",
			Units: []plan.Unit{{Position: 0, Role: "mora"}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{multilingual: true}},
		{"single CV legato", &plan.Plan{SingleCV: true, Language: "ja",
			Morae: []frontend.Mora{{Vowel: "a"}, {Vowel: "a"}},
			Units: []plan.Unit{{Position: 0, Role: "mora"}, {Position: 1, Role: "mora"}}}, false, 1, false, true, false,
			worldlineSpeechStrategy{singleCV: true, singleCVLegato: true}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			builder := worldlineUnitBuilder{plan: test.plan, legacyMix: test.legacyMix}
			got := builder.resolveWorldlineSpeechStrategy(test.index, test.coda, test.singleCV, test.vcv)
			if got != test.want {
				t.Fatalf("strategy = %+v, want %+v", got, test.want)
			}
		})
	}
}
