package tts

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestEnglishPhoneCodaDurationAdjustment(t *testing.T) {
	m := []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "t", Role: "onset"}}}, {Phones: []frontend.Phone{{Symbol: "t", Role: "coda"}}}, {Phones: []frontend.Phone{{Symbol: "s", Role: "coda"}}}}
	for i := range m {
		m[i].Language = frontend.LanguageEnglish
	}
	p := []prosody.Prediction{{DurationFactor: 1}, {DurationFactor: 1}, {DurationFactor: 1}}
	p = (englishProfile{}).AdjustPredictions(Config{}, nil, m, p, nil)
	if p[0].DurationMS != 42 || p[1].DurationMS < 60 || p[2].DurationMS <= p[1].DurationMS {
		t.Fatal(p)
	}
}
