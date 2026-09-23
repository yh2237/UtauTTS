package frontend

import "testing"

func TestEnglishPunctuationPreservesPhrasePauses(t *testing.T) {
	for _, parser := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishARPAsing, ParseEnglishDelta, ParseEnglishVCCV, ParseEnglishCV} {
		reading, units, err := parser("Cat, is!", "", map[string]string{"cat": "K AE1 T", "is": "IH0 Z"})
		if err != nil {
			t.Fatal(err)
		}
		pauses := 0
		for _, unit := range units {
			if unit.Pause {
				pauses++
			}
		}
		if pauses != 2 {
			t.Fatalf("reading=%q pauses=%d", reading, pauses)
		}
		_, roundtrip, err := parser("", reading, nil)
		if err != nil || len(roundtrip) != len(units) {
			t.Fatalf("reading roundtrip: %v", err)
		}
	}
}

func TestParseEnglishARPAsingReading(t *testing.T) {
	reading, units, err := ParseEnglishARPAsing("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "HH AH0 L OW1" || len(units) != 4 {
		t.Fatalf("reading=%q units=%#v", reading, units)
	}
	if units[0].Text != "hh" || units[0].Aliases.Main[0] != "- hh" {
		t.Fatalf("first unit = %#v", units[0])
	}
	if units[1].Text != "ah" || units[1].Aliases.Main[0] != "hh ah" {
		t.Fatalf("second unit = %#v", units[1])
	}
	if units[0].DurationScale != 0.45 || units[1].DurationScale != 1 || units[1].Stress != 0 {
		t.Fatalf("timing metadata = %#v", units[:2])
	}
}

func TestParseEnglishCV(t *testing.T) {
	_, units, err := ParseEnglishCV("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 4 {
		t.Fatalf("units=%#v", units)
	}
	if units[0].Text != "hh" || units[0].Aliases.Main[0] != "- hh" {
		t.Fatalf("phrase-initial consonant = %#v", units[0].Aliases)
	}
	if units[1].Text != "ah" || units[1].Aliases.Main[0] != "-ah" {
		t.Fatalf("medial vowel = %#v", units[1].Aliases)
	}
	if units[2].Text != "l" || units[2].Aliases.Main[0] != "l" {
		t.Fatalf("medial consonant = %#v", units[2].Aliases)
	}
	if got := units[3].Aliases.Endings; len(got) != 1 || got[0][0] != "ow -" || got[0][1] != "ow-" {
		t.Fatalf("ending = %#v", got)
	}
	if units[0].DurationScale != 0.45 || units[1].DurationScale != 1 {
		t.Fatalf("timing metadata = %#v", units[:2])
	}
	_, vowelInitial, err := ParseEnglishCV("", "AA1 R", nil)
	if err != nil {
		t.Fatal(err)
	}
	if vowelInitial[0].Text != "aa" || vowelInitial[0].Aliases.Main[0] != "-aa" {
		t.Fatalf("phrase-initial vowel = %#v", vowelInitial[0].Aliases)
	}
	if _, _, err := ParseEnglishCV("", "K INVALID AE1", nil); err == nil {
		t.Fatal("invalid phone accepted")
	}
}

func TestParseEnglishUsesBuiltInG2P(t *testing.T) {
	reading, units, err := ParseEnglishDelta("hello", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "HH AH0 L OW1" || len(units) != 2 {
		t.Fatalf("reading=%q units=%#v", reading, units)
	}
}

func TestParseEnglishDelta(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Aliases.Main[0] != "- h@" || units[1].Aliases.Main[0] != "loU" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Transition[0] != "@ l" {
		t.Fatalf("transition=%#v", units[1].Aliases.Transition)
	}
}

func TestParseEnglishVCCV(t *testing.T) {
	_, units, err := ParseEnglishVCCV("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Aliases.Main[0] != "-hx" || units[1].Aliases.Main[0] != "lO" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Transition[0] != "x l" {
		t.Fatalf("transition=%#v", units[1].Aliases.Transition)
	}
}

func TestEnglishVowelInitialSyllableGeneratesVVTransition(t *testing.T) {
	for _, tc := range []struct {
		name      string
		parser    func(string, string, map[string]string) (string, []Mora, error)
		reading   string
		previous  string
		next      []string
		mainFirst string
	}{
		{"delta", ParseEnglishDelta, "M IY1 | AH0 T", "i", []string{"@", "V"}, "@"},
		{"vccv", ParseEnglishVCCV, "M IY1 | AE1 T", "E", []string{"@"}, "@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, units, err := tc.parser("", tc.reading, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(units) != 2 || units[1].Aliases.Main[0] != tc.mainFirst {
				t.Fatalf("units=%#v", units)
			}
			transition := units[1].Aliases.Transition
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
	_, units, err := ParseEnglishDelta("", "M IY1 | T AE1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("units=%#v", units)
	}
	if got := units[1].Aliases.Transition; len(got) != 1 || got[0] != "i t" {
		t.Fatalf("onset must keep VC transition: %#v", got)
	}
}

func TestEnglishVowelTransitionPrefersDashThenFallback(t *testing.T) {
	got := combineEnglishVowelTransitionAliases([]string{"i"}, []string{"@"}, " ")
	want := []string{"i @-", "i@-", "i @", "i@"}
	if len(got) != len(want) {
		t.Fatalf("candidates=%#v", got)
	}
	for index, alias := range want {
		if got[index] != alias {
			t.Fatalf("dash release must lead then fall back: %#v", got)
		}
	}
}

func TestEnglishCVVCKeepsFinalConsonant(t *testing.T) {
	_, delta, err := ParseEnglishDelta("", "K AE1 T", nil)
	if err != nil || len(delta) != 1 || delta[0].Aliases.Endings[0][0] != "{ t-" {
		t.Fatalf("delta=%#v err=%v", delta, err)
	}
	_, vccv, err := ParseEnglishVCCV("", "K AE1 T", nil)
	if err != nil || len(vccv) != 1 || vccv[0].Aliases.Endings[0][0] != "@ t-" {
		t.Fatalf("vccv=%#v err=%v", vccv, err)
	}
}

func TestEnglishCVVCSplitsFinalCluster(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "T EH1 K S T", nil)
	if err != nil || len(units) != 1 || len(units[0].Aliases.Endings) != 2 {
		t.Fatalf("units=%#v err=%v", units, err)
	}
	if units[0].Aliases.Endings[0][0] != "E k" || units[0].Aliases.Endings[1][0] != "k st-" {
		t.Fatalf("endings=%#v", units[0].Aliases.Endings)
	}
	for _, alias := range []string{"k st", "kst", "st-", "st"} {
		if !containsString(units[0].Aliases.Endings[1], alias) {
			t.Fatalf("missing fallback %q: %#v", alias, units[0].Aliases.Endings[1])
		}
	}
}

func TestEnglishFinalClusterCanUseLastConsonantAlone(t *testing.T) {
	_, units, err := ParseEnglishDelta("world", "", nil)
	if err != nil || len(units) != 1 || len(units[0].Aliases.Endings) != 2 {
		t.Fatalf("units=%#v err=%v", units, err)
	}
	if !containsString(units[0].Aliases.Endings[1], "d") {
		t.Fatalf("d fallback missing: %#v", units[0].Aliases.Endings[1])
	}
}

func TestEnglishSyllabificationSplitsIllegalOnset(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "AE1 T L AH0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[1].Consonant != "l" {
		t.Fatalf("units=%#v", units)
	}
	if len(units[0].Aliases.Endings) != 2 || units[0].Aliases.Endings[0][0] != "{ t" || units[0].Aliases.Endings[1][0] != "t l-" {
		t.Fatalf("bridge=%#v", units[0].Aliases.Endings)
	}
	// 語境界のCCは解放マーカー付きを優先し、非マーカー形も候補に残す。
	for _, alias := range []string{"tl-", "t l", "tl"} {
		if !containsString(units[0].Aliases.Endings[1], alias) {
			t.Fatalf("missing bridge fallback %q: %#v", alias, units[0].Aliases.Endings[1])
		}
	}
}

func TestEnglishSyllableBridgeIncludesDashReleaseCandidates(t *testing.T) {
	got := englishSyllableBridge([]string{"@"}, []string{"t"}, []string{"m"}, deltaEnglishSymbols, " ")
	if len(got) != 2 {
		t.Fatalf("groups=%#v", got)
	}
	if got[0][0] != "@ t" || got[1][0] != "t m-" {
		t.Fatalf("dash release must lead: %#v", got)
	}
	for _, alias := range []string{"tm-", "t m", "tm"} {
		if !containsString(got[1], alias) {
			t.Fatalf("missing bridge candidate %q: %#v", alias, got[1])
		}
	}
	cluster := englishSyllableBridge([]string{"{"}, []string{"k"}, []string{"s", "t", "r"}, deltaEnglishSymbols, " ")
	if cluster[1][0] != "k str-" || !containsString(cluster[1], "k str") || !containsString(cluster[1], "kstr-") {
		t.Fatalf("cluster bridge=%#v", cluster[1])
	}
}

func TestEnglishSyllabificationKeepsValidThreePhoneOnset(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "EH1 K S T R AH0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[1].Consonant != "s t r" {
		t.Fatalf("units=%#v", units)
	}
	if units[0].Aliases.Endings[0][0] != "E k" || units[0].Aliases.Endings[1][0] != "k str-" {
		t.Fatalf("bridge=%#v", units[0].Aliases.Endings)
	}
	if !containsString(units[1].Aliases.Main, "rV") || !containsString(units[1].Aliases.Transition, "str") {
		t.Fatalf("fallback main=%#v transition=%#v", units[1].Aliases.Main, units[1].Aliases.Transition)
	}
}

func TestEnglishGeneratedReadingKeepsWordBoundaries(t *testing.T) {
	reading, units, err := ParseEnglishDelta("cat is", "", map[string]string{
		"cat": "K AE T",
		"is":  "IH Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading != "K AE T | IH Z" {
		t.Fatalf("reading=%q", reading)
	}
	if len(units) != 2 || units[0].Aliases.Endings[0][0] != "{ t" || units[1].Aliases.Main[0] != "tI" {
		t.Fatalf("units=%#v", units)
	}
}

func TestEnglishExplicitReadingAcceptsWordBoundary(t *testing.T) {
	reading, units, err := ParseEnglishARPAsing("", "K AE T | IH Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "K AE T | IH Z" || len(units) != 5 || units[3].Aliases.Main[0] != "t ih" {
		t.Fatalf("reading=%q units=%#v", reading, units)
	}
	if len(units[2].Aliases.Endings) != 0 || len(units[4].Aliases.Endings) != 1 {
		t.Fatal("word boundaries must not insert phrase-final releases")
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestChineseCVVCUsesPresampClasses(t *testing.T) {
	config := PresampConfig{
		Vowels:     map[string]string{"zhi": "ir", "hao": "ao"},
		Consonants: map[string]string{"zhi": "zh", "hao": "h"},
		Endings:    []string{"%v% R"},
	}
	_, units, err := ParseChineseCVVCWithConfig("", "zhi hao", nil, config)
	if err != nil {
		t.Fatal(err)
	}
	if units[0].Vowel != "ir" || units[1].Aliases.Transition[0] != "ir h" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Endings[0][0] != "ao R" {
		t.Fatalf("ending=%#v", units[1].Aliases.Endings)
	}
}

func TestParseChineseCVVC(t *testing.T) {
	reading, units, err := ParseChineseCVVC("你好", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "ni3 hao3" || len(units) != 2 {
		t.Fatalf("reading=%q units=%#v", reading, units)
	}
	if units[0].Tone != 3 || units[1].Tone != 3 {
		t.Fatalf("tones=%#v", units)
	}
	if units[0].Aliases.Main[0] != "- ni" || units[1].Aliases.Main[0] != "i hao" {
		t.Fatalf("units = %#v", units)
	}
	if units[1].Aliases.Transition[0] != "i h" {
		t.Fatalf("transition = %#v", units[1].Aliases.Transition)
	}
}

func TestParseChineseCVVCUsesLongestDictionaryEntry(t *testing.T) {
	reading, _, err := ParseChineseCVVC("重庆人", "", map[string]string{
		"重": "zhong", "重庆": "chong qing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading != "chong qing ren2" {
		t.Fatalf("reading = %q", reading)
	}
}

func TestResolveLanguageDefaults(t *testing.T) {
	language, phonemizer, err := ResolveLanguage("zh", "")
	if err != nil || language != LanguageChinese || phonemizer != PhonemizerChinese {
		t.Fatalf("language=%q phonemizer=%q err=%v", language, phonemizer, err)
	}
}

func TestResolveLanguageAcceptsEnglishPhonemizers(t *testing.T) {
	for _, phonemizer := range []string{PhonemizerEnglish, PhonemizerEnglishDelta, PhonemizerEnglishVCCV, PhonemizerEnglishCV} {
		language, resolved, err := ResolveLanguage("en", phonemizer)
		if err != nil || language != LanguageEnglish || resolved != phonemizer {
			t.Fatalf("phonemizer=%q language=%q resolved=%q err=%v", phonemizer, language, resolved, err)
		}
	}
}
