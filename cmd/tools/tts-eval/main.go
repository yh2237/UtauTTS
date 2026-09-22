// tts-evalは聴取用音声と計測結果を作る。
package main

import (
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
)

type prompt struct {
	ID, Text, Focus, Reading, Language, Phonemizer string
	MoraDurationsMS                                []float64          `json:"mora_durations_ms,omitempty"`
	PitchCurve                                     *render.PitchCurve `json:"pitch_curve,omitempty"`
}
type measurement struct {
	ID                       string  `json:"id"`
	Text                     string  `json:"text"`
	Focus                    string  `json:"focus"`
	Renderer                 string  `json:"renderer"`
	Repetition               int     `json:"repetition"`
	ElapsedMS                float64 `json:"elapsed_ms"`
	AudioMS                  float64 `json:"audio_ms"`
	RTF                      float64 `json:"rtf"`
	Peak                     float64 `json:"peak"`
	RMS                      float64 `json:"rms"`
	SilentUnits              int     `json:"silent_units"`
	MissingPhoneGroups       int     `json:"missing_phone_groups"`
	V13CompatibleUnits       int     `json:"v1_3_compatible_units"`
	AdaptiveUnits            int     `json:"adaptive_units"`
	GapRepairUnits           int     `json:"gap_repair_units"`
	StopBurstUnits           int     `json:"stop_burst_units"`
	UnreliableTransientUnits int     `json:"unreliable_transient_units"`
	MeanStopBurstGain        float64 `json:"mean_stop_burst_gain,omitempty"`
	Error                    string  `json:"error,omitempty"`
	WAV                      string  `json:"wav,omitempty"`
}

// evalReportは単一モードと掃引モードで共有するreport.jsonのスキーマ。
type evalReport struct {
	WordBoundaryEnvelope           bool
	MoraMS                         float64
	ProsodyExperiment, Phonemizer  string
	MeasurePitch, SpeechTiming     bool
	WorldMix, WorldGapRepair       string
	GOOS, GOARCH, Voicebank, Model string
	CorpusSHA256, Bridge           string
	Build                          *debug.BuildInfo
	Measurements                   []measurement
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	wordEnvelope := flag.Bool("word-boundary-envelope", false, "halve fades at word boundaries without changing source or pitch (CPU WORLD)")
	exportSources := flag.Bool("export-sources", false, "export original, selected and mixed-output source audit clips")
	moraMS := flag.Float64("mora-ms", 120, "base syllable duration in milliseconds")
	experiment := flag.String("prosody-experiment", "baseline", "speech prosody comparison: baseline, timing, pitch, both (CPU WORLD only)")
	measurePitch := flag.Bool("measure-pitch", false, "write WORLD target and measured output F0 traces")
	phonemizer := flag.String("phonemizer", "", "override corpus phonemizer for the selected voicebank")
	speechTiming := flag.Bool("speech-timing", false, "experimental speech timing and voicebank calibration")
	aliasPolicy := flag.String("alias-policy", "auto", "voicebank mode: auto or cv-only")
	bank := flag.String("voicebank", "", "voicebank directory (required)")
	diagnose := flag.Bool("diagnose", false, "write frontend and candidate diagnostics without rendering")
	corpus := flag.String("corpus", "tools/evaluation/japanese-v1.json", "JSON listening corpus")
	out := flag.String("out", "out/tts-eval", "new output directory")
	renderers := flag.String("renderers", "utautts-world-phrase", "comma-separated renderer IDs")
	model := flag.String("model", "frame-intonation-v9-t", "prosody model ID")
	modelFile := flag.String("model-file", "", "explicit experimental prosody model JSON (overrides model ID)")
	bridge := flag.String("bridge", "", "override WORLD bridge executable")
	worldMix := flag.String("world-mix", "auto", "WORLD feature mixing: auto, v1.3, adaptive")
	worldGapRepair := flag.String("world-gap-repair", "auto", "WORLD gap repair: auto, on, off")
	repeats := flag.Int("repeat", 2, "repetitions in the same process; first and warm runs are separate (ignored with --sweep)")
	sweep := flag.Bool("sweep", false, "sweep correction presets across the corpus; each case runs exactly once and --repeat is ignored")
	presets := flag.String("presets", defaultPresets, "comma-separated sweep preset names (only with --sweep)")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout per synthesis")
	flag.Parse()
	if *sweep && *diagnose {
		return fmt.Errorf("sweep and diagnose cannot be combined")
	}
	if *wordEnvelope && *diagnose {
		return fmt.Errorf("word-boundary-envelope requires synthesis")
	}
	if *moraMS <= 0 || math.IsNaN(*moraMS) || math.IsInf(*moraMS, 0) {
		return fmt.Errorf("mora-ms must be positive and finite")
	}
	if *bank == "" || *timeout <= 0 || (!*sweep && *repeats < 1) {
		return fmt.Errorf("voicebank, positive repeat and timeout are required")
	}
	if !oneOf(*worldMix, "auto", "v1.3", "adaptive") {
		return fmt.Errorf("world-mix must be auto, v1.3 or adaptive")
	}
	if !oneOf(*worldGapRepair, "auto", "on", "off") {
		return fmt.Errorf("world-gap-repair must be auto, on or off")
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
	if *phonemizer != "" {
		for i := range prompts {
			prompts[i].Phonemizer = *phonemizer
		}
	}
	if err := validatePrompts(prompts); err != nil {
		return err
	}
	if err := validateProsodyExperiment(*experiment, *renderers, *model, *modelFile, *diagnose, prompts); err != nil {
		return err
	}
	if *sweep {
		return runSweep(sweepRequest{
			bank: *bank, out: *out, presets: *presets, aliasPolicy: *aliasPolicy, bridge: *bridge,
			model: *model, modelFile: *modelFile, experiment: *experiment, phonemizer: *phonemizer,
			corpusData: data, prompts: prompts, moraMS: *moraMS, wordEnvelope: *wordEnvelope, timeout: *timeout,
		})
	}
	if *measurePitch && (*diagnose || *renderers != "utautts-world-phrase") {
		return fmt.Errorf("pitch measurement requires CPU WORLD synthesis")
	}
	if *diagnose {
		if *exportSources {
			return fmt.Errorf("export-sources requires synthesis")
		}
		return diagnoseCorpus(*bank, *out, prompts)
	}
	catalog, err := plugin.DiscoverWithDefaults(nil, nil, render.IsKnownRenderer)
	if err != nil {
		return err
	}
	prosody, ok := catalog.Model(*model)
	modelIdentity := *model
	if *modelFile != "" {
		modelIdentity = *modelFile
	}
	if !ok && *model != "none" && *modelFile == "" {
		return fmt.Errorf("unknown model %q", *model)
	}
	prosodyPath := ""
	if *model != "none" {
		prosodyPath = prosody.Path
	}
	if *modelFile != "" {
		prosodyPath = *modelFile
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
				result, elapsed, callErr := synthesizeCase(p, caseOptions{
					bank: *bank, aliasPolicy: *aliasPolicy, bridge: *bridge,
					model: *model, modelFile: *modelFile, prosodyModelPath: prosodyPath,
					moraMS: *moraMS, experiment: *experiment, wordEnvelope: *wordEnvelope,
					rendererID: rendererID, mix: *worldMix, gapRepair: *worldGapRepair,
					speechTiming: *speechTiming, applyPitch: true, timeout: *timeout,
				}, catalog)
				row.ElapsedMS = elapsed
				if callErr == nil {
					renderedPlan := fillMeasurement(&row, result)
					row.WAV = fmt.Sprintf("%02d-renderer%02d-%d.wav", index+1, rendererIndex+1, repetition)
					callErr = synth.WriteFiles(filepath.Join(*out, row.WAV), result, synth.ExportOptions{Text: p.Text, WriteText: true, WriteLab: true})
					if callErr == nil {
						var planData []byte
						planData, callErr = json.MarshalIndent(renderedPlan, "", "  ")
						if callErr == nil {
							callErr = atomicfile.WriteFile(filepath.Join(*out, strings.TrimSuffix(row.WAV, ".wav")+".plan.json"), planData)
						}
					}
					if callErr == nil && *measurePitch {
						callErr = writePitchTrace(filepath.Join(*out, strings.TrimSuffix(row.WAV, ".wav")+".pitch.json"), result)
					}
					if callErr == nil && *exportSources {
						callErr = writeSourceAudit(filepath.Join(*out, strings.TrimSuffix(row.WAV, ".wav")+"-sources"), result)
					}
				}
				if callErr != nil {
					row.Error = callErr.Error()
					failed = true
				}
				rows = append(rows, row)
				fmt.Printf("%s %s #%d: %.0f ms, RTF %.3f %s\n", rendererID, p.ID, repetition, row.ElapsedMS, row.RTF, row.Error)
				// 後続ケースが失敗しても途中結果を保存する。
				report := evalReport{*wordEnvelope, *moraMS, *experiment, *phonemizer, *measurePitch, *speechTiming, *worldMix, *worldGapRepair, runtime.GOOS, runtime.GOARCH, *bank, modelIdentity, fmt.Sprintf("%x", sha256.Sum256(data)), *bridge, buildInfo, rows}
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

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
