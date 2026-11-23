package refspeech

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/encoder"
)

func TestOpenReportsBrokenModel(t *testing.T) {
	dir := t.TempDir()
	weights := filepath.Join(dir, "kokoro-v1_0.pth")
	if err := os.WriteFile(weights, []byte("not a torch file"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := Open(Models{
		KokoroWeights: weights,
		KokoroConfig:  filepath.Join(dir, "missing.json"),
		KokoroVoice:   filepath.Join(dir, "missing.pt"),
		WavLM:         filepath.Join(dir, "missing.safetensors"),
	})
	if err == nil {
		t.Fatal("Open accepted a broken model")
	}
	if engine != nil {
		t.Fatalf("Open returned an engine with the error: %v", err)
	}
}

type voicedAudioWorld struct{ audioTestWorld }

func (w *voicedAudioWorld) Analyze(samples []float64, rate int, frameMS, threshold float64) ([]float64, []float64, []float64, int, error) {
	f0, spectrum, ap, fftSize, err := w.audioTestWorld.Analyze(samples, rate, frameMS, threshold)
	for i := range f0 {
		f0[i] = 200
	}
	return f0, spectrum, ap, fftSize, err
}

func TestSpeakAudioClampsExplicitPitchShift(t *testing.T) {
	engine := externalTestEngine(t)
	const dims = 768
	id := encoder.IDWavLMBasePlusL6
	idx := testIndex(id, 2, dims)
	idx.F0Median = 220
	idx.Channels[id][0], idx.Channels[id][dims] = 1, 1
	world := &voicedAudioWorld{}
	audio := make([]float64, 1600)
	semitones := 24.0
	opts := AudioOptions{PitchSemitones: &semitones}
	_, err := engine.SpeakAudio(idx, world, audio, 16000, opts)
	var need *NeedInference
	if !errors.As(err, &need) || len(need.Jobs) != 1 {
		t.Fatalf("expected an inference job, got %v", err)
	}
	frames := encoder.ConvFrameCount(len(audio))
	values := make([]float32, frames*dims)
	for i := range frames {
		values[i*dims] = 1
	}
	job := need.Jobs[0]
	if err := engine.ProvideFeatures(job.EncoderID, job.Key, values, frames, dims); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		options AudioOptions
		want    float64
	}{
		{opts, 400},
		{AudioOptions{}, 220},
	} {
		speech, err := engine.SpeakAudio(idx, world, audio, 16000, tc.options)
		if err != nil {
			t.Fatal(err)
		}
		if len(speech.F0) == 0 || math.Abs(speech.F0[0]-tc.want) > 1e-6 || len(speech.Audio) == 0 {
			t.Fatalf("F0=%v, samples=%d, want %.0f Hz and audio", speech.F0, len(speech.Audio), tc.want)
		}
	}
}
