// source-span-listenは原音区間指定の比較音声を作る。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/render/base"
	"utautts/internal/synth"
	"utautts/internal/tts"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	planPath := flag.String("plan", "", "reference plan")
	spanPaths := flag.String("spans", "", "comma-separated span reports; later reports override earlier ones")
	out := flag.String("out", "", "new output directory")
	bridge := flag.String("bridge", "out/utautts-worldline-bridge-current.exe", "WORLD bridge")
	automatic := flag.Bool("automatic", false, "compare normal source-library integration")
	flag.Parse()
	if *planPath == "" || (*spanPaths == "" && !*automatic) || *out == "" {
		return fmt.Errorf("--plan, --spans, --out required")
	}
	var p plan.Plan
	if err := readJSON(*planPath, &p); err != nil {
		return err
	}
	spans := map[int]base.ExperimentalSourceSpan{}
	for _, path := range strings.Split(*spanPaths, ",") {
		if path == "" {
			continue
		}
		var report struct {
			Units []struct {
				base.ExperimentalSourceSpan
				Index int `json:"unit_index"`
			}
		}
		if err := readJSON(path, &report); err != nil {
			return err
		}
		for _, row := range report.Units {
			if row.Index < 0 || row.Index >= len(p.Units) {
				return fmt.Errorf("invalid unit index")
			}
			spans[row.Index] = row.ExperimentalSourceSpan
		}
	}
	if len(spans) == 0 && !*automatic {
		return fmt.Errorf("no selected spans")
	}
	if _, err := os.Stat(*out); !os.IsNotExist(err) {
		return fmt.Errorf("output directory already exists or inaccessible")
	}
	runtime, err := synth.NewRuntime(synth.RuntimeConfig{Renderer: "utautts-world-phrase", WorldlineBridgePath: *bridge}, nil)
	if err != nil {
		return err
	}
	model := "frame-intonation-tcn-en-v1"
	if p.Language == "zh" {
		model = "none"
	}
	r, err := runtime.Service.ResolveSynthesis(synth.Request{VoicebankPath: p.Voicebank, Text: p.Text, Reading: p.Reading, Language: p.Language, Phonemizer: p.Phonemizer, Tone: "C4", MoraDurationMS: 120, PauseDurationMS: 180, ModelID: model, ProsodyPitchOnly: true, ApplyPitch: true, IntonationStrength: 1, EnglishWeakForm: true})
	if err != nil {
		return err
	}
	disabled := false
	beforeOptions := r.ProviderOptions
	beforeOptions.Worldline.SourcePhoneMapping = &disabled
	before, err := tts.SynthesizeWithOptions(r.Config, beforeOptions)
	if err != nil {
		return err
	}
	for i, s := range spans {
		if i >= len(before.Plan.Units) || before.Plan.Units[i].Alias != s.Alias || before.Plan.Units[i].Source != p.Units[i].Source {
			return fmt.Errorf("reference plan changed at unit %d", i)
		}
	}
	options := r.ProviderOptions
	if !*automatic {
		options.Worldline.SourcePhoneMapping = &disabled
	}
	options.Worldline.ExperimentalSourceSpans = spans
	after, err := tts.SynthesizeWithOptions(r.Config, options)
	if err != nil {
		return err
	}
	if before.Audio.SampleRate != after.Audio.SampleRate || before.Audio.Channels != after.Audio.Channels || len(before.Audio.Data) != len(after.Audio.Data) {
		return fmt.Errorf("comparison audio timing changed")
	}
	if !reflect.DeepEqual(before.Plan.PhoneTimings, after.Plan.PhoneTimings) {
		return fmt.Errorf("comparison phone timing changed")
	}
	if len(before.PitchPoints) != len(after.PitchPoints) {
		return fmt.Errorf("comparison pitch length changed")
	}
	maxPitchDifference := 0.0
	for i, v := range before.PitchPoints {
		d := math.Abs(v - after.PitchPoints[i])
		if math.IsNaN(d) || math.IsInf(d, 0) || d > .01 {
			return fmt.Errorf("comparison pitch changed: %g cents", d)
		}
		maxPitchDifference = math.Max(maxPitchDifference, d)
	}
	fmt.Printf("maximum pitch difference: %.6f cents\n", maxPitchDifference)
	mappedCount := len(spans)
	if *automatic {
		mappedCount = 0
		for _, u := range after.RenderedPlan().Units {
			if u.SpeechMapping == "source-phone-library-v1" {
				mappedCount++
			}
		}
		if mappedCount == 0 {
			return fmt.Errorf("automatic mapping did not apply to any unit")
		}
	}
	for i := range spans {
		if !strings.HasPrefix(after.RenderedPlan().Units[i].SpeechMapping, "experimental-aligned-source-span") {
			return fmt.Errorf("span not applied at unit %d", i)
		}
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	for _, v := range []struct {
		Name   string
		Result *tts.Result
	}{{"before", before}, {"after", after}} {
		if err = audio.WriteWav(filepath.Join(*out, v.Name+".wav"), v.Result.Audio); err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(*out, v.Name+".plan.json"), v.Result.RenderedPlan()); err != nil {
			return err
		}
	}
	joined := *before.Audio
	joined.Data = append(append(append([]int16{}, before.Audio.Data...), make([]int16, before.Audio.SampleRate*before.Audio.Channels/2)...), after.Audio.Data...)
	if err = audio.WriteWav(filepath.Join(*out, "comparison.wav"), &joined); err != nil {
		return err
	}
	fmt.Printf("created before/after comparison: %s (%d mapped units)\n", *out, mappedCount)
	return nil
}
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
