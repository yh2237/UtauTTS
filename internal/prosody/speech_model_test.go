package prosody

import (
	"math"
	"reflect"
	"testing"
	"utautts/internal/frontend"
)

func TestSpeechFeaturesDoNotCrossPauses(t *testing.T) {
	m := []frontend.Mora{{Language: "en", WordIndex: 0, Phones: []frontend.Phone{{Symbol: "AA", Role: "nucleus"}}, Stress: 1, StressKnown: true},
		{Pause: true}, {Language: "en", WordIndex: 1, Phones: []frontend.Phone{{Symbol: "D", Role: "coda"}}}}
	f := SpeechPhoneFeatures(m)
	has := func(features []string, key string) bool {
		for _, value := range features {
			if value == key {
				return true
			}
		}
		return false
	}
	if !has(f[0][0], "next=#") || !has(f[2][0], "prev=#") || !has(f[0][0], "stress=1") || !has(f[2][0], "stress=unknown") {
		t.Fatalf("incorrect features: %v", f)
	}
}

func TestEnglishFeaturesIgnoreAliasGrouping(t *testing.T) {
	phones := []frontend.Phone{{Symbol: "P", Role: "onset"}, {Symbol: "AA", Role: "nucleus"}, {Symbol: "D", Role: "coda"}}
	grouped := []frontend.Mora{{Language: "en", Phones: phones, Stress: 1, StressKnown: true}}
	separate := []frontend.Mora{}
	for _, phone := range phones {
		separate = append(separate, frontend.Mora{Language: "en", Phones: []frontend.Phone{phone}, Stress: 1, StressKnown: phone.Role == "nucleus"})
	}
	want := SpeechPhoneFeatures(grouped)[0]
	got := SpeechPhoneFeatures(separate)
	for i := range want {
		if !reflect.DeepEqual(want[i], got[i][0]) {
			t.Fatalf("alias grouping altered features: %v vs %v", want[i], got[i][0])
		}
	}
}

func TestSpeechModelRejectsInvalidProvenanceAndCoefficients(t *testing.T) {
	model := SpeechModel{Version: 1, FeatureVersion: 1, ID: "fixture", Language: "en", Corpus: "test", License: "test", TrainingDataKind: "natural", PhoneCounts: map[string]int{"aa": 5}, Duration: map[string]float64{"bias": 0}}
	if err := model.Validate(); err != nil {
		t.Fatal(err)
	}
	model.TrainingDataKind = "generated"
	if model.Validate() == nil {
		t.Fatal("generated training accepted as natural")
	}
	model.TrainingDataKind = "natural"
	model.Duration["bias"] = math.NaN()
	if model.Validate() == nil {
		t.Fatal("nonfinite coefficient accepted")
	}
}
