package plan

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func pauseMorae(kind string, final bool) []linguistics.Mora {
	morae := []linguistics.Mora{{Text: "あ", Vowel: "a"}, {Pause: true, PauseKind: kind}}
	if !final {
		morae = append(morae, linguistics.Mora{Text: "い", Vowel: "i"})
	}
	return morae
}

func TestPauseContextKindOrdering(t *testing.T) {
	cfg := Config{PauseContext: true, PauseContextStrength: 1}
	factor := func(kind string, final bool) float64 { return pauseContextFactor(pauseMorae(kind, final), 1, cfg) }
	comma, period, question, ellipsis := factor(linguistics.PauseKindComma, false), factor(linguistics.PauseKindPeriod, false),
		factor(linguistics.PauseKindQuestion, false), factor(linguistics.PauseKindEllipsis, false)
	if !(comma < period && period < question && question < ellipsis) {
		t.Fatalf("kind ordering = comma:%v period:%v question:%v ellipsis:%v", comma, period, question, ellipsis)
	}
	for _, kind := range []string{linguistics.PauseKindSpace, linguistics.PauseKindBracket, linguistics.PauseKindOther, ""} {
		if got := factor(kind, false); got != period {
			t.Errorf("kind %q factor = %v, want neutral %v", kind, got, period)
		}
	}
	if final := factor(linguistics.PauseKindPeriod, true); final <= period {
		t.Fatalf("utterance final pause was not longer: final=%v nonFinal=%v", final, period)
	}
}

func TestPauseContextStrengthAndDisable(t *testing.T) {
	morae := pauseMorae(linguistics.PauseKindComma, false)
	if got := pauseContextFactor(morae, 1, Config{PauseContext: false, PauseContextStrength: 1}); got != 1 {
		t.Fatalf("disabled factor = %v, want 1", got)
	}
	if got := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: 0}); math.Abs(got-pauseContextCommaFactor) > 1e-9 {
		t.Fatalf("zero strength factor = %v, want %v", got, pauseContextCommaFactor)
	}
	if got := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: -1}); got != 1 {
		t.Fatalf("negative strength factor = %v, want 1", got)
	}
	half := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: 0.5})
	if math.Abs(half-0.85) > 1e-9 {
		t.Fatalf("half strength factor = %v, want 0.85", half)
	}
	capped := pauseContextFactor(pauseMorae(linguistics.PauseKindEllipsis, false), 1, Config{PauseContext: true, PauseContextStrength: 5})
	limit := pauseContextFactor(pauseMorae(linguistics.PauseKindEllipsis, false), 1, Config{PauseContext: true, PauseContextStrength: pauseContextMaxStrength})
	if math.Abs(capped-limit) > 1e-9 {
		t.Fatalf("strength clamp = %v, want %v", capped, limit)
	}
}

func buildPausePlan(t *testing.T, morae []linguistics.Mora, cfg Config) *Plan {
	t.Helper()
	bank := &voicebank.Bank{Root: "bank"}
	selections := []voicebank.Selection{
		{Position: 0, Mora: morae[0], Alias: "あ", Entry: oto.Entry{Filename: "a.wav"}},
		{Position: 2, Mora: morae[2], Alias: "い", Entry: oto.Entry{Filename: "i.wav"}},
	}
	got, err := Build(bank, "あ、い", morae, selections, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestBuildAppliesPauseContext(t *testing.T) {
	morae := []linguistics.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: linguistics.PauseKindComma},
		{Text: "い", Vowel: "i"},
	}
	base := Config{MoraDurationMS: 100, PauseDurationMS: 200}
	off := buildPausePlan(t, morae, base)
	if off.Units[1].NoteStartMS != 300 {
		t.Fatalf("disabled pause start = %v, want 300", off.Units[1].NoteStartMS)
	}
	on := buildPausePlan(t, morae, Config{MoraDurationMS: 100, PauseDurationMS: 200, PauseContext: true, PauseContextStrength: 1})
	wantPause := 200 * pauseContextCommaFactor
	if math.Abs(on.Units[1].NoteStartMS-(100+wantPause)) > 1e-9 {
		t.Fatalf("comma pause start = %v, want %v", on.Units[1].NoteStartMS, 100+wantPause)
	}
}

func TestBuildPauseContextManualOverrideWins(t *testing.T) {
	morae := []linguistics.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: linguistics.PauseKindComma},
		{Text: "い", Vowel: "i"},
	}
	got := buildPausePlan(t, morae, Config{
		MoraDurationMS: 100, PauseDurationMS: 200,
		PauseContext: true, PauseContextStrength: 1,
		MoraDurationsMS: []float64{0, 500, 0},
	})
	if got.Units[1].NoteStartMS != 600 {
		t.Fatalf("manual pause override = %v, want 600", got.Units[1].NoteStartMS)
	}
}

func TestBuildPauseContextPredictionWins(t *testing.T) {
	morae := []linguistics.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: linguistics.PauseKindComma},
		{Text: "い", Vowel: "i"},
	}
	got := buildPausePlan(t, morae, Config{
		MoraDurationMS: 100, PauseDurationMS: 200,
		PauseContext: true, PauseContextStrength: 1,
		Predictions: []prosody.Prediction{{}, {DurationMS: 333}, {}},
	})
	if got.Units[1].NoteStartMS != 433 {
		t.Fatalf("predicted pause duration = %v, want 433", got.Units[1].NoteStartMS)
	}
}
