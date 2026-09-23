package plan

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/prosody"
	"utautts/internal/voicebank"
)

func pauseMorae(kind string, final bool) []frontend.Mora {
	morae := []frontend.Mora{{Text: "あ", Vowel: "a"}, {Pause: true, PauseKind: kind}}
	if !final {
		morae = append(morae, frontend.Mora{Text: "い", Vowel: "i"})
	}
	return morae
}

func TestPauseContextKindFactors(t *testing.T) {
	cfg := Config{PauseContext: true, PauseContextStrength: 1}
	cases := []struct {
		kind string
		want float64
	}{
		{frontend.PauseKindComma, pauseContextCommaFactor},
		{frontend.PauseKindPeriod, pauseContextPeriodFactor},
		{frontend.PauseKindQuestion, pauseContextQuestionFactor},
		{frontend.PauseKindEllipsis, pauseContextEllipsisFactor},
		{frontend.PauseKindSpace, pauseContextNeutralFactor},
		{frontend.PauseKindBracket, pauseContextNeutralFactor},
		{frontend.PauseKindOther, pauseContextNeutralFactor},
		{"", pauseContextNeutralFactor},
	}
	for _, testCase := range cases {
		morae := pauseMorae(testCase.kind, false)
		got := pauseContextFactor(morae, 1, cfg)
		if math.Abs(got-testCase.want) > 1e-9 {
			t.Errorf("kind %q factor = %v, want %v", testCase.kind, got, testCase.want)
		}
	}
}

func TestPauseContextUtteranceFinalIsLonger(t *testing.T) {
	cfg := Config{PauseContext: true, PauseContextStrength: 1}
	nonFinal := pauseContextFactor(pauseMorae(frontend.PauseKindPeriod, false), 1, cfg)
	final := pauseContextFactor(pauseMorae(frontend.PauseKindPeriod, true), 1, cfg)
	if math.Abs(final-nonFinal*pauseContextFinalFactor) > 1e-9 {
		t.Fatalf("final factor = %v, want %v", final, nonFinal*pauseContextFinalFactor)
	}
	if final <= nonFinal {
		t.Fatalf("utterance final pause was not longer: final=%v nonFinal=%v", final, nonFinal)
	}
}

func TestPauseContextKindOrdering(t *testing.T) {
	cfg := Config{PauseContext: true, PauseContextStrength: 1}
	comma := pauseContextFactor(pauseMorae(frontend.PauseKindComma, false), 1, cfg)
	period := pauseContextFactor(pauseMorae(frontend.PauseKindPeriod, false), 1, cfg)
	question := pauseContextFactor(pauseMorae(frontend.PauseKindQuestion, false), 1, cfg)
	if !(comma < period && period < question) {
		t.Fatalf("kind ordering = comma:%v period:%v question:%v", comma, period, question)
	}
}

func TestPauseContextStrengthAndDisable(t *testing.T) {
	morae := pauseMorae(frontend.PauseKindComma, false)
	if got := pauseContextFactor(morae, 1, Config{PauseContext: false, PauseContextStrength: 1}); got != 1 {
		t.Fatalf("disabled factor = %v, want 1", got)
	}
	// strength 0は既定1.0として扱う。
	if got := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: 0}); math.Abs(got-pauseContextCommaFactor) > 1e-9 {
		t.Fatalf("zero strength factor = %v, want %v", got, pauseContextCommaFactor)
	}
	// 負値は恒等。
	if got := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: -1}); got != 1 {
		t.Fatalf("negative strength factor = %v, want 1", got)
	}
	// 強度は中立1.0からの偏差へ掛かる。
	half := pauseContextFactor(morae, 1, Config{PauseContext: true, PauseContextStrength: 0.5})
	if math.Abs(half-0.85) > 1e-9 {
		t.Fatalf("half strength factor = %v, want 0.85", half)
	}
	// 上限を超える強度は2.0へクランプする。
	capped := pauseContextFactor(pauseMorae(frontend.PauseKindEllipsis, false), 1, Config{PauseContext: true, PauseContextStrength: 5})
	limit := pauseContextFactor(pauseMorae(frontend.PauseKindEllipsis, false), 1, Config{PauseContext: true, PauseContextStrength: pauseContextMaxStrength})
	if math.Abs(capped-limit) > 1e-9 {
		t.Fatalf("strength clamp = %v, want %v", capped, limit)
	}
}

func TestClampPauseContextFactor(t *testing.T) {
	if got := clampPauseContextFactor(0.1); got != pauseContextMinFactor {
		t.Fatalf("min clamp = %v, want %v", got, pauseContextMinFactor)
	}
	if got := clampPauseContextFactor(9); got != pauseContextMaxFactor {
		t.Fatalf("max clamp = %v, want %v", got, pauseContextMaxFactor)
	}
	if got := clampPauseContextFactor(1.2); got != 1.2 {
		t.Fatalf("passthrough = %v, want 1.2", got)
	}
}

func buildPausePlan(t *testing.T, morae []frontend.Mora, cfg Config) *Plan {
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
	morae := []frontend.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: frontend.PauseKindComma},
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
	morae := []frontend.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: frontend.PauseKindComma},
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
	morae := []frontend.Mora{
		{Text: "あ", Vowel: "a"},
		{Pause: true, PauseKind: frontend.PauseKindComma},
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
