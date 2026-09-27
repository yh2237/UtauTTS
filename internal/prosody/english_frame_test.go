package prosody

import (
	"reflect"
	"testing"
	"utautts/internal/frontend"
)

func TestEnglishFrameFeaturesIgnoreBankAliases(t *testing.T) {
	_, delta, err := frontend.ParseEnglishDelta("", "HH AH0 L OW1 | W ER1 L D", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, vccv, err := frontend.ParseEnglishVCCV("", "HH AH0 L OW1 | W ER1 L D", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range delta {
		if !reflect.DeepEqual(featuresFor(delta, i), featuresFor(vccv, i)) {
			t.Fatal("bank alias changed language features")
		}
	}
	f := featuresFor(delta, 1)
	if f["syllable_stress=1"] != 1 || f["en_word_end"] != 1 || f["en_word_start"] != 0 || f["syllable_nucleus=ow"] != 1 {
		t.Fatal(f)
	}
	m := &Model{FramePitch: &FramePitchModel{FeatureNames: []string{"en_word_end", "syllable_stress=1"}}}
	if m.RequiresExternalFeatures() {
		t.Fatal("English features must be available in Go")
	}
}

func TestEnglishFramePauseFeaturesMatchTraining(t *testing.T) {
	units := []frontend.Mora{{Language: "en", WordIndex: 0, WordEnd: true, Phones: []frontend.Phone{{Symbol: "eh", Role: "nucleus"}}, Stress: 1, StressKnown: true}, {Pause: true}}
	f := featuresFor(units, 1)
	if f["syllable=<PAUSE>"] != 1 || f["prev_nucleus=eh"] != 1 || f["prev_stress=1"] != 1 || f["next=<EOS>"] != 1 || f["en_word_end"] != 0 {
		t.Fatal(f)
	}
}
