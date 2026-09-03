package tts

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

func TestSynthesizeHonorsCanceledContextBeforeLoadingInputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Synthesize(Config{Context: ctx, VoicebankPath: filepath.Join(t.TempDir(), "missing")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestAliasProfilesBundleSelectionAndRendererSettings(t *testing.T) {
	legacy := Config{AliasPolicy: voicebank.AliasPolicyLegacy, CVVCTiming: render.CVVCTimingSequential, CVVCTransitionGain: 0.2, CVVCPreBoundaryFade: true}
	applyAliasProfile(nil, &legacy)
	if legacy.AliasPolicy != voicebank.AliasPolicyAuto || legacy.CVVCTiming != render.CVVCTimingLegacy || legacy.CVVCTransitionGain != 1 || legacy.CVVCPreBoundaryFade {
		t.Fatalf("legacy profile = %+v", legacy)
	}

	enhanced := Config{AliasPolicy: voicebank.AliasPolicyEnhanced}
	applyAliasProfile(nil, &enhanced)
	if enhanced.AliasPolicy != voicebank.AliasPolicyCVVCPrefer || enhanced.CVVCTiming != render.CVVCTimingSequential || enhanced.CVVCTransitionGain != 0.35 || enhanced.CVVCPreBoundaryFade {
		t.Fatalf("enhanced profile = %+v", enhanced)
	}

	expert := Config{AliasPolicy: voicebank.AliasPolicyVCVPrefer, CVVCTiming: render.CVVCTimingSequential, CVVCTransitionGain: 0.6}
	applyAliasProfile(nil, &expert)
	if expert.AliasPolicy != voicebank.AliasPolicyVCVPrefer || expert.CVVCTiming != render.CVVCTimingSequential || expert.CVVCTransitionGain != 0.6 {
		t.Fatalf("expert settings were overwritten: %+v", expert)
	}
}

func TestMoraTimingsIncludePausesMissingFromPlanUnits(t *testing.T) {
	morae := []frontend.Mora{{Text: "a"}, {Pause: true}, {Text: "i"}}
	p := &plan.Plan{DurationMS: 380, Units: []plan.Unit{
		{Position: 0, NoteStartMS: 0, DurationMS: 100},
		{Position: 2, NoteStartMS: 280, DurationMS: 100},
	}}
	got := moraTimings(morae, p)
	if len(got) != 3 || got[0].DurationMS != 100 || got[1].StartMS != 100 || got[1].DurationMS != 180 || got[2].StartMS != 280 {
		t.Fatalf("timings=%+v", got)
	}
}

func TestMergeManualPitchCurveAddsToLearnedCurve(t *testing.T) {
	base := &render.PitchCurve{FrameMS: 20, Cents: []float64{10, 30, 50}}
	manual := &prosody.PitchContour{FrameMS: 10, Cents: []float64{0, 10, 20, 30, 40}}
	got := mergeManualPitchCurve(base, manual, "offset")
	want := []float64{10, 30, 50, 70, 90}
	for index, value := range want {
		if math.Abs(got.Cents[index]-value) > 1e-9 {
			t.Fatalf("merged[%d] = %.2f, want %.2f", index, got.Cents[index], value)
		}
	}
}

func TestMergeManualPitchCurveCanReplaceLearnedCurve(t *testing.T) {
	base := &render.PitchCurve{FrameMS: 10, Cents: []float64{100, 100}}
	manual := &prosody.PitchContour{FrameMS: 10, Cents: []float64{0, -20}}
	got := mergeManualPitchCurve(base, manual, "replace")
	if got.Cents[0] != 0 || got.Cents[1] != -20 {
		t.Fatalf("replacement curve = %#v", got.Cents)
	}
}

func TestScaleAutomaticPitchCurveUsesTheConfiguredStrength(t *testing.T) {
	base := &render.PitchCurve{FrameMS: 10, Cents: []float64{20, -40}}
	if got := scaleAutomaticPitchCurve(base, 0); got != nil {
		t.Fatal("zero intonation strength kept the automatic curve")
	}
	if got := scaleAutomaticPitchCurve(base, 1); got != base {
		t.Fatal("normal intonation strength unnecessarily copied the curve")
	}
	got := scaleAutomaticPitchCurve(base, 2)
	if got == base || !reflect.DeepEqual(got.Cents, []float64{40, -80}) {
		t.Fatalf("amplified curve = %#v", got)
	}
	if !reflect.DeepEqual(base.Cents, []float64{20, -40}) {
		t.Fatal("amplifying the curve mutated the source")
	}
}

func TestIntonationStrengthAcceptsAmplificationRange(t *testing.T) {
	if err := validateConfig(Config{IntonationStrength: render.MaxIntonationStrength}); err != nil {
		t.Fatalf("maximum intonation strength rejected: %v", err)
	}
	if err := validateConfig(Config{IntonationStrength: render.MaxIntonationStrength + 0.01}); err == nil {
		t.Fatal("intonation strength above the maximum was accepted")
	}
}

func TestPredictProsodyDoesNotRenderAudio(t *testing.T) {
	features := []prosody.FeatureFrame{{"marker": 1}, {"marker": 2}, {"marker": 3}}
	preview, err := PredictProsody(Config{
		Reading:         "あいう",
		ProsodyFeatures: features,
		MoraDurationMS:  100,
		PauseDurationMS: 180,
		ApplyPitch:      false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reading != "あいう" || len(preview.Morae) != 3 {
		t.Fatalf("unexpected preview reading/morae: %#v", preview)
	}
	if !reflect.DeepEqual(preview.MoraDurationsMS, []float64{100, 100, 100}) {
		t.Fatalf("durations = %#v", preview.MoraDurationsMS)
	}
	if !reflect.DeepEqual(preview.MoraPositionsMS, []float64{50, 150, 250}) {
		t.Fatalf("positions = %#v", preview.MoraPositionsMS)
	}
	if !reflect.DeepEqual(preview.PitchPoints, []float64{0, 0, 0}) {
		t.Fatalf("pitch points = %#v", preview.PitchPoints)
	}
	if !reflect.DeepEqual(preview.Features, features) {
		t.Fatalf("features = %#v", preview.Features)
	}
}

func TestMoraTimingsDistributeConsecutiveTrailingPauses(t *testing.T) {
	morae := []frontend.Mora{{Text: "a"}, {Pause: true}, {Pause: true}}
	p := &plan.Plan{DurationMS: 300, Units: []plan.Unit{{Position: 0, NoteStartMS: 0, DurationMS: 100}}}
	got := moraTimings(morae, p)
	if got[1].DurationMS != 100 || got[2].StartMS != 200 || got[2].DurationMS != 100 {
		t.Fatalf("timings=%+v", got)
	}
}

func TestExternalPitchFactorsDoNotImplicitlyEnableWaveformPitchProcessing(t *testing.T) {
	if applyPitchEnabled(Config{PitchFactors: []float64{1.02}}) {
		t.Fatal("external pitch targets implicitly enabled waveform pitch processing")
	}
	if !applyPitchEnabled(Config{PitchFactors: []float64{1.02}, ApplyPitch: true}) {
		t.Fatal("explicit ApplyPitch did not enable waveform pitch processing")
	}
	if !applyPitchEnabled(Config{ProsodyPitchOnly: true}) {
		t.Fatal("ProsodyPitchOnly did not enable pitch processing")
	}
}

func TestPitchProcessingSwitchControlsModelFrameContour(t *testing.T) {
	model := &prosody.Model{FramePitch: &prosody.FramePitchModel{}}
	capabilities := &plugin.Capabilities{FramePitch: true}
	if shouldPredictFrameContour(Config{RendererCapabilities: capabilities}, model) {
		t.Fatal("model frame contour was enabled while pitch processing was off")
	}
	if !shouldPredictFrameContour(Config{ApplyPitch: true, RendererCapabilities: capabilities}, model) {
		t.Fatal("model frame contour was disabled while pitch processing was on")
	}
	if got := effectiveIntonationStrength(Config{IntonationStrength: 0.5}); got != 0 {
		t.Fatalf("disabled pitch processing kept intonation strength %.2f", got)
	}
	if got := effectiveIntonationStrength(Config{ApplyPitch: true, IntonationStrength: 0.5}); got != 0.5 {
		t.Fatalf("enabled pitch processing changed intonation strength to %.2f", got)
	}
}

func TestWaveformRendererSupportsFramePitch(t *testing.T) {
	if !rendererSupportsFramePitch("waveform", nil) {
		t.Fatal("waveform renderer rejected a frame pitch contour")
	}
}

func TestApplyResolvedRendererPropagatesExternalResamplerOptions(t *testing.T) {
	velocity, modulation := 86, 4
	cfg := Config{}
	ApplyResolvedRenderer(&cfg, plugin.Renderer{
		Backend: "utau-external-resampler",
		Assets:  map[string]string{"resampler": "resampler.exe", "wavtool": "wavtool.exe"},
		ResamplerOptions: &plugin.ResamplerOptions{
			Velocity: &velocity, Flags: "g-3Mt10", Modulation: &modulation, Tempo: 150,
		},
	}, "", "")
	if cfg.ExternalResamplerPath != "resampler.exe" || cfg.ExternalWavtoolPath != "wavtool.exe" || !cfg.ExternalResamplerVelocitySet || cfg.ExternalResamplerVelocity != 86 ||
		cfg.ExternalResamplerFlags != "g-3Mt10" || !cfg.ExternalResamplerModulationSet || cfg.ExternalResamplerModulation != 4 ||
		cfg.ExternalResamplerTempo != 150 {
		t.Fatalf("external resampler options were not propagated: %#v", cfg)
	}
}

func TestAlignRuntimeProsodyFeaturesAcceptsAlternatePronunciations(t *testing.T) {
	morae := []frontend.Mora{
		{Text: "\u3044", Vowel: "i"},
		{Text: "\u304b", Vowel: "a"},
		{Text: "\u308a", Vowel: "i"},
	}
	analysis := &openjtalk.Analysis{
		Morae: []string{"\u304a", "\u3053", "\u308a"},
		Features: []prosody.FeatureFrame{
			{"source": 0},
			{"source": 1},
			{"source": 2},
		},
	}
	aligned, err := alignRuntimeProsodyFeatures(morae, analysis)
	if err != nil {
		t.Fatal(err)
	}
	for index, source := range []float64{0, 1, 2} {
		if aligned[index]["source"] != source {
			t.Fatalf("aligned[%d] = %#v, want source %.0f", index, aligned[index], source)
		}
	}
}

func TestAlignRuntimeProsodyFeaturesAcceptsLongVowelNotation(t *testing.T) {
	morae := []frontend.Mora{{Text: "\u305b", Vowel: "e"}, {Text: "\u3044", Vowel: "i"}, {Text: "\u3044", Vowel: "i"}}
	analysis := &openjtalk.Analysis{
		Morae:    []string{"\u305b", "\u30fc", "\u3044"},
		Features: []prosody.FeatureFrame{{"source": 0}, {"source": 1}, {"source": 2}},
	}
	aligned, err := alignRuntimeProsodyFeatures(morae, analysis)
	if err != nil {
		t.Fatalf("long vowel notation was rejected: %v", err)
	}
	if aligned[1]["source"] != 1 {
		t.Fatalf("aligned long vowel feature = %#v", aligned[1])
	}
}

func TestAlignRuntimeProsodyFeaturesFillsMissingGoMora(t *testing.T) {
	morae := []frontend.Mora{{Text: "こ"}, {Text: "れ"}, {Text: "い"}}
	analysis := &openjtalk.Analysis{
		Morae:    []string{"こ", "れ"},
		Features: []prosody.FeatureFrame{{"source": 0}, {"source": 1}},
	}
	aligned, err := alignRuntimeProsodyFeatures(morae, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(aligned) != len(morae) || aligned[2]["source"] != 1 {
		t.Fatalf("aligned features = %#v", aligned)
	}
}

func TestAlignRuntimeProsodyFeaturesSkipsExtraOpenJTalkMorae(t *testing.T) {
	morae := []frontend.Mora{{Text: "\u3053"}, {Text: "\u3044"}}
	analysis := &openjtalk.Analysis{
		Morae: []string{"\u3053", "\u304f", "\u3089", "\u3044"},
		Features: []prosody.FeatureFrame{
			{"source": 0},
			{"source": 1},
			{"source": 2},
			{"source": 3},
		},
	}
	aligned, err := alignRuntimeProsodyFeatures(morae, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(aligned) != 2 || aligned[0]["source"] != 0 || aligned[1]["source"] != 3 {
		t.Fatalf("aligned features = %#v", aligned)
	}
}

func TestAlignRuntimeProsodyFeaturesAcceptsAlternatePronunciation(t *testing.T) {
	morae := []frontend.Mora{{Text: "\u3044"}, {Text: "\u304b"}, {Text: "\u308a"}}
	analysis := &openjtalk.Analysis{
		Morae: []string{"\u304a", "\u3053", "\u308a"},
		Features: []prosody.FeatureFrame{
			{"source": 0},
			{"source": 1},
			{"source": 2},
		},
	}
	aligned, err := alignRuntimeProsodyFeatures(morae, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(aligned) != 3 || aligned[0]["source"] != 0 || aligned[2]["source"] != 2 {
		t.Fatalf("aligned features = %#v", aligned)
	}
}

func TestValidateConfigRejectsNonFiniteValues(t *testing.T) {
	for _, cfg := range []Config{
		{MoraDurationMS: math.NaN()},
		{PauseDurationMS: math.Inf(1)},
		{ReleaseMS: math.Inf(-1)},
		{PitchFactors: []float64{math.NaN()}},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("accepted invalid config: %#v", cfg)
		}
	}
}

func TestConvertToReadingUsesBuiltInTokenizer(t *testing.T) {
	reading, err := ConvertToReading("こんにちは。", nil, openjtalk.Config{})
	if err != nil || reading == "" {
		t.Fatalf("kana text failed: %v", err)
	}
}

func TestConvertToReadingReportsOpenJTalkFallback(t *testing.T) {
	// 存在しないhelperを指定し、同梱物に依存せずフォールバック失敗を再現する。
	_, err := ConvertToReading("2024年です。", nil, openjtalk.Config{
		HelperPath: filepath.Join(t.TempDir(), "missing-helper"),
	})
	if err == nil {
		t.Fatal("text the tokenizer cannot read was silently accepted")
	}
	if !strings.Contains(err.Error(), "convert text to reading") || !strings.Contains(err.Error(), "Open JTalk fallback") {
		t.Fatalf("combined fallback error missing context: %v", err)
	}
}

func TestVoicebankCacheInvalidatedByClearCaches(t *testing.T) {
	bankDir := t.TempDir()
	if err := os.MkdirAll(bankDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := loadVoicebankCached(bankDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadVoicebankCached(bankDir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("voicebank was loaded more than once")
	}
	ClearCaches()
	third, err := loadVoicebankCached(bankDir)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("voicebank cache survived ClearCaches")
	}
}

func TestSynthesizePropagatesAliasPolicyIntoPlan(t *testing.T) {
	root := t.TempDir()
	samples := make([]int16, 8000)
	for index := range samples {
		samples[index] = int16(3000 * math.Sin(2*math.Pi*220*float64(index)/16000))
	}
	if err := audio.WriteWav(filepath.Join(root, "source.wav"), &audio.PCM{SampleRate: 16000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte(
		"source.wav=- あ,0,100,0,50,10\n"+"source.wav=a か,0,100,0,50,10\n"+"source.wav=あ,0,100,0,50,10\n"+"source.wav=か,0,100,0,50,10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Synthesize(Config{
		VoicebankPath: root, Reading: "あか", MoraDurationMS: 100,
		AliasPolicy: voicebank.AliasPolicyCVOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.AliasPolicy != string(voicebank.AliasPolicyCVOnly) || len(result.Plan.Units) != 2 {
		t.Fatalf("plan policy/units = %#v", result.Plan)
	}
	for _, unit := range result.Plan.Units {
		if unit.AliasKind != string(voicebank.AliasCV) {
			t.Fatalf("cv-only unit = %#v", unit)
		}
	}
}

func TestSynthesizeUsesCVVCTransitionInAutoMode(t *testing.T) {
	root := t.TempDir()
	samples := make([]int16, 8000)
	for index := range samples {
		samples[index] = int16(3000 * math.Sin(2*math.Pi*220*float64(index)/16000))
	}
	if err := audio.WriteWav(filepath.Join(root, "source.wav"), &audio.PCM{SampleRate: 16000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte(
		"source.wav=あ,0,100,0,50,10\n"+"source.wav=か,0,100,0,50,10\n"+"source.wav=a k,0,100,0,50,10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Synthesize(Config{VoicebankPath: root, Reading: "あか", MoraDurationMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Units) != 3 || result.Plan.Units[1].Role != "transition" || result.Plan.Units[1].Alias != "a k" {
		t.Fatalf("CVVC plan = %#v", result.Plan.Units)
	}
	if len(result.MoraDurationsMS) != 2 || result.MoraDurationsMS[0] != 100 || result.MoraDurationsMS[1] != 100 {
		t.Fatalf("mora timings = %#v", result.MoraDurationsMS)
	}
}
