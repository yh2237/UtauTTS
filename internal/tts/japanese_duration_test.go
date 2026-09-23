package tts

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func testMorae(texts ...string) []frontend.Mora {
	morae := make([]frontend.Mora, len(texts))
	for i, text := range texts {
		morae[i] = frontend.Mora{Text: text, Vowel: "a"}
	}
	return morae
}

func TestJapaneseContextDurationFactorsParticleIsShort(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{
		{},
		{"pos=助詞": 1},
		{},
	}
	factors := japaneseContextDurationFactors(morae, features, false)
	if factors[1] >= 1 {
		t.Fatalf("particle factor = %.4f, want < 1", factors[1])
	}
	if factors[1] != japaneseParticleFactor {
		t.Fatalf("particle factor = %.4f, want %.4f", factors[1], japaneseParticleFactor)
	}
}

func TestJapaneseContextDurationFactorsAccentPhraseEndIsLong(t *testing.T) {
	morae := testMorae("は", "な")
	features := []prosody.FeatureFrame{
		{"accent_phrase_end": 1},
		{},
	}
	factors := japaneseContextDurationFactors(morae, features, false)
	if factors[0] <= 1 {
		t.Fatalf("accent phrase end factor = %.4f, want > 1", factors[0])
	}
	if factors[0] != japaneseAccentPhraseEndFactor {
		t.Fatalf("accent phrase end factor = %.4f, want %.4f", factors[0], japaneseAccentPhraseEndFactor)
	}
}

func TestJapaneseContextDurationFactorsQuestionFinalIsLongest(t *testing.T) {
	morae := testMorae("か")
	features := []prosody.FeatureFrame{{"accent_phrase_end": 1}}
	statement := japaneseContextDurationFactors(morae, features, false)
	question := japaneseContextDurationFactors(morae, features, true)
	if question[0] <= statement[0] || statement[0] <= 1 {
		t.Fatalf("statement = %.4f question = %.4f", statement[0], question[0])
	}
}

func TestJapaneseContextDurationFactorsQuestionOnlyAtUtteranceEnd(t *testing.T) {
	morae := []frontend.Mora{
		{Text: "き", Vowel: "i"},
		{Text: "は", Vowel: "a"},
		{Pause: true},
		{Text: "な", Vowel: "a"},
	}
	features := []prosody.FeatureFrame{{"accent_phrase_start": 1}, {}, {}, {}}
	factors := japaneseContextDurationFactors(morae, features, true)
	if factors[1] != japanesePhraseFinalFactor {
		t.Fatalf("internal pause factor = %.4f, want %.4f", factors[1], japanesePhraseFinalFactor)
	}
	if factors[2] != 1 {
		t.Fatalf("pause mora factor = %.4f, want 1", factors[2])
	}
	if factors[3] != japaneseQuestionFinalFactor {
		t.Fatalf("utterance final factor = %.4f, want %.4f", factors[3], japaneseQuestionFinalFactor)
	}
}

func TestJapaneseContextDurationFactorsIdentityWithoutFeatures(t *testing.T) {
	morae := testMorae("き", "は", "な")
	for _, features := range [][]prosody.FeatureFrame{
		nil,
		{},
		{{"pos=助詞": 1}},
		{{"marker": 1}, {"marker": 2}, {"marker": 3}},
	} {
		factors := japaneseContextDurationFactors(morae, features, false)
		if len(factors) != len(morae) {
			t.Fatalf("factors length = %d, want %d", len(factors), len(morae))
		}
		for i, factor := range factors {
			if factor != 1 {
				t.Fatalf("features %v: factor[%d] = %.4f, want 1", features, i, factor)
			}
		}
	}
}

func TestApplyJapaneseContextDurationMultipliesExistingFactor(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{{}, {"pos=助詞": 1}, {}}
	predictions := []prosody.Prediction{
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
	}
	result := applyJapaneseContextDuration(Config{}, morae, features, predictions, false)
	if result[1].DurationFactor != 2*japaneseParticleFactor {
		t.Fatalf("particle factor = %.4f, want %.4f", result[1].DurationFactor, 2*japaneseParticleFactor)
	}
}

func TestApplyJapaneseContextDurationDisabledIsIdentity(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{{}, {"pos=助詞": 1}, {}}
	disabled := false
	predictions := []prosody.Prediction{
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
	}
	result := applyJapaneseContextDuration(Config{ContextDuration: &disabled}, morae, features, predictions, false)
	for i := range result {
		if result[i].DurationFactor != 2 {
			t.Fatalf("disabled factor[%d] = %.4f, want 2", i, result[i].DurationFactor)
		}
	}
}

func TestApplyJapaneseContextDurationScalesDeviationByStrength(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{{}, {"pos=助詞": 1}, {}}
	predictions := []prosody.Prediction{
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 2, PitchFactor: 1, EnergyFactor: 1},
	}
	half := applyJapaneseContextDuration(Config{ContextDurationStrength: 0.5}, morae, features, predictions, false)
	want := 2 * (1 + (japaneseParticleFactor-1)*0.5)
	if half[1].DurationFactor != want {
		t.Fatalf("half strength factor = %.4f, want %.4f", half[1].DurationFactor, want)
	}
	// 強度0.5では助詞の短縮偏差が半分になり、係数は中立へ近づく。
	if half[1].DurationFactor <= 2*japaneseParticleFactor {
		t.Fatalf("half strength %.4f should be closer to the base 2 than full %.4f", half[1].DurationFactor, 2*japaneseParticleFactor)
	}
}

func TestApplyJapaneseContextDurationZeroStrengthUsesDefault(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{{}, {"pos=助詞": 1}, {}}
	predictions := []prosody.Prediction{
		{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1},
		{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1},
	}
	result := applyJapaneseContextDuration(Config{ContextDurationStrength: 0}, morae, features, predictions, false)
	if result[1].DurationFactor != japaneseParticleFactor {
		t.Fatalf("zero strength factor = %.4f, want default %.4f", result[1].DurationFactor, japaneseParticleFactor)
	}
}

func TestApplyJapaneseContextDurationInitializesMissingPredictions(t *testing.T) {
	morae := testMorae("き", "は", "な")
	features := []prosody.FeatureFrame{{}, {"pos=助詞": 1}, {}}
	result := applyJapaneseContextDuration(Config{}, morae, features, nil, false)
	if len(result) != len(morae) {
		t.Fatalf("predictions length = %d, want %d", len(result), len(morae))
	}
	if result[0].PitchFactor != 1 || result[0].EnergyFactor != 1 {
		t.Fatalf("defaults not filled: %+v", result[0])
	}
	if result[1].DurationFactor != japaneseParticleFactor {
		t.Fatalf("particle factor = %.4f, want %.4f", result[1].DurationFactor, japaneseParticleFactor)
	}
}
