package frontend

import (
	"strings"
	"testing"
)

func TestDictionaryPreservesStressBeyondMiniDictionary(t *testing.T) {
	for _, word := range []string{"hello", "international", "pronunciation", "beautiful"} {
		reading, err := englishWordPronunciation(word)
		if err != nil || !strings.Contains(reading, "1") {
			t.Fatalf("%s: %q %v", word, reading, err)
		}
	}
	reading, _, err := ParseEnglishARPAsing("hello", "", map[string]string{"hello": "HH EH1 L OW0"})
	if err != nil || reading != "HH EH1 L OW0" {
		t.Fatalf("user dictionary: %q %v", reading, err)
	}
}

func TestSpeechNumberNormalization(t *testing.T) {
	for input, want := range map[string]string{"3:15": "three fifteen", "3:05": "three oh five", "120": "one hundred twenty", "0.25": "zero point two five", "007": "zero zero seven"} {
		got := strings.Join(strings.Fields(normalizeEnglishText(input)), " ")
		if got != want {
			t.Errorf("%q: %q want %q", input, got, want)
		}
	}
	for input, want := range map[string]string{"2026年9月9日": "二零二六年九月九日", "101": "一百零一", "110": "一百一十", "10010": "一万零一十", "3.14": "三点一四"} {
		if got := normalizeChineseText(input); got != want {
			t.Errorf("%q: %q want %q", input, got, want)
		}
	}
	reading, _, err := ParseEnglishARPAsing("Meet me at 3:15.", "", nil)
	if err != nil || !strings.Contains(reading, "TH R IY1") {
		t.Fatalf("number lost: %q %v", reading, err)
	}
}

func TestChineseWordReadingAndWhitespace(t *testing.T) {
	reading, units, err := ParseChineseCVVC("银行行长来了。", "", nil)
	if err != nil || reading != "yin2 hang2 hang2 zhang3 lai2 le5 |" {
		t.Fatalf("%q %v", reading, err)
	}
	if units[0].WordEnd || !units[1].WordEnd || units[0].WordIndex != units[1].WordIndex {
		t.Fatalf("word boundaries: %+v", units)
	}
	reading, _, err = ParseChineseCVVC("你好 世界", "", nil)
	if err != nil || strings.Contains(reading, "|") {
		t.Fatalf("spaces became pauses: %q %v", reading, err)
	}
}

func TestEnglishSpeechTimingDistinguishesPhoneClasses(t *testing.T) {
	_, units, err := ParseEnglishARPAsing("", "S T AE1", nil)
	if err != nil || units[0].DurationScale <= units[1].DurationScale {
		t.Fatalf("%+v %v", units, err)
	}
	if _, _, err := ParseEnglishARPAsing("", "K INVALID AE1", nil); err == nil {
		t.Fatal("invalid phone accepted")
	}
}

func TestEnglishWordBoundaryIsNotPhraseBoundary(t *testing.T) {
	for _, parser := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishDelta, ParseEnglishVCCV} {
		_, units, err := parser("", "K AE1 T | IH1 Z | SP | K AE1 T", nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(units[1].Aliases.Main[0], "-") || !strings.HasPrefix(units[3].Aliases.Main[0], "-") {
			t.Fatalf("phrase context: %+v", units)
		}
		if len(units[0].Aliases.EndingPhones) != 1 || units[0].Aliases.EndingPhones[0][0] != "t" {
			t.Fatal("required coda missing")
		}
	}
}
