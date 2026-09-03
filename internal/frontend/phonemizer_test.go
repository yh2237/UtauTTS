package frontend

import "testing"

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
	if len(units) != 2 || units[0].Aliases.Main[0] != "-h@" || units[1].Aliases.Main[0] != "l0" {
		t.Fatalf("units=%#v", units)
	}
	if units[1].Aliases.Transition[0] != "@l" {
		t.Fatalf("transition=%#v", units[1].Aliases.Transition)
	}
}

func TestEnglishCVVCKeepsFinalConsonant(t *testing.T) {
	_, delta, err := ParseEnglishDelta("", "K AE1 T", nil)
	if err != nil || len(delta) != 2 || delta[1].Aliases.Main[0] != "{ t" {
		t.Fatalf("delta=%#v err=%v", delta, err)
	}
	_, vccv, err := ParseEnglishVCCV("", "K AE1 T", nil)
	if err != nil || len(vccv) != 2 || vccv[1].Aliases.Main[0] != "At" {
		t.Fatalf("vccv=%#v err=%v", vccv, err)
	}
}

func TestParseChineseCVVC(t *testing.T) {
	reading, units, err := ParseChineseCVVC("你好", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "ni hao" || len(units) != 2 {
		t.Fatalf("reading=%q units=%#v", reading, units)
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
	if reading != "chong qing ren" {
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
