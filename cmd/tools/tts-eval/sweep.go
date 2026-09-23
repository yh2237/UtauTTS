package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// defaultPresetsは--presetsの既定値。
const defaultPresets = "default,legacy,legacy-gap,adaptive,timing,no-pitch,waveform"

// baseRenderer, baseMix, baseGapはプリセット表の基準値。
const (
	baseRenderer = "utautts-world-phrase"
	baseMix      = "auto"
	baseGap      = "auto"
)

type preset struct {
	Name         string
	Renderer     string
	Mix          string
	GapRepair    string
	SpeechTiming bool
	ApplyPitch   bool
}

// presetTableは補正プリセットの定義。各フィールドは基準値か上書き値を持つ。
var presetTable = []preset{
	{Name: "default", Renderer: baseRenderer, Mix: baseMix, GapRepair: baseGap, ApplyPitch: true},
	{Name: "legacy", Renderer: baseRenderer, Mix: "v1.3", GapRepair: "off", ApplyPitch: true},
	{Name: "legacy-gap", Renderer: baseRenderer, Mix: "v1.3", GapRepair: "auto", ApplyPitch: true},
	{Name: "adaptive", Renderer: baseRenderer, Mix: "adaptive", GapRepair: "off", ApplyPitch: true},
	{Name: "timing", Renderer: baseRenderer, Mix: baseMix, GapRepair: baseGap, SpeechTiming: true, ApplyPitch: true},
	{Name: "no-pitch", Renderer: baseRenderer, Mix: baseMix, GapRepair: baseGap, ApplyPitch: false},
	{Name: "waveform", Renderer: "waveform", Mix: baseMix, GapRepair: baseGap, ApplyPitch: true},
}

func presetNames() []string {
	names := make([]string, len(presetTable))
	for i, p := range presetTable {
		names[i] = p.Name
	}
	return names
}

func resolvePresets(names string) ([]preset, error) {
	lookup := make(map[string]preset, len(presetTable))
	for _, p := range presetTable {
		lookup[p.Name] = p
	}
	var selected []preset
	seen := map[string]bool{}
	for _, raw := range strings.Split(names, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		p, ok := lookup[name]
		if !ok {
			return nil, fmt.Errorf("unknown preset %q; valid presets: %s", name, strings.Join(presetNames(), ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		selected = append(selected, p)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no presets selected; valid presets: %s", strings.Join(presetNames(), ", "))
	}
	return selected, nil
}

type sweepRequest struct {
	bank, out, presets, aliasPolicy, bridge  string
	model, modelFile, experiment, phonemizer string
	corpusData                               []byte
	prompts                                  []prompt
	moraMS                                   float64
	wordEnvelope                             bool
	contextDuration                          bool
	contextDurationStrength                  float64
	timeout                                  time.Duration
}

type sweepPresetJSON struct {
	Name         string `json:"name"`
	Renderer     string `json:"renderer"`
	Mix          string `json:"mix"`
	GapRepair    string `json:"gap_repair"`
	SpeechTiming bool   `json:"speech_timing"`
	ApplyPitch   bool   `json:"apply_pitch"`
}

type sweepUnits struct {
	V13       int `json:"v13"`
	Adaptive  int `json:"adaptive"`
	GapRepair int `json:"gap_repair"`
	StopBurst int `json:"stop_burst"`
	Silent    int `json:"silent"`
	Missing   int `json:"missing"`
}

type sweepPromptJSON struct {
	ID              string                `json:"id"`
	Text            string                `json:"text"`
	SelectionSHA256 map[string]string     `json:"selection_sha256"`
	AudioMS         map[string]float64    `json:"audio_ms"`
	Units           map[string]sweepUnits `json:"units"`
}

type sweepJSON struct {
	CorpusSHA256 string            `json:"corpus_sha256"`
	Voicebank    string            `json:"voicebank"`
	Model        string            `json:"model"`
	Renderers    []string          `json:"renderers"`
	Presets      []sweepPresetJSON `json:"presets"`
	Prompts      []sweepPromptJSON `json:"prompts"`
	Build        *debug.BuildInfo  `json:"build,omitempty"`
}

type sweepPromptData struct {
	selectionSHA map[string]string
	audioMS      map[string]float64
	units        map[string]sweepUnits
}

func runSweep(req sweepRequest) error {
	selected, err := resolvePresets(req.presets)
	if err != nil {
		return err
	}
	catalog, err := plugin.DiscoverWithDefaults(nil, nil, render.IsKnownRenderer)
	if err != nil {
		return err
	}
	prosody, ok := catalog.Model(req.model)
	modelIdentity := req.model
	if req.modelFile != "" {
		modelIdentity = req.modelFile
	}
	if !ok && req.model != "none" && req.modelFile == "" {
		return fmt.Errorf("unknown model %q", req.model)
	}
	prosodyPath := ""
	if req.model != "none" {
		prosodyPath = prosody.Path
	}
	if req.modelFile != "" {
		prosodyPath = req.modelFile
	}
	// 出力先は存在してはならない。
	if err := os.MkdirAll(filepath.Dir(req.out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(req.out, 0755); err != nil {
		return err
	}
	buildInfo, _ := debug.ReadBuildInfo()
	corpusSHA := fmt.Sprintf("%x", sha256.Sum256(req.corpusData))
	promptData := make([]sweepPromptData, len(req.prompts))
	for i := range promptData {
		promptData[i] = sweepPromptData{selectionSHA: map[string]string{}, audioMS: map[string]float64{}, units: map[string]sweepUnits{}}
	}
	failed := false
	for _, ps := range selected {
		dir := filepath.Join(req.out, ps.Name)
		if err := os.Mkdir(dir, 0755); err != nil {
			return err
		}
		fmt.Printf("sweep %s: renderer=%s mix=%s gap=%s speech-timing=%t apply-pitch=%t\n", ps.Name, ps.Renderer, ps.Mix, ps.GapRepair, ps.SpeechTiming, ps.ApplyPitch)
		var rows []measurement
		for index, p := range req.prompts {
			row := measurement{ID: p.ID, Text: p.Text, Focus: p.Focus, Renderer: ps.Renderer, Repetition: 1}
			result, elapsed, callErr := synthesizeCase(p, caseOptions{
				bank: req.bank, aliasPolicy: req.aliasPolicy, bridge: req.bridge,
				model: req.model, modelFile: req.modelFile, prosodyModelPath: prosodyPath,
				moraMS: req.moraMS, experiment: req.experiment, wordEnvelope: req.wordEnvelope,
				rendererID: ps.Renderer, mix: ps.Mix, gapRepair: ps.GapRepair,
				speechTiming: ps.SpeechTiming, applyPitch: ps.ApplyPitch, timeout: req.timeout,
				contextDuration: req.contextDuration, contextDurationStrength: req.contextDurationStrength,
			}, catalog)
			row.ElapsedMS = elapsed
			if callErr == nil {
				renderedPlan := fillMeasurement(&row, result)
				wav := fmt.Sprintf("%02d-%s.wav", index+1, p.ID)
				row.WAV = wav
				callErr = synth.WriteFiles(filepath.Join(dir, wav), result, synth.ExportOptions{Text: p.Text, WriteText: true, WriteLab: true})
				if callErr == nil {
					var planData []byte
					planData, callErr = json.MarshalIndent(renderedPlan, "", "  ")
					if callErr == nil {
						callErr = atomicfile.WriteFile(filepath.Join(dir, strings.TrimSuffix(wav, ".wav")+".plan.json"), planData)
					}
				}
				if callErr == nil {
					promptData[index].selectionSHA[ps.Name] = selectionPlanDigest(result.Plan)
					promptData[index].audioMS[ps.Name] = row.AudioMS
					promptData[index].units[ps.Name] = unitsFrom(row)
				}
			}
			if callErr != nil {
				row.Error = callErr.Error()
				failed = true
			}
			rows = append(rows, row)
			fmt.Printf("%s %s: %.0f ms, RTF %.3f %s\n", ps.Name, p.ID, row.ElapsedMS, row.RTF, row.Error)
			report := evalReport{req.wordEnvelope, req.moraMS, req.experiment, req.phonemizer, false, ps.SpeechTiming, ps.Mix, ps.GapRepair, runtime.GOOS, runtime.GOARCH, req.bank, modelIdentity, corpusSHA, req.bridge, buildInfo, rows}
			encoded, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			if err := atomicfile.WriteFile(filepath.Join(dir, "report.json"), encoded); err != nil {
				return err
			}
		}
	}
	report := buildSweepJSON(req, selected, promptData, modelIdentity, corpusSHA, buildInfo)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.WriteFile(filepath.Join(req.out, "sweep.json"), encoded); err != nil {
		return err
	}
	if err := writeSweepIndex(filepath.Join(req.out, "index.md"), selected, req.prompts, promptData); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("some synthesis cases failed; see sweep.json")
	}
	return nil
}

func unitsFrom(row measurement) sweepUnits {
	return sweepUnits{
		V13:       row.V13CompatibleUnits,
		Adaptive:  row.AdaptiveUnits,
		GapRepair: row.GapRepairUnits,
		StopBurst: row.StopBurstUnits,
		Silent:    row.SilentUnits,
		Missing:   row.MissingPhoneGroups,
	}
}

func buildSweepJSON(req sweepRequest, selected []preset, promptData []sweepPromptData, modelIdentity, corpusSHA string, buildInfo *debug.BuildInfo) sweepJSON {
	report := sweepJSON{CorpusSHA256: corpusSHA, Voicebank: req.bank, Model: modelIdentity, Build: buildInfo}
	seen := map[string]bool{}
	for _, ps := range selected {
		if !seen[ps.Renderer] {
			seen[ps.Renderer] = true
			report.Renderers = append(report.Renderers, ps.Renderer)
		}
		report.Presets = append(report.Presets, sweepPresetJSON{ps.Name, ps.Renderer, ps.Mix, ps.GapRepair, ps.SpeechTiming, ps.ApplyPitch})
	}
	for index, p := range req.prompts {
		out := sweepPromptJSON{ID: p.ID, Text: p.Text, SelectionSHA256: promptData[index].selectionSHA, AudioMS: promptData[index].audioMS, Units: promptData[index].units}
		report.Prompts = append(report.Prompts, out)
	}
	return report
}

func writeSweepIndex(path string, selected []preset, prompts []prompt, promptData []sweepPromptData) error {
	var b strings.Builder
	b.WriteString("# Sweep\n\n")
	for _, ps := range selected {
		fmt.Fprintf(&b, "- **%s**: renderer=%s, mix=%s, gap_repair=%s, speech_timing=%t, apply_pitch=%t\n", ps.Name, ps.Renderer, ps.Mix, ps.GapRepair, ps.SpeechTiming, ps.ApplyPitch)
	}
	b.WriteString("\n| prompt |")
	for _, ps := range selected {
		fmt.Fprintf(&b, " %s |", ps.Name)
	}
	b.WriteString("\n| --- |")
	for range selected {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for index, p := range prompts {
		fmt.Fprintf(&b, "| %s |", p.ID)
		for _, ps := range selected {
			wav := fmt.Sprintf("%02d-%s.wav", index+1, p.ID)
			sha := promptData[index].selectionSHA[ps.Name]
			if sha == "" {
				fmt.Fprintf(&b, " [wav](%s/%s) error |", ps.Name, wav)
				continue
			}
			u := promptData[index].units[ps.Name]
			fmt.Fprintf(&b, " [wav](%s/%s) v13:%d ad:%d gr:%d sb:%d sil:%d |", ps.Name, wav, u.V13, u.Adaptive, u.GapRepair, u.StopBurst, u.Silent)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## same selection plan\n\n")
	any := false
	for index, p := range prompts {
		groups := map[string][]string{}
		var order []string
		for _, ps := range selected {
			sha := promptData[index].selectionSHA[ps.Name]
			if sha == "" {
				continue
			}
			if _, ok := groups[sha]; !ok {
				order = append(order, sha)
			}
			groups[sha] = append(groups[sha], ps.Name)
		}
		var same []string
		for _, sha := range order {
			if len(groups[sha]) > 1 {
				same = append(same, strings.Join(groups[sha], " = "))
			}
		}
		if len(same) > 0 {
			any = true
			fmt.Fprintf(&b, "- %s: %s\n", p.ID, strings.Join(same, "; "))
		}
	}
	if !any {
		b.WriteString("(no preset pairs shared a selection plan)\n")
	}
	return atomicfile.WriteFile(path, []byte(b.String()))
}
