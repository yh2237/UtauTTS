package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/processutil"
	"utautts/internal/render"
)

type result struct {
	Renderer    string  `json:"renderer"`
	Path        string  `json:"path"`
	Status      string  `json:"status"`
	ExitCode    int     `json:"exit_code,omitempty"`
	ElapsedMS   int64   `json:"elapsed_ms"`
	OutputPath  string  `json:"output_path,omitempty"`
	OutputBytes int64   `json:"output_bytes,omitempty"`
	SampleRate  int     `json:"sample_rate,omitempty"`
	DurationMS  float64 `json:"duration_ms,omitempty"`
	Peak        float64 `json:"peak,omitempty"`
	RMS         float64 `json:"rms,omitempty"`
	Detail      string  `json:"detail,omitempty"`
}

func main() {
	input := flag.String("input", "", "source WAV passed to every resampler")
	outputDirectory := flag.String("out-dir", "", "directory for rendered WAV files")
	timeout := flag.Duration("timeout", time.Minute, "timeout for each resampler")
	mode := flag.String("mode", "direct", "probe mode: direct or integration")
	wavtool := flag.String("wavtool", "", "external wavtool used in integration mode")
	velocity := flag.Int("velocity", 100, "classic resampler velocity")
	resamplerFlags := flag.String("flags", "", "classic resampler flags")
	modulation := flag.Int("modulation", 0, "classic resampler modulation")
	tempo := flag.Float64("tempo", 120, "classic resampler tempo")
	flag.Parse()
	if *input == "" || *outputDirectory == "" || flag.NArg() == 0 || (*mode != "direct" && *mode != "integration") {
		fmt.Fprintln(os.Stderr, "usage: resampler-compat --input source.wav --out-dir output renderer.exe...")
		os.Exit(2)
	}
	inputPath, err := filepath.Abs(*input)
	if err != nil {
		fatal(err)
	}
	outputRoot, err := filepath.Abs(*outputDirectory)
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		fatal(err)
	}
	wavtoolPath := ""
	if *wavtool != "" {
		wavtoolPath, err = filepath.Abs(*wavtool)
		if err != nil {
			fatal(err)
		}
	}

	results := make([]result, 0, flag.NArg())
	for index, value := range flag.Args() {
		executable, pathErr := filepath.Abs(value)
		if pathErr != nil {
			results = append(results, result{Renderer: filepath.Base(value), Path: value, Status: "invalid-path", Detail: pathErr.Error()})
			continue
		}
		name := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
		output := filepath.Join(outputRoot, fmt.Sprintf("%02d-%s.wav", index+1, name))
		if *mode == "integration" {
			results = append(results, probeIntegration(inputPath, output, executable, wavtoolPath, *timeout, *velocity, *resamplerFlags, *modulation, *tempo))
		} else {
			results = append(results, probe(inputPath, output, executable, *timeout, *velocity, *resamplerFlags, *modulation, *tempo))
		}
	}
	encoded, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(encoded))
}

func probeIntegration(input, output, executable, wavtool string, timeout time.Duration, velocity int, flags string, modulation int, tempo float64) result {
	r := result{Renderer: filepath.Base(executable), Path: executable, Status: "failed", OutputPath: output}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	synthesisPlan := &plan.Plan{Reading: "compatibility probe", DurationMS: 300, Units: []plan.Unit{{
		Position: 0, Role: "mora", Mora: "probe", Alias: "probe", Source: input,
		NoteStartMS: 0, DurationMS: 300, ConsonantMS: 100, PreutteranceMS: 100,
		OverlapMS: 20, PitchFactor: 1, EnergyFactor: 1,
	}}}
	started := time.Now()
	pcm, err := render.Render(synthesisPlan, render.Config{
		Context: ctx, Backend: "utau-external-resampler",
		ProviderOptions: render.ProviderOptions{Classic: render.ClassicOptions{
			ResamplerPath: executable, WavtoolPath: wavtool,
			Velocity: velocity, VelocitySet: true, Flags: flags,
			Modulation: modulation, ModulationSet: true, Tempo: tempo,
		}},
		ReleaseSet: true, CVVCTiming: render.CVVCTimingLegacy,
	})
	r.ElapsedMS = time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		r.Status, r.Detail = "timeout", ctx.Err().Error()
		return r
	}
	if err != nil {
		r.Detail = err.Error()
		return r
	}
	if err := audio.WriteWav(output, pcm); err != nil {
		r.Detail = "write integrated WAV: " + err.Error()
		return r
	}
	return inspectOutput(r, output)
}

func probe(input, output, executable string, timeout time.Duration, velocity int, flags string, modulation int, tempo float64) result {
	r := result{Renderer: filepath.Base(executable), Path: executable, Status: "failed", OutputPath: output}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	arguments := []string{input, output, "C4", fmt.Sprint(velocity), flags, "0", "300", "100", "0", "100", fmt.Sprint(modulation), "!" + fmt.Sprint(tempo), "AA#58#"}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = filepath.Dir(executable)
	processutil.Configure(command)
	started := time.Now()
	message, err := command.CombinedOutput()
	r.ElapsedMS = time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		r.Status, r.Detail = "timeout", ctx.Err().Error()
		return r
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			r.ExitCode = exitErr.ExitCode()
		}
		r.Detail = strings.TrimSpace(string(message))
		if r.Detail == "" {
			r.Detail = err.Error()
		}
		return r
	}
	return inspectOutput(r, output)
}

func inspectOutput(r result, output string) result {
	info, err := os.Stat(output)
	if err != nil {
		r.Detail = "process succeeded without an output WAV: " + err.Error()
		return r
	}
	r.OutputBytes = info.Size()
	pcm, err := audio.ReadWav(output)
	if err != nil {
		r.Detail = "invalid output WAV: " + err.Error()
		return r
	}
	r.SampleRate = pcm.SampleRate
	if pcm.SampleRate > 0 && pcm.Channels > 0 {
		r.DurationMS = float64(len(pcm.Data)) / float64(pcm.SampleRate*pcm.Channels) * 1000
	}
	var sum float64
	for _, sample := range pcm.Data {
		value := math.Abs(float64(sample) / 32768)
		r.Peak = math.Max(r.Peak, value)
		sum += value * value
	}
	if len(pcm.Data) > 0 {
		r.RMS = math.Sqrt(sum / float64(len(pcm.Data)))
	}
	if r.Peak == 0 {
		r.Detail = "output WAV is silent"
		return r
	}
	r.Status = "ok"
	return r
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
