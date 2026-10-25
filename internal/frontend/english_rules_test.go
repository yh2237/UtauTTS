package frontend

import "testing"

func TestEnglishOOVAcrossPhonemizersAndDictionaryOverride(t *testing.T) {
	for _, parse := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishARPAsing, func(text, reading string, dictionary map[string]string) (string, []Mora, error) {
		return ParseEnglishDeltaWithConfig(text, reading, dictionary, PresampConfig{})
	}, ParseEnglishVCCV, ParseEnglishCV} {
		reading, units, err := parse("phlame", "", nil)
		if err != nil || reading != "F L EY M" || len(units) == 0 {
			t.Fatalf("%q %v", reading, err)
		}
		for _, unit := range units {
			if unit.StressKnown {
				t.Fatalf("unknown word gained stress: %+v", unit)
			}
		}
		reading, _, err = parse("phlame", "", map[string]string{"phlame": "F L AE1 M"})
		if err != nil || reading != "F L AE1 M" {
			t.Fatalf("override: %q %v", reading, err)
		}
		reading, _, err = parse("phlame", "F L IY1 M", nil)
		if err != nil || reading != "F L IY1 M" {
			t.Fatalf("explicit reading: %q %v", reading, err)
		}
	}
}
