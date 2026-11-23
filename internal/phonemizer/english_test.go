package phonemizer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p/en"
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func parseAndHints(t *testing.T, phonemizerID, text, reading string, dictionary map[string]string, presamp voicebank.PresampConfig) (linguistics.Utterance, []voicebank.AliasHints) {
	t.Helper()
	utterance, err := en.Parse(phonemizerID, text, reading, dictionary)
	if err != nil {
		t.Fatal(err)
	}
	ph, ok := Lookup(phonemizerID)
	if !ok {
		t.Fatalf("phonemizer %q not registered", phonemizerID)
	}
	hints, err := ph.Aliases(utterance, presamp)
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != len(utterance.Morae) {
		t.Fatalf("hints length = %d, morae = %d", len(hints), len(utterance.Morae))
	}
	return utterance, hints
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestRegistryCapabilities(t *testing.T) {
	for _, lang := range []string{linguistics.LanguageJapanese, linguistics.LanguageEnglish} {
		if !SupportsLanguage(lang) {
			t.Fatalf("SupportsLanguage(%q) = false, want true", lang)
		}
	}
	for _, lang := range []string{linguistics.LanguageChinese, linguistics.LanguageSpanish, linguistics.LanguagePortuguese} {
		if SupportsLanguage(lang) {
			t.Fatalf("SupportsLanguage(%q) = true, want false", lang)
		}
	}
	if _, ok := Lookup(linguistics.PhonemizerChinese); ok {
		t.Fatal("zh-cvvc must not be registered as a waveform phonemizer")
	}
	ja, ok := DefaultForLanguage("ja")
	if !ok || ja.ID() != linguistics.PhonemizerJapanese {
		t.Fatalf("ja default = %#v, %v", ja, ok)
	}
	if hints, err := ja.Aliases(linguistics.Utterance{}, voicebank.PresampConfig{}); err != nil || hints != nil {
		t.Fatalf("ja aliases = %#v, %v", hints, err)
	}
}

func TestEnglishARPAsingAliases(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglish, "", "HH AH0 L OW1", nil, voicebank.PresampConfig{})
	if utterance.Reading != "HH AH0 L OW1" || len(utterance.Morae) != 4 {
		t.Fatalf("reading=%q morae=%#v", utterance.Reading, utterance.Morae)
	}
	if utterance.Morae[0].Text != "hh" || hints[0].Main[0] != "- hh" {
		t.Fatalf("first = %#v %#v", utterance.Morae[0], hints[0])
	}
	if utterance.Morae[1].Text != "ah" || hints[1].Main[0] != "hh ah" {
		t.Fatalf("second = %#v %#v", utterance.Morae[1], hints[1])
	}
	if utterance.Morae[1].Stress != 0 {
		t.Fatalf("stress = %#v", utterance.Morae[1])
	}
}

func TestEnglishCVAliases(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglishCV, "", "HH AH0 L OW1", nil, voicebank.PresampConfig{})
	if len(utterance.Morae) != 4 {
		t.Fatalf("morae=%#v", utterance.Morae)
	}
	if utterance.Morae[0].Text != "hh" || hints[0].Main[0] != "- hh" {
		t.Fatalf("phrase-initial consonant = %#v", hints[0])
	}
	if utterance.Morae[1].Text != "ah" || hints[1].Main[0] != "-ah" {
		t.Fatalf("medial vowel = %#v", hints[1])
	}
	if utterance.Morae[2].Text != "l" || hints[2].Main[0] != "l" {
		t.Fatalf("medial consonant = %#v", hints[2])
	}
	if got := hints[3].Endings; len(got) != 1 || got[0][0] != "ow -" || got[0][1] != "ow-" {
		t.Fatalf("ending = %#v", got)
	}
	vowelUtterance, vowelHints := parseAndHints(t, linguistics.PhonemizerEnglishCV, "", "AA1 R", nil, voicebank.PresampConfig{})
	if vowelUtterance.Morae[0].Text != "aa" || vowelHints[0].Main[0] != "-aa" {
		t.Fatalf("phrase-initial vowel = %#v", vowelHints[0])
	}
}

func TestEnglishDeltaAliases(t *testing.T) {
	_, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "HH AH0 L OW1", nil, voicebank.PresampConfig{})
	if len(hints) != 2 || hints[0].Main[0] != "- h@" || hints[1].Main[0] != "loU" {
		t.Fatalf("hints=%#v", hints)
	}
	if hints[1].Transition[0] != "@ l" {
		t.Fatalf("transition=%#v", hints[1].Transition)
	}
}

func TestEnglishVCCVAliases(t *testing.T) {
	_, hints := parseAndHints(t, linguistics.PhonemizerEnglishVCCV, "", "HH AH0 L OW1", nil, voicebank.PresampConfig{})
	if len(hints) != 2 || hints[0].Main[0] != "-hx" || hints[1].Main[0] != "lO" {
		t.Fatalf("hints=%#v", hints)
	}
	if hints[1].Transition[0] != "x l" {
		t.Fatalf("transition=%#v", hints[1].Transition)
	}

	// IH+NGとsp語末のVCCV特例も保持する。
	_, ihNg := parseAndHints(t, linguistics.PhonemizerEnglishVCCV, "", "S IH1 NG", nil, voicebank.PresampConfig{})
	if ihNg[0].Endings[0][0] != "1 ng" || ihNg[0].Endings[0][1] != "1ng" {
		t.Fatalf("ih+ng endings = %#v", ihNg[0].Endings)
	}
	_, sp := parseAndHints(t, linguistics.PhonemizerEnglishVCCV, "", "G R AE1 S P", nil, voicebank.PresampConfig{})
	if !containsString(sp[0].Endings[0], "@ sp") || len(sp[0].EndingFallbacks) != 1 || len(sp[0].EndingFallbacks[0]) != 3 {
		t.Fatalf("sp endings = %#v %#v", sp[0].Endings, sp[0].EndingFallbacks)
	}
}

func TestEnglishVowelInitialSyllableGeneratesVVTransition(t *testing.T) {
	for _, tc := range []struct {
		name       string
		phonemizer string
		reading    string
		previous   string
		next       []string
		mainFirst  string
	}{
		{"delta", linguistics.PhonemizerEnglishDelta, "M IY1 | AH0 T", "i", []string{"@", "V"}, "@"},
		{"vccv", linguistics.PhonemizerEnglishVCCV, "M IY1 | AE1 T", "E", []string{"@"}, "@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, hints := parseAndHints(t, tc.phonemizer, "", tc.reading, nil, voicebank.PresampConfig{})
			if len(hints) != 2 || hints[1].Main[0] != tc.mainFirst {
				t.Fatalf("hints=%#v", hints)
			}
			transition := hints[1].Transition
			// 母音始まりはVCではなくVV候補になる。
			for _, next := range tc.next {
				if !containsString(transition, tc.previous+" "+next) {
					t.Fatalf("missing VV %q: %#v", tc.previous+" "+next, transition)
				}
				if !containsString(transition, tc.previous+next) {
					t.Fatalf("missing compact VV %q: %#v", tc.previous+next, transition)
				}
			}
		})
	}
}

func TestEnglishOnsetSyllableKeepsVCTransition(t *testing.T) {
	_, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "M IY1 | T AE1", nil, voicebank.PresampConfig{})
	if len(hints) != 2 {
		t.Fatalf("hints=%#v", hints)
	}
	if got := hints[1].Transition; len(got) != 1 || got[0] != "i t" {
		t.Fatalf("onset must keep VC transition: %#v", got)
	}
}

func TestEnglishVowelTransitionPrefersDashThenFallback(t *testing.T) {
	got := combineEnglishVowelTransitionAliases([]string{"i"}, []string{"@"}, " ")
	want := []string{"i @-", "i@-", "i @", "i@"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dash release must lead then fall back: %#v", got)
	}
}

func TestEnglishCVVCKeepsFinalConsonant(t *testing.T) {
	_, delta := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "K AE1 T", nil, voicebank.PresampConfig{})
	if len(delta) != 1 || delta[0].Endings[0][0] != "{ t-" {
		t.Fatalf("delta=%#v", delta)
	}
	_, vccv := parseAndHints(t, linguistics.PhonemizerEnglishVCCV, "", "K AE1 T", nil, voicebank.PresampConfig{})
	if len(vccv) != 1 || vccv[0].Endings[0][0] != "@ t-" {
		t.Fatalf("vccv=%#v", vccv)
	}
}

func TestEnglishCVVCSplitsFinalCluster(t *testing.T) {
	_, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "T EH1 K S T", nil, voicebank.PresampConfig{})
	if len(hints) != 1 || len(hints[0].Endings) != 2 {
		t.Fatalf("hints=%#v", hints)
	}
	if hints[0].Endings[0][0] != "E k" || hints[0].Endings[1][0] != "k st-" {
		t.Fatalf("endings=%#v", hints[0].Endings)
	}
	for _, alias := range []string{"k st", "kst", "st-", "st"} {
		if !containsString(hints[0].Endings[1], alias) {
			t.Fatalf("missing fallback %q: %#v", alias, hints[0].Endings[1])
		}
	}
}

func TestEnglishFinalClusterCanUseLastConsonantAlone(t *testing.T) {
	_, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "world", "", nil, voicebank.PresampConfig{})
	if len(hints) != 1 || len(hints[0].Endings) != 2 {
		t.Fatalf("hints=%#v", hints)
	}
	if !containsString(hints[0].Endings[1], "d") {
		t.Fatalf("d fallback missing: %#v", hints[0].Endings[1])
	}
}

func TestEnglishSyllabificationSplitsIllegalOnset(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "AE1 T L AH0", nil, voicebank.PresampConfig{})
	if len(hints) != 2 || utterance.Morae[1].Consonant != "l" {
		t.Fatalf("morae=%#v hints=%#v", utterance.Morae, hints)
	}
	if len(hints[0].Endings) != 2 || hints[0].Endings[0][0] != "{ t" || hints[0].Endings[1][0] != "t l-" {
		t.Fatalf("bridge=%#v", hints[0].Endings)
	}
	// 語境界のCCは解放マーカー付きを優先し、非マーカー形も候補に残す。
	for _, alias := range []string{"tl-", "t l", "tl"} {
		if !containsString(hints[0].Endings[1], alias) {
			t.Fatalf("missing bridge fallback %q: %#v", alias, hints[0].Endings[1])
		}
	}
}

func TestEnglishSyllabificationKeepsValidThreePhoneOnset(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "", "EH1 K S T R AH0", nil, voicebank.PresampConfig{})
	if len(hints) != 2 || utterance.Morae[1].Consonant != "s t r" {
		t.Fatalf("morae=%#v", utterance.Morae)
	}
	if hints[0].Endings[0][0] != "E k" || hints[0].Endings[1][0] != "k str-" {
		t.Fatalf("bridge=%#v", hints[0].Endings)
	}
	if !containsString(hints[1].Main, "rV") || !containsString(hints[1].Transition, "str") {
		t.Fatalf("fallback main=%#v transition=%#v", hints[1].Main, hints[1].Transition)
	}
}

func TestEnglishGeneratedReadingKeepsWordBoundaries(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglishDelta, "cat is", "", map[string]string{
		"cat": "K AE T",
		"is":  "IH Z",
	}, voicebank.PresampConfig{})
	if utterance.Reading != "K AE T | IH Z" {
		t.Fatalf("reading=%q", utterance.Reading)
	}
	if len(hints) != 2 || hints[0].Endings[0][0] != "{ t" || hints[1].Main[0] != "tI" {
		t.Fatalf("hints=%#v", hints)
	}
}

func TestEnglishExplicitReadingAcceptsWordBoundary(t *testing.T) {
	utterance, hints := parseAndHints(t, linguistics.PhonemizerEnglish, "", "K AE T | IH Z", nil, voicebank.PresampConfig{})
	if utterance.Reading != "K AE T | IH Z" || len(hints) != 5 || hints[3].Main[0] != "t ih" {
		t.Fatalf("reading=%q hints=%#v", utterance.Reading, hints)
	}
	if len(hints[2].Endings) != 0 || len(hints[4].Endings) != 1 {
		t.Fatal("word boundaries must not insert phrase-final releases")
	}
}

func TestEnglishSchwaAliasesPreserveLexicalPhones(t *testing.T) {
	for _, tc := range []struct {
		phonemizer   string
		weak, strong string
	}{
		{linguistics.PhonemizerEnglishDelta, "@", "V"},
		{linguistics.PhonemizerEnglishVCCV, "x", "u"},
	} {
		t.Run(tc.phonemizer, func(t *testing.T) {
			for _, phone := range []struct {
				vowel, symbol string
				stress        int
				stressKnown   bool
			}{
				{"AH0", "ah", 0, true}, {"AH1", "ah", 1, true}, {"AH2", "ah", 2, true},
				{"AH", "ah", 0, false}, {"AX", "ax", 0, false},
			} {
				vowel := phone.vowel
				utterance, hints := parseAndHints(t, tc.phonemizer, "", "HH "+vowel+" N | SP", nil, voicebank.PresampConfig{})
				want := tc.strong
				if vowel == "AH0" || vowel == "AX" {
					want = tc.weak
				}
				if utterance.Reading != "HH "+vowel+" N | SP" {
					t.Fatal(vowel, utterance.Reading)
				}
				if utterance.Morae[0].Vowel != phone.symbol || utterance.Morae[0].Phones[1].Symbol != phone.symbol || utterance.Morae[0].Stress != phone.stress || utterance.Morae[0].StressKnown != phone.stressKnown {
					t.Fatal("lexical stress/phone metadata changed", vowel, utterance.Morae[0])
				}
				if hints[0].Endings[0][0] != want+" n-" {
					t.Fatal("coda used wrong vowel", hints[0])
				}
				if vowel == "AH0" && !containsString(hints[0].Main, "h"+tc.strong) {
					t.Fatal("missing fallback for banks without schwa")
				}
			}
			_, hints := parseAndHints(t, tc.phonemizer, "", "S T R AH0 L OW1", nil, voicebank.PresampConfig{})
			if !reflect.DeepEqual(hints[0].MainMissing["r"+tc.weak], []string{"s", "t"}) {
				t.Fatal("cluster coverage lost")
			}
			if hints[1].Transition[0] != tc.weak+" l" {
				t.Fatal("transition used stressed vowel")
			}
		})
	}
}

func TestPhoneEnglishWordCodasKeepOptionalReleaseEmpty(t *testing.T) {
	for _, phonemizerID := range []string{linguistics.PhonemizerEnglish, linguistics.PhonemizerEnglishCV} {
		t.Run(phonemizerID, func(t *testing.T) {
			utterance, hints := parseAndHints(t, phonemizerID, "", "T EH1 S T | R IY1 D | SP", nil, voicebank.PresampConfig{})
			for i, role := range []string{"onset", "nucleus", "coda", "coda", "onset", "nucleus", "coda"} {
				if utterance.Morae[i].Phones[0].Role != role {
					t.Fatalf("phone %d: got %s, want %s", i, utterance.Morae[i].Phones[0].Role, role)
				}
			}
			if len(hints[6].EndingPhones) != 0 {
				t.Fatal("optional release became required")
			}
		})
	}
}

func TestEnglishWordBoundaryIsNotPhraseBoundary(t *testing.T) {
	for _, phonemizerID := range []string{linguistics.PhonemizerEnglishDelta, linguistics.PhonemizerEnglishVCCV} {
		t.Run(phonemizerID, func(t *testing.T) {
			_, hints := parseAndHints(t, phonemizerID, "", "K AE1 T | IH1 Z | SP | K AE1 T", nil, voicebank.PresampConfig{})
			if strings.HasPrefix(hints[1].Main[0], "-") || !strings.HasPrefix(hints[3].Main[0], "-") {
				t.Fatalf("phrase context: %+v", hints)
			}
			if len(hints[0].EndingPhones) != 1 || hints[0].EndingPhones[0][0] != "t" {
				t.Fatal("required coda missing")
			}
		})
	}
}
