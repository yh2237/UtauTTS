package utautts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yh2237/audiodsp/wav"

	"github.com/yh2237/UtauTTS"
)

func testConfig(t *testing.T) utautts.Config {
	t.Helper()
	root := t.TempDir()
	bank, renderer := filepath.Join(root, "bank"), filepath.Join(root, "renderer")
	for _, dir := range []string{bank, renderer} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	samples := make([]int16, 8000)
	for i := range samples {
		samples[i] = int16(2000 * math.Sin(2*math.Pi*220*float64(i)/16000))
	}
	var audio bytes.Buffer
	if err := wav.Encode(&audio, &wav.PCM{SampleRate: 16000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string][]byte{
		filepath.Join(bank, "a.wav"):   audio.Bytes(),
		filepath.Join(bank, "oto.ini"): []byte("a.wav=あ,0,40,0,60,30\n"),
		filepath.Join(renderer, "renderer.json"): []byte(`{
			"manifest_version":2,"kind":"synthesis-engine","id":"test-waveform",
			"display_name":"Test waveform","contract":"unit-renderer",
			"provider":"waveform","provider_version":"1"
		}`),
	} {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return utautts.Config{VoiceDirectory: bank, Renderer: "test-waveform", RendererDirectories: []string{renderer}}
}

func newEngine(t *testing.T, config utautts.Config) *utautts.Engine {
	t.Helper()
	engine, err := utautts.New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	return engine
}

func TestEngineSynthesisAndIsolation(t *testing.T) {
	config := testConfig(t)
	first, second := newEngine(t, config), newEngine(t, config)
	ctx := context.Background()
	voices, err := first.Voicebanks(ctx)
	if err != nil || len(voices) != 1 {
		t.Fatalf("voicebanks = %v, %v", voices, err)
	}
	request := utautts.Request{Reading: "あ、あ", VoicebankID: voices[0].ID, ModelID: "none", Options: utautts.Options{ApplyPitch: new(false)}}
	analysis, err := first.Analyze(ctx, request)
	if err != nil || analysis.Reading != request.Reading || len(analysis.Morae) != 3 || !analysis.Morae[1].Pause {
		t.Fatalf("analysis = %v, %v", analysis, err)
	}
	baseline, err := first.Synthesize(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Options.PauseDurationMS, request.Options.ReleaseMS = new(0.0), new(0.0)
	short, err := second.Synthesize(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if short.DurationMS >= baseline.DurationMS-100 || short.RendererID != "test-waveform" || short.Reading != request.Reading {
		t.Fatalf("explicit zero pause/release not applied: %v vs %v", short.DurationMS, baseline.DurationMS)
	}
	if len(short.Phonemes) == 0 || short.Phonemes[0].StartMS != 0 || math.Abs(short.Phonemes[len(short.Phonemes)-1].EndMS-short.DurationMS) > 0.001 {
		t.Fatalf("phonemes do not span audio: %v", short.Phonemes)
	}
	var output bytes.Buffer
	if err := short.Audio.WriteWAV(&output); err != nil || output.Len() < 44 {
		t.Fatalf("WAV export: %v", err)
	}
	if err := first.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Synthesize(ctx, request); !errors.Is(err, utautts.ErrClosed) {
		t.Fatalf("synthesis after close: %v", err)
	}
	warm, err := second.Synthesize(ctx, request)
	if err != nil || !slices.Equal(short.Audio.Samples, warm.Audio.Samples) {
		t.Fatalf("other engine changed after reload/close: %v", err)
	}
}

func TestEngineRequestErrors(t *testing.T) {
	engine := newEngine(t, testConfig(t))
	for _, request := range []utautts.Request{
		{},
		{Reading: "あ", Options: utautts.Options{MoraDurationMS: new(0.0)}},
		{Reading: "あ", Options: utautts.Options{IntonationStrength: new(math.NaN())}},
		{Reading: "あ", RendererSettings: map[string]json.RawMessage{"timing_warp": json.RawMessage(`"false"`)}},
		{Reading: "あ", RendererSettings: map[string]json.RawMessage{"pause_duration_ms": json.RawMessage(`0`)}},
	} {
		if _, err := engine.Synthesize(context.Background(), request); !errors.Is(err, utautts.ErrInvalidRequest) {
			t.Errorf("invalid request returned %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Analyze(ctx, utautts.Request{Reading: "あ"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled analysis: %v", err)
	}
	if _, err := engine.Synthesize(context.Background(), utautts.Request{Reading: "あ", VoicebankID: "missing"}); !errors.Is(err, utautts.ErrUnavailable) {
		t.Fatalf("missing voicebank: %v", err)
	}
}

// 同梱音源とWORLDを配置し、UTAUTTS_INTEGRATION=1で実行する。
func TestBundledWorldRuntimeIsolation(t *testing.T) {
	if os.Getenv("UTAUTTS_INTEGRATION") != "1" {
		t.Skip("set UTAUTTS_INTEGRATION=1 to test bundled WORLD synthesis")
	}
	voice := os.Getenv("UTAUTTS_TEST_VOICE")
	if voice == "" {
		t.Skip("set UTAUTTS_TEST_VOICE to the voicebank folder name under voice/")
	}
	config := utautts.Config{VoiceDirectory: filepath.Join("voice", voice), Renderer: "utautts-world-phrase"}
	first, second := newEngine(t, config), newEngine(t, config)
	ctx := context.Background()
	request := utautts.Request{Text: "こんにちは。今日はいい天気ですね。", Tone: "C4"}
	for _, engine := range []*utautts.Engine{first, second} {
		if _, err := engine.Synthesize(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Synthesize(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Synthesize(ctx, request); err != nil {
		t.Fatalf("closing first engine stopped second: %v", err)
	}
}
