// tts-evalは日本語の聴取用音声と再現可能な計測結果を作る。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"utautts/internal/atomicfile"
	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
)

type prompt struct{ ID, Text, Focus string }
type measurement struct {
	ID          string  `json:"id"`
	Text        string  `json:"text"`
	Focus       string  `json:"focus"`
	Renderer    string  `json:"renderer"`
	Repetition  int     `json:"repetition"`
	ElapsedMS   float64 `json:"elapsed_ms"`
	AudioMS     float64 `json:"audio_ms"`
	RTF         float64 `json:"rtf"`
	Peak        float64 `json:"peak"`
	RMS         float64 `json:"rms"`
	SilentUnits int     `json:"silent_units"`
	Error       string  `json:"error,omitempty"`
	WAV         string  `json:"wav,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	bank := flag.String("voicebank", "", "voicebank directory (required)")
	corpus := flag.String("corpus", "tools/evaluation/japanese-v1.json", "JSON listening corpus")
	out := flag.String("out", "out/tts-eval", "new output directory")
	renderers := flag.String("renderers", "utautts-world-phrase,utautts-world-phrase-cuda", "comma-separated renderer IDs")
	model := flag.String("model", "frame-intonation-v8", "prosody model ID")
	bridge := flag.String("bridge", "", "override WORLD bridge executable")
	gpu := flag.String("gpu", "", "override CUDA DLL")
	repeats := flag.Int("repeat", 2, "repetitions in the same process; first and warm runs are separate")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout per synthesis")
	flag.Parse()
	if *bank == "" || *repeats < 1 || *timeout <= 0 {
		return fmt.Errorf("voicebank, positive repeat and timeout are required")
	}
	data, err := os.ReadFile(*corpus)
	if err != nil {
		return err
	}
	var prompts []prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return err
	}
	if len(prompts) == 0 {
		return fmt.Errorf("empty corpus")
	}
	catalog, err := plugin.DiscoverWithDefaults(nil, nil, render.IsKnownRenderer)
	if err != nil {
		return err
	}
	prosody, ok := catalog.Model(*model)
	if !ok {
		return fmt.Errorf("unknown model %q", *model)
	}
	// 既存の基準音声は上書きしない。
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(*out, 0755); err != nil {
		return err
	}
	profilePath, err := filepath.Abs(filepath.Join(*out, "world-profile.jsonl"))
	if err != nil {
		return err
	}
	if err := os.Setenv("UTAUTTS_WORLD_PROFILE", profilePath); err != nil {
		return err
	}
	var rows []measurement
	failed := false
	buildInfo, _ := debug.ReadBuildInfo()
	for rendererIndex, rendererID := range strings.Split(*renderers, ",") {
		rendererID = strings.TrimSpace(rendererID)
		for index, p := range prompts {
			for repetition := 1; repetition <= *repeats; repetition++ {
				row := measurement{ID: p.ID, Text: p.Text, Focus: p.Focus, Renderer: rendererID, Repetition: repetition}
				cfg := tts.Config{VoicebankPath: *bank, Text: p.Text, Language: "ja", Tone: "C4", MoraDurationMS: 120, PauseDurationMS: 180, ApplyPitch: true, IntonationStrength: 1, ProsodyModelPath: prosody.Path}
				resolved, callErr := tts.ApplyRenderer(&cfg, catalog, rendererID, "", *bridge)
				if *gpu != "" {
					cfg.WorldGPUPath = *gpu
				}
				ctx, cancel := context.WithTimeout(context.Background(), *timeout)
				cfg.Context = ctx
				started := time.Now()
				var result *synth.Result
				if callErr == nil {
					result, callErr = synth.SynthesizeConfig(cfg, resolved)
				}
				row.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
				cancel()
				if callErr == nil {
					row.AudioMS = result.DurationMS
					if row.AudioMS > 0 {
						row.RTF = row.ElapsedMS / row.AudioMS
					}
					for _, s := range result.Audio.Data {
						x := float64(s) / 32768
						row.Peak = math.Max(row.Peak, math.Abs(x))
						row.RMS += x * x
					}
					if len(result.Audio.Data) > 0 {
						row.RMS = math.Sqrt(row.RMS / float64(len(result.Audio.Data)))
					}
					for _, u := range result.Plan.Units {
						if u.Silent {
							row.SilentUnits++
						}
					}
					row.WAV = fmt.Sprintf("%02d-renderer%02d-%d.wav", index+1, rendererIndex+1, repetition)
					callErr = synth.WriteFiles(filepath.Join(*out, row.WAV), result, synth.ExportOptions{Text: p.Text, WriteText: true, WriteLab: true})
				}
				if callErr != nil {
					row.Error = callErr.Error()
					failed = true
				}
				rows = append(rows, row)
				fmt.Printf("%s %s #%d: %.0f ms, RTF %.3f %s\n", rendererID, p.ID, repetition, row.ElapsedMS, row.RTF, row.Error)
				// 後続ケースが失敗しても途中結果を保存する。
				report := struct {
					GOOS, GOARCH, Voicebank, Model string
					CorpusSHA256, Bridge, GPU      string
					Build                          *debug.BuildInfo
					Measurements                   []measurement
				}{runtime.GOOS, runtime.GOARCH, *bank, *model, fmt.Sprintf("%x", sha256.Sum256(data)), *bridge, *gpu, buildInfo, rows}
				encoded, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				if err := atomicfile.WriteFile(filepath.Join(*out, "report.json"), encoded); err != nil {
					return err
				}
			}
		}
	}
	if failed {
		return fmt.Errorf("some synthesis cases failed; see report.json")
	}
	return nil
}
