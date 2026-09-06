package tts

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/diffsinger"
	"utautts/internal/frontend"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func TestDiffSingerIsRegisteredAsNeuralSynthesizer(t *testing.T) {
	synthesizer, found := neuralSynthesizerForProvider("diffsinger")
	if !found || synthesizer.ProviderID() != "diffsinger" {
		t.Fatalf("DiffSinger neural provider = %#v, found=%v", synthesizer, found)
	}
	if _, found := neuralSynthesizerForProvider("waveform"); found {
		t.Fatal("unit renderer was registered as a neural synthesizer")
	}
}

func TestDiffSingerUsesSelectedSpeechModel(t *testing.T) {
	morae, _ := frontend.ParseKana("あい")
	model := &prosody.Model{Version: prosody.FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
		FramePitch: &prosody.FramePitchModel{FeatureNames: []string{"mora_progress"}, InputWeights: [][]float64{{2}}, InputBias: []float64{0}, OutputWeight: []float64{100}, FrameMS: 10, LowCents: -60, HighCents: 60},
	}
	cfg, _, err := prepareDiffSingerProsody(Config{Reading: "あい", ProsodyModel: model, ApplyPitch: true, IntonationStrength: 1, RendererCapabilities: &plugin.Capabilities{FramePitch: true}}, "あい", morae, 10)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PitchCurve == nil {
		t.Fatal("selected model did not reach DiffSinger")
	}
	low, high := math.Inf(1), math.Inf(-1)
	for _, value := range cfg.PitchCurve.Cents {
		low = math.Min(low, value)
		high = math.Max(high, value)
	}
	if high-low < 1 {
		t.Fatal("speech model contour was flattened")
	}
}

func TestDiffSingerSpeechProsodyPreservesManualTimingAndPitch(t *testing.T) {
	morae, err := frontend.ParseKana("あい")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Reading: "あい", Renderer: "diffsinger", MoraDurationMS: 120,
		MoraDurationsMS: []float64{90, 150},
		ManualPitch: &prosody.ManualPitchFile{Version: 1, Reading: "あい", Mode: "replace",
			Points: []prosody.ManualPitchPoint{{Position: 0, Cents: 120}, {Position: 1, Cents: -120}}},
	}
	prepared, preview, err := prepareDiffSingerProsody(cfg, "あい", morae, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.MoraDurationsMS, cfg.MoraDurationsMS) {
		t.Fatalf("durations = %v", preview.MoraDurationsMS)
	}
	for _, point := range preview.PitchPoints {
		if point != 0 {
			t.Fatal("manual pitch leaked into automatic preview")
		}
	}
	padding := diffsinger.HeadFrames * 10.0
	if prepared.PitchCurve == nil || pitchCurveCentsAt(prepared.PitchCurve, padding+45) <= pitchCurveCentsAt(prepared.PitchCurve, padding+165) || pitchCurveCentsAt(prepared.PitchCurve, padding+240) >= 0 {
		t.Fatalf("manual speech curve not reflected at padded mora positions: %#v", prepared.PitchCurve)
	}
	cfg.ManualPitch.Reading = "う"
	if _, _, err := prepareDiffSingerProsody(cfg, "あい", morae, 10); err == nil {
		t.Fatal("accepted mismatched manual reading")
	}
}

func TestDiffSingerSpeechCurveStartsAfterHeadPadding(t *testing.T) {
	morae, _ := frontend.ParseKana("あ")
	curve := &render.PitchCurve{FrameMS: 10, Cents: []float64{0, 100, 200, 300}}
	cfg, _, err := prepareDiffSingerProsody(Config{Reading: "あ", Renderer: "diffsinger", PitchCurve: curve}, "あ", morae, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := pitchCurveCentsAt(cfg.PitchCurve, diffsinger.HeadFrames*10+10); got != 100 {
		t.Fatalf("shifted pitch = %v", got)
	}
	if curve.Cents[1] != 100 {
		t.Fatal("input curve mutated")
	}
}

func TestDiffSingerPhones(t *testing.T) {
	singer := &diffsinger.Singer{Tokens: map[string]int64{"SP": 0, "k": 1, "a": 2, "N": 3}}
	morae := []frontend.Mora{
		{Text: "か", Consonant: "k", Vowel: "a"},
		{Text: "ん", Vowel: "n"},
		{Pause: true},
	}
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100, 90, 180})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"k", "a", "N", "SP"}) {
		t.Fatalf("phones = %#v", phones)
	}
	if !reflect.DeepEqual(durations, []float64{45, 55, 90, 180}) {
		t.Fatalf("durations = %#v", durations)
	}
	if !reflect.DeepEqual(counts, []int64{2, 1, 1}) {
		t.Fatalf("counts = %#v", counts)
	}
}

func TestDiffSingerConsonantDurationStrengthensFricatives(t *testing.T) {
	if got := diffsingerConsonantDuration("ja/h", 100); math.Abs(got-48) > 0.001 {
		t.Fatalf("h duration = %v", got)
	}
	if got := diffsingerConsonantDuration("ja/w", 100); got != 42 {
		t.Fatalf("w duration = %v", got)
	}
}

func TestDiffSingerPhonesUsesSingerDictionary(t *testing.T) {
	singer := &diffsinger.Singer{
		Tokens:             map[string]int64{"SP": 0, "kx": 1, "oo": 2},
		JapaneseDictionary: map[string][]string{"こ": {"kx", "oo"}},
	}
	morae := []frontend.Mora{{Text: "こ", Consonant: "k", Vowel: "o"}}
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"kx", "oo"}) || !reflect.DeepEqual(durations, []float64{45, 55}) || !reflect.DeepEqual(counts, []int64{2}) {
		t.Fatalf("phones = %#v, durations = %#v, counts = %#v", phones, durations, counts)
	}
}

func TestDiffSingerDictionaryUsesVowelForLongMark(t *testing.T) {
	singer := &diffsinger.Singer{JapaneseDictionary: map[string][]string{"お": {"oo"}}}
	got := diffsingerDictionarySymbols(singer, frontend.Mora{Text: "ー", Vowel: "o"})
	if !reflect.DeepEqual(got, []string{"oo"}) {
		t.Fatalf("symbols = %#v", got)
	}
}

func TestDurationsMSToFramesKeepsAccumulatedLength(t *testing.T) {
	got := durationsMSToFrames([]float64{80, 35, 65, 80}, 10)
	if !reflect.DeepEqual(got, []int64{8, 4, 6, 8}) {
		t.Fatalf("frames = %#v", got)
	}
}

func TestGroupedFrameDurations(t *testing.T) {
	got := groupedFrameDurations([]int64{8, 3, 7, 5, 8}, []int64{1, 2, 1, 1})
	if !reflect.DeepEqual(got, []int64{8, 10, 5, 8}) {
		t.Fatalf("durations = %#v", got)
	}
}
