package worldline

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/provider"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func TestWorldlineProviderJobCarriesCommonPlanAndResources(t *testing.T) {
	synthesisPlan := &plan.Plan{Version: plan.Version, Voicebank: "bank", Units: []plan.Unit{{Source: "voice.wav", DurationMS: 100}}}
	job, err := worldlineProviderJob(synthesisPlan, base.Config{ApplyPitch: true}, worldlineManifest{
		Engine: "utautts-world-phrase", OutputPath: "output.wav",
		SampleRate: 44100, F0Curve: []float64{220, 220},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Version != provider.UnitRendererJobVersion || job.Contract != "unit-renderer" ||
		len(job.Plan) == 0 || job.Resources != nil || !job.Options.ApplyPitch ||
		job.Options.Worldline == nil {
		t.Fatalf("job = %#v", job)
	}
	if job.Options.Worldline.Engine != "utautts-world-phrase" || job.Options.Worldline.SampleRate != 44100 {
		t.Fatalf("worldline options = %#v", job.Options.Worldline)
	}
}

func TestWorldlineProviderJobCarriesEnergyFactor(t *testing.T) {
	job, err := worldlineProviderJob(&plan.Plan{Version: plan.Version}, base.Config{}, worldlineManifest{
		Engine: "utautts-world-phrase", SampleRate: 44100,
		Units: []worldlineManifestUnit{{Source: "voice.wav", EnergyFactor: .65, LegacyMix: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := job.Options.Worldline.Units[0].EnergyFactor; got != .65 {
		t.Fatalf("energy factor = %f, want .65", got)
	}
	if !job.Options.Worldline.Units[0].LegacyMix {
		t.Fatal("legacy mix flag was not carried into the provider job")
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
	job, err := worldlineProviderJob(p, base.Config{}, worldlineManifest{Engine: "utautts-world-phrase", Units: []worldlineManifestUnit{{Speech: speech}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := job.Options.Worldline.Units[0].Speech; got == nil || !reflect.DeepEqual(got, speech) {
		t.Fatal("lost speech controls", got)
	}
}
