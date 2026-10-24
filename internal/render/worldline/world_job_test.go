package worldline

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/provider"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func TestWorldlineOptionsCarryManifest(t *testing.T) {
	options := worldlineOptions(&plan.Plan{Version: plan.Version}, base.Config{}, worldlineManifest{
		Engine: "utautts-world-phrase", SampleRate: 44100, F0Curve: []float64{220, 220},
	})
	if options.Engine != "utautts-world-phrase" || options.SampleRate != 44100 || len(options.F0Curve) != 2 {
		t.Fatalf("worldline options = %#v", options)
	}
}

func TestWorldlineOptionsCarryEnergyFactor(t *testing.T) {
	options := worldlineOptions(&plan.Plan{Version: plan.Version}, base.Config{}, worldlineManifest{
		Engine: "utautts-world-phrase", SampleRate: 44100,
		Units: []worldlineManifestUnit{{Source: "voice.wav", EnergyFactor: .65, LegacyMix: true}},
	})
	if got := options.Units[0].EnergyFactor; got != .65 {
		t.Fatalf("energy factor = %f, want .65", got)
	}
	if !options.Units[0].LegacyMix {
		t.Fatal("legacy mix flag was not carried into the WORLD options")
	}
}

func TestLegacyJapaneseContinuousMixOnlyUsesOrdinaryJapanesePlans(t *testing.T) {
	cases := []struct {
		name string
		plan *plan.Plan
		want bool
	}{
		{name: "japanese", plan: &plan.Plan{Language: "ja", Phonemizer: "ja-kana"}, want: true},
		{name: "single-cv", plan: &plan.Plan{Language: "ja", SingleCV: true}, want: false},
		{name: "english", plan: &plan.Plan{Language: "en", Phonemizer: "en-vccv"}, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := legacyJapaneseContinuousMix(test.plan); got != test.want {
				t.Fatalf("legacy mix = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWorldSpeechJobCarriesSpeechRequirement(t *testing.T) {
	speech := &provider.WorldSpeechTiming{UnitIndex: 0, SourceOnsetMS: 60, TargetOnsetMS: 40, ProtectStop: true}
	p := &plan.Plan{Units: []plan.Unit{{DurationMS: 120}}}
	options := worldlineOptions(p, base.Config{}, worldlineManifest{Engine: "utautts-world-phrase", Units: []worldlineManifestUnit{{Speech: speech}}})
	if got := options.Units[0].Speech; got == nil || !reflect.DeepEqual(got, speech) {
		t.Fatal("lost speech controls", got)
	}
}
