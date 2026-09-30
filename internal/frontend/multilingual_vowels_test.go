package frontend

import (
	"reflect"
	"testing"
)

func TestEnglishSchwaAliasesPreserveLexicalPhones(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parse        func(string, string, map[string]string) (string, []Mora, error)
		weak, strong string
	}{{"delta", ParseEnglishDelta, "@", "V"}, {"vccv", ParseEnglishVCCV, "x", "u"}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, phone := range []struct {
				vowel, symbol string
				stress        int
				stressKnown   bool
			}{
				{"AH0", "ah", 0, true}, {"AH1", "ah", 1, true}, {"AH2", "ah", 2, true},
				{"AH", "ah", 0, false}, {"AX", "ax", 0, false},
			} {
				vowel := phone.vowel
				reading, units, err := tc.parse("", "HH "+vowel+" N | SP", nil)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.strong
				if vowel == "AH0" || vowel == "AX" {
					want = tc.weak
				}
				if units[0].Vowel != want || reading != "HH "+vowel+" N | SP" {
					t.Fatal(vowel, reading, units)
				}
				if units[0].Phones[1].Symbol != phone.symbol || units[0].Stress != phone.stress || units[0].StressKnown != phone.stressKnown {
					t.Fatal("lexical stress/phone metadata changed", vowel, units[0])
				}
				if units[0].Aliases.Endings[0][0] != want+" n-" {
					t.Fatal("coda used wrong vowel", units[0].Aliases)
				}
				if vowel == "AH0" && !containsString(units[0].Aliases.Main, "h"+tc.strong) {
					t.Fatal("missing fallback for banks without schwa")
				}
			}
			_, units, err := tc.parse("", "S T R AH0 L OW1", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(units[0].Aliases.MainMissing["r"+tc.weak], []string{"s", "t"}) {
				t.Fatal("cluster coverage lost")
			}
			if units[1].Aliases.Transition[0] != tc.weak+" l" {
				t.Fatal("transition used stressed vowel")
			}
		})
	}
}

func TestChineseUmlautUsesPresampClassesAcrossSpellings(t *testing.T) {
	for _, pair := range [][2]string{{"lve", "lue"}, {"lue", "lve"}, {"nve", "nue"}, {"nue", "nve"}} {
		spelling, other := pair[0], pair[1]
		cfg := PresampConfig{Vowels: map[string]string{other: "e0", "li": "i"}, Consonants: map[string]string{other: "l", "li": "ly"}, Endings: []string{"%v% R"}}
		reading, units, err := ParseChineseCVVCWithConfig("", spelling+"4 li3 | "+spelling+"4", nil, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if reading != spelling+"4 li3 | "+spelling+"4" || units[0].Text != spelling || units[0].Tone != 4 {
			t.Fatal("reading changed")
		}
		if units[0].Vowel != "e0" || units[1].Aliases.Transition[0] != "e0 ly" || units[3].Aliases.Endings[0][0] != "e0 R" {
			t.Fatal("presamp class not propagated", units)
		}
		if !containsString(units[0].Aliases.Main, "- "+other) || len(units[0].Aliases.Main) != len(units[0].Aliases.MainKinds) {
			t.Fatal("alias/kind mismatch")
		}
		cfg.Vowels[spelling] = "custom"
		_, units, err = ParseChineseCVVCWithConfig("", spelling+"4", nil, cfg)
		if err != nil || units[0].Vowel != "custom" {
			t.Fatal("explicit class overwritten")
		}
	}
	// These vowels must stay distinct; only ue/ve spellings are interchangeable.
	for _, spelling := range []string{"nu", "lu", "nv", "lv", "ju", "qu", "xu"} {
		if got := chineseAliasSpellings(spelling); !reflect.DeepEqual(got, []string{spelling}) {
			t.Fatal("different vowels conflated", spelling, got)
		}
	}
	_, units, err := ParseChineseCVVCWithConfig("", "nüe4", nil, PresampConfig{Vowels: map[string]string{"nue": "e0"}})
	if err != nil || units[0].Vowel != "e0" || !containsString(units[0].Aliases.Main, "nue") {
		t.Fatal("ü normalization lost", units, err)
	}
}
