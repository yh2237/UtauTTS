package worldline

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
	"github.com/yh2237/UtauTTS/internal/voicebank"
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
		{"english syllable uses utau placement", &plan.Plan{Language: "en", PhoneTimingSource: "multilingual-speech-score-v1",
			Units: []plan.Unit{{Position: 0, Role: "mora"}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{}},
		{"english coda keeps speech placement", &plan.Plan{Language: "en", PhoneTimingSource: "multilingual-speech-score-v1",
			Units: []plan.Unit{{Position: 0, Role: "ending", CodaPhones: []string{"t"}}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{multilingual: true}},
		{"english tail uses utau placement", &plan.Plan{Language: "en", PhoneTimingSource: "multilingual-speech-score-v1",
			Units: []plan.Unit{{Position: 0, Role: "ending"}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{}},
		{"chinese syllable uses utau placement", &plan.Plan{Language: "zh", PhoneTimingSource: "multilingual-speech-score-v1",
			Units: []plan.Unit{{Position: 0, Role: "mora"}}}, false, 0, false, false, false,
			worldlineSpeechStrategy{}},
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

func TestEnglishMainStopsAtCodaEnding(t *testing.T) {
	units := []plan.Unit{
		{Position: 0, Role: "mora", NoteStartMS: 0, DurationMS: 351},
		{Position: 0, Role: "ending", NoteStartMS: 215, DurationMS: 66, CodaPhones: []string{"l"}},
		{Position: 0, Role: "ending", NoteStartMS: 281, DurationMS: 69, CodaPhones: []string{"d"}},
	}
	english := &plan.Plan{Language: "en", PhoneTimingSource: "multilingual-speech-score-v1", Units: units}
	if cut, ok := codaEndingStart(english, units[0]); !ok || cut != 215 {
		t.Fatalf("english cut = %v %v, want 215", cut, ok)
	}
	if _, ok := codaEndingStart(english, units[1]); ok {
		t.Fatal("ending unit must not be cut")
	}
	japanese := &plan.Plan{Language: "ja", Units: units}
	if _, ok := codaEndingStart(japanese, units[0]); ok {
		t.Fatal("japanese must not be cut")
	}

	points := []base.WorldlineEnvelopePoint{{XMS: 0, Y: 0}, {XMS: 20, Y: 1}, {XMS: 200, Y: 1}, {XMS: 380, Y: 1}, {XMS: 400, Y: 0}}
	got := envelopeEndAt(points, 250)
	if got[4].XMS != 250 || got[3].XMS != 240 || got[2].XMS != 200 || points[4].XMS != 400 {
		t.Fatalf("capped envelope = %+v (original %+v)", got, points)
	}
	if longer := envelopeEndAt(points, 500); longer[4].XMS != 500 || longer[3].XMS != 490 || longer[2].XMS != 200 {
		t.Fatalf("extended envelope = %+v", longer)
	}
}
