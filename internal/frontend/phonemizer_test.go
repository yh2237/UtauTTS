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
