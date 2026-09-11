package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
)

func TestPitchTraceUsesAudioMarginAndMeasuresKnownTone(t *testing.T) {
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 3200)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(12000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	hz := make([]float64, 20)
	for i := range hz {
		hz[i] = 200
	}
	result := &synth.Result{Result: &tts.Result{Audio: pcm, RenderReport: &render.RenderReport{TargetF0: &render.F0Track{StartMS: -30, FrameMS: 10, Hz: hz}}}}
	path := filepath.Join(t.TempDir(), "pitch.json")
	if err := writePitchTrace(path, result); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var trace pitchTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	if trace.PairedFrames < 10 || trace.MedianAbsoluteCents > 5 || trace.Frames[3].PlanMS != 0 || trace.Frames[3].AudioMS != 30 {
		t.Fatalf("trace=%+v", trace)
	}
	if trace.Frames[0].MeasuredHz != 0 || trace.Frames[0].ErrorCents != nil {
		t.Fatal("edge window falsely measured")
	}
}

func TestProsodyExperimentRejectsUnsupportedInputs(t *testing.T) {
	good := []prompt{{ID: "x", Language: "en", Phonemizer: "en-delta", Reading: "AH0"}}
	if err := validateProsodyExperiment("both", "utautts-world-phrase", "none", "", false, good); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"invalid", "Both"} {
		if validateProsodyExperiment(mode, "utautts-world-phrase", "none", "", false, good) == nil {
			t.Fatal("invalid mode accepted")
		}
	}
	if validateProsodyExperiment("both", "waveform", "none", "", false, good) == nil {
		t.Fatal("unsupported renderer accepted")
	}
	if validateProsodyExperiment("pitch", "utautts-world-phrase", "none", "", false, []prompt{{ID: "ja", Language: "ja", Text: "あ"}}) == nil {
		t.Fatal("Japanese experiment accepted")
	}
}
