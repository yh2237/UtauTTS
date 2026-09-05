package frontend

import "testing"

func TestEnglishPunctuationPreservesPhrasePauses(t *testing.T) {
	for _, parser := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishARPAsing, ParseEnglishDelta, ParseEnglishVCCV} {
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

func TestParseEnglishARPAsingDictionary(t *testing.T) {
	_, units, err := ParseEnglishARPAsing("Hello", "", map[string]string{"hello": "HH AH L OW"})
	if err != nil || len(units) != 4 {
		t.Fatalf("units=%#v err=%v", units, err)
	}
}

func TestParseEnglishUsesBuiltInG2P(t *testing.T) {
	reading, units, err := ParseEnglishDelta("hello", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "HH AH L OW" || len(units) != 2 {
		t.Fatalf("reading=%q units=%#v", reading, units)
	}
}

func TestParseEnglishDelta(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Aliases.Main[0] != "- hV" || units[1].Aliases.Main[0] != "loU" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Transition[0] != "V l" {
		t.Fatalf("transition=%#v", units[1].Aliases.Transition)
	}
}

func TestParseEnglishVCCV(t *testing.T) {
	_, units, err := ParseEnglishVCCV("", "HH AH0 L OW1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Aliases.Main[0] != "-hu" || units[1].Aliases.Main[0] != "lO" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Transition[0] != "u l" {
		t.Fatalf("transition=%#v", units[1].Aliases.Transition)
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
}

func TestEnglishSyllabificationSplitsIllegalOnset(t *testing.T) {
	_, units, err := ParseEnglishDelta("", "AE1 T L AH0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[1].Consonant != "l" {
		t.Fatalf("units=%#v", units)
	}
	if len(units[0].Aliases.Endings) != 2 || units[0].Aliases.Endings[0][0] != "{ t" || units[0].Aliases.Endings[1][0] != "t l" {
		t.Fatalf("bridge=%#v", units[0].Aliases.Endings)
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
	if units[0].Aliases.Endings[0][0] != "E k" || units[0].Aliases.Endings[1][0] != "k str" {
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
	if len(units) != 2 || units[0].Aliases.Endings[0][0] != "{ t-" || units[1].Aliases.Main[0] != "- I" {
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
	for _, phonemizer := range []string{PhonemizerEnglish, PhonemizerEnglishDelta, PhonemizerEnglishVCCV} {
		language, resolved, err := ResolveLanguage("en", phonemizer)
		if err != nil || language != LanguageEnglish || resolved != phonemizer {
			t.Fatalf("phonemizer=%q language=%q resolved=%q err=%v", phonemizer, language, resolved, err)
		}
	}
}
