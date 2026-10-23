package tts

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

func unifiedProsodyFixture(t testing.TB) (*prosody.Model, []frontend.Mora, []prosody.FeatureFrame, *plan.Plan) {
	t.Helper()
	model, err := prosody.LoadModel(filepath.Join("..", "..", "models", "frame-intonation-tcn-v12.json"))
	if err != nil {
		t.Fatal(err)
	}
	morae, err := frontend.ParseKana("あか、さな")
	if err != nil {
		t.Fatal(err)
	}
	features := []prosody.FeatureFrame{
		{"accent_phrase_start": 1, "word_start": 1, "pos=名詞": 1},
		{"accent_phrase_end": 1, "word_end": 1, "pos=名詞": 1},
		{},
		{"accent_phrase_start": 1, "word_start": 1, "pos=名詞": 1},
		{"accent_phrase_end": 1, "word_end": 1, "pos=名詞": 1},
	}
	timings := []prosody.MoraTiming{
		{StartMS: 0, DurationMS: 120}, {StartMS: 120, DurationMS: 120},
		{StartMS: 240, DurationMS: 180}, {StartMS: 420, DurationMS: 120}, {StartMS: 540, DurationMS: 120},
	}
	return model, morae, features, previewPlan(morae, timings, 660)
}

func TestUnifiedProsodyReleaseBaseline(t *testing.T) {
	model, morae, features, synthesisPlan := unifiedProsodyFixture(t)
	disabled := false
	cfg := Config{ApplyPitch: true, IntonationStrength: 4, BoundaryTone: &disabled}
	pitch, err := resolveSynthesisPitch(cfg, japaneseProfile{}, model, morae, features, "あか、さな", "ja", synthesisPlan)
	if err != nil {
		t.Fatal(err)
	}
	if pitch.Curve == nil || len(pitch.Curve.Cents) != 66 {
		t.Fatalf("curve = %+v", pitch.Curve)
	}
	const pitchToleranceCents = 1e-3
	for index, want := range map[int]float64{
		0: -31.3250548971411, 12: -4.87169909897725, 24: -22.5160530000444,
		42: -253.808609667546, 54: -158.255286184227, 65: -20.832039513697,
	} {
		if got := pitch.Curve.Cents[index]; !(math.Abs(got-want) <= pitchToleranceCents) {
			t.Fatalf("cents[%d] = %g, want %g (tolerance %g cents)", index, got, want, pitchToleranceCents)
		}
	}
	for index, want := range []float64{1.3, 1.3, 0.75, 1.3} {
		if got := synthesisPlan.Units[index].EnergyFactor; math.Abs(got-want) > 1e-9 {
			t.Fatalf("energy[%d] = %g, want %g", index, got, want)
		}
	}
	preview, err := PredictProsody(Config{Reading: "あか、さな", ProsodyModel: model, ProsodyFeatures: features,
		ApplyPitch: true, IntonationStrength: 4, BoundaryTone: &disabled, PauseContext: &disabled,
		ReleaseSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.FramePitchCurve == nil || len(preview.FramePitchCurve.Cents) != len(pitch.Curve.Cents) {
		t.Fatalf("preview curve = %+v", preview.FramePitchCurve)
	}
	for index, want := range pitch.Curve.Cents {
		if got := preview.FramePitchCurve.Cents[index]; got != want {
			t.Fatalf("preview cents[%d] = %g, synthesis = %g", index, got, want)
		}
	}
	cfg.ProsodyPitchOnly = true
	for index := range synthesisPlan.Units {
		synthesisPlan.Units[index].EnergyFactor = 1
	}
	if _, err := resolveSynthesisPitch(cfg, japaneseProfile{}, model, morae, features, "あか、さな", "ja", synthesisPlan); err != nil {
		t.Fatal(err)
	}
	for _, unit := range synthesisPlan.Units {
		if unit.EnergyFactor != 1 {
			t.Fatalf("pitch-only mode changed energy: %g", unit.EnergyFactor)
		}
	}
}

func BenchmarkResolveUnifiedProsody(b *testing.B) {
	model, morae, features, synthesisPlan := unifiedProsodyFixture(b)
	disabled := false
	cfg := Config{ApplyPitch: true, IntonationStrength: 4, BoundaryTone: &disabled}
	for _, mode := range []string{"cold", "warm"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if mode == "cold" {
					synthesisCache.clear()
				}
				if _, err := resolveSynthesisPitch(cfg, japaneseProfile{}, model, morae, features, "あか、さな", "ja", synthesisPlan); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
