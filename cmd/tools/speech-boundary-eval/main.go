package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

type result struct {
	ID              string                  `json:"id"`
	Noise           float64                 `json:"noise_amplitude"`
	ExpectedStart   float64                 `json:"expected_activity_start_ms"`
	ExpectedEnd     float64                 `json:"expected_activity_end_ms"`
	ExpectedRelease float64                 `json:"expected_release_ms"`
	StartError      float64                 `json:"activity_start_error_ms"`
	EndError        float64                 `json:"activity_end_error_ms"`
	ReleaseError    float64                 `json:"release_error_ms"`
	Profile         voicebank.SpeechProfile `json:"profile"`
}

func main() {
	out := flag.String("out", "", "benchmark output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// 境界既知の統制波形。実音声の音素境界精度とは分けて記録する。
func run(out string) error {
	if out == "" {
		return fmt.Errorf("--out required")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	bank := &voicebank.Bank{}
	random := rand.New(rand.NewSource(928))
	var rows []result
	var priorChecks []map[string]any
	for _, rate := range []int{16000, 44100} {
		for _, noise := range []float64{0, .001, .01} {
			for _, release := range []float64{220, 280, 340} {
				const offset = 37.0
				start, end := 50.0, release+40
				pcm := &audio.PCM{SampleRate: rate, Channels: 1, Data: make([]int16, int(.55*float64(rate)))}
				for i := range pcm.Data {
					ms := float64(i)*1000/float64(rate) - offset
					value := noise * (random.Float64()*2 - 1)
					if ms >= start && ms < end {
						value += .15 * math.Sin(2*math.Pi*180*float64(i)/float64(rate))
					}
					if ms >= release-20 && ms < release {
						value *= .05
					}
					if ms >= release && ms < release+4 {
						value += .55 * math.Exp(-(ms - release)) * math.Sin(2*math.Pi*3100*(ms-release)/1000)
					}
					pcm.Data[i] = int16(math.Max(-32767, math.Min(32767, value*32767)))
				}
				id := fmt.Sprintf("%d-noise-%.3f-release-%.0f", rate, noise, release)
				path := filepath.Join(out, id+".wav")
				if err := audio.WriteWav(path, pcm); err != nil {
					return err
				}
				entry := oto.Entry{Filename: path, Offset: offset, Blank: -450, Preutterance: 60, Fixed: release}
				profile := bank.CalibrateSpeech(entry)
				for _, shift := range []float64{-15, 20, 50} {
					probe := entry
					probe.Fixed += shift
					estimated := bank.CalibrateSpeech(probe)
					priorChecks = append(priorChecks, map[string]any{"id": id, "prior_shift_ms": shift, "release_error_ms": math.Abs(estimated.ReleaseTransientMS - release), "confidence": estimated.ReleaseTransientConfidence, "within_5_ms": estimated.ReleaseTransientConfidence >= .25 && math.Abs(estimated.ReleaseTransientMS-release) <= 5})
				}
				rows = append(rows, result{id, noise, start, end, release, math.Abs(profile.ActivityStartMS - start), math.Abs(profile.ActivityEndMS - end), math.Abs(profile.ReleaseTransientMS - release), profile})
			}
		}
	}
	var metrics []map[string]any
	for _, noise := range []float64{0, .001, .01} {
		n, start, end, burst, detected := 0, 0.0, 0.0, 0.0, 0
		for _, r := range rows {
			if r.Noise != noise {
				continue
			}
			n++
			start += r.StartError
			end += r.EndError
			if r.Profile.ReleaseTransientConfidence >= .25 {
				detected++
				burst += r.ReleaseError
			}
		}
		var releaseMAE any
		if detected > 0 {
			releaseMAE = burst / float64(detected)
		}
		metrics = append(metrics, map[string]any{"noise_amplitude": noise, "cases": n, "activity_start_mae_ms": start / float64(n), "activity_end_mae_ms": end / float64(n), "release_detected": detected, "release_mae_ms": releaseMAE})
	}
	data, err := json.MarshalIndent(map[string]any{"data_kind": "controlled-synthetic", "not_natural_phoneme_accuracy": true, "metrics": metrics, "cases": rows, "prior_sensitivity": priorChecks}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "accuracy.json"), data, 0644)
}
