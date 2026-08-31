package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"utautts/internal/appinfo"
	"utautts/internal/openutau"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/sidecar"
	"utautts/internal/synth"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

func main() {
	var (
		voicebankPath           string
		otoPath                 string
		reading                 string
		text                    string
		tone                    string
		color                   string
		outPath                 string
		planPath                string
		ustxOut                 string
		dictionaryPath          string
		moraDurationsPath       string
		moraMS                  float64
		pauseMS                 float64
		leadingPreutteranceMS   float64
		releaseMS               float64
		prosodyPath             string
		manualPitchPath         string
		prosodyFeaturesPath     string
		prosodyFeaturesCase     string
		prosodyPitchOnly        bool
		openJTalkPath           string
		openJTalkDictionaryPath string
		pitchContourPath        string
		pitchContourCase        string
		applyPitch              bool
		intonationStrength      float64
		renderer                string
		worldlinePath           string
		worldlineBridgePath     string
		boundaryBridgeMS        float64
		boundaryBridgeThreshold float64
		cvvcTiming              string
		cvvcTransitionGain      float64
		cvvcPreBoundaryFade     bool
		selectionMode           string
		aliasPolicy             string
		acousticMode            string
		joinModelPath           string
		joinScoreScale          float64
		rendererDirectories     []string
		modelDirectories        []string
		writeText               bool
		writeLab                bool
		textEncoding            string
		showVersion             bool
	)
	flag.BoolVar(&showVersion, "version", false, "print application version")
	flag.StringVar(&voicebankPath, "voicebank", "", "path to a UTAU voicebank directory")
	flag.StringVar(&otoPath, "oto", "", "deprecated alias for --voicebank")
	flag.StringVar(&reading, "kana", "", "kana reading to synthesize")
	flag.StringVar(&text, "text", "", "Japanese text to synthesize")
	flag.StringVar(&tone, "tone", "C4", "voicebank tone used with prefix.map")
	flag.StringVar(&color, "color", "", "voicebank subbank/color (character.yaml)")
	flag.StringVar(&outPath, "out", "", "output WAV path")
	flag.StringVar(&planPath, "plan-out", "", "optional synthesis plan JSON path")
	flag.StringVar(&ustxOut, "ustx-out", "", "optional OpenUtau USTX project path")
	flag.StringVar(&dictionaryPath, "dictionary", "", "optional user dictionary JSON path")
	flag.StringVar(&moraDurationsPath, "mora-durations", "", "optional mora duration JSON path")
	flag.Float64Var(&moraMS, "mora-ms", 140, "base mora duration in milliseconds")
	flag.Float64Var(&pauseMS, "pause-ms", 180, "punctuation pause in milliseconds")
	flag.Float64Var(&leadingPreutteranceMS, "leading-preutterance-ms", 0, "leading preutterance in milliseconds (0 uses oto.ini)")
	flag.Float64Var(&releaseMS, "release-ms", 20, "unit release envelope in milliseconds")
	flag.StringVar(&prosodyPath, "prosody", "", "optional prosody model plugin ID")
	flag.StringVar(&manualPitchPath, "manual-pitch", "", "optional mora pitch edit JSON")
	flag.StringVar(&prosodyFeaturesPath, "prosody-features", "", "optional per-case mora-level accent feature JSON")
	flag.StringVar(&prosodyFeaturesCase, "prosody-feature-case", "", "case ID in --prosody-features")
	flag.BoolVar(&prosodyPitchOnly, "prosody-pitch-only", false, "apply only learned pitch and keep fixed duration/energy")
	flag.StringVar(&openJTalkPath, "openjtalk-features", "", "path to the Open JTalk feature helper (default: runtime directory)")
	flag.StringVar(&openJTalkDictionaryPath, "openjtalk-dictionary", "", "path to the Open JTalk dictionary (default: runtime directory)")
	flag.StringVar(&pitchContourPath, "pitch-contours", "", "optional per-case pitch contour JSON (recorded in the plan; use --apply-pitch for direct waveform processing)")
	flag.StringVar(&pitchContourCase, "pitch-case", "", "case ID in --pitch-contours")
	flag.BoolVar(&applyPitch, "apply-pitch", false, "experimental waveform pitch resampling")
	flag.Float64Var(&intonationStrength, "intonation-strength", 0, "experimental source-pitch stabilization and phrase contour strength (0..2)")
	flag.StringVar(&renderer, "renderer", "", "renderer plugin ID (default: highest manifest priority)")
	flag.StringVar(&worldlinePath, "worldline", "", "path to OpenUtau worldline library (default: next to executable)")
	flag.StringVar(&worldlineBridgePath, "worldline-bridge", "", "path to utautts-worldline-bridge executable")
	flag.Float64Var(&boundaryBridgeMS, "boundary-bridge-ms", 0, "maximum width for phase-aligned waveform boundary repair candidates (0 disables)")
	flag.Float64Var(&boundaryBridgeThreshold, "boundary-bridge-threshold", 0, "apply boundary repair when handcrafted join score is at or below this value")
	flag.StringVar(&cvvcTiming, "cvvc-timing", render.CVVCTimingLegacy, "CVVC timing: legacy or sequential")
	flag.Float64Var(&cvvcTransitionGain, "cvvc-transition-gain", 1, "CVVC transition volume multiplier (0..1)")
	flag.BoolVar(&cvvcPreBoundaryFade, "cvvc-pre-boundary-fade", false, "fade CVVC transitions out before the following CV consonant")
	flag.StringVar(&selectionMode, "selection", string(voicebank.SelectionViterbi), "unit selection: viterbi, greedy, or target-only")
	flag.StringVar(&aliasPolicy, "alias-policy", string(voicebank.AliasPolicyAuto), "voicebank mode: auto, legacy, cvvc-enhanced, vcv-prefer, cvvc-prefer, or cv-only")
	flag.StringVar(&acousticMode, "acoustic-selection", "", "acoustic candidate diagnostics: dry-run or apply")
	flag.StringVar(&joinModelPath, "join-model", "", "optional learned join-cost model JSON")
	flag.Float64Var(&joinScoreScale, "join-scale", 0, "learned logit score scale (default: model or 4)")
	flag.BoolVar(&writeText, "write-text", false, "write a text file next to the WAV")
	flag.BoolVar(&writeLab, "write-lab", false, "write an HTK label file next to the WAV")
	flag.StringVar(&textEncoding, "text-encoding", sidecar.EncodingUTF8, "text sidecar encoding: utf-8 or shift_jis")
	flag.Func("renderer-dir", "renderer plugin directory (repeatable)", func(value string) error { rendererDirectories = append(rendererDirectories, value); return nil })
	flag.Func("model-dir", "prosody model directory (repeatable)", func(value string) error { modelDirectories = append(modelDirectories, value); return nil })
	flag.Parse()
	if showVersion {
		fmt.Printf("%s %s\n", appinfo.Name(), appinfo.Version())
		return
	}
	catalog, catalogErr := plugin.DiscoverWithDefaults(rendererDirectories, modelDirectories, render.IsKnownRenderer)
	if catalogErr != nil {
		log.Printf("plugin discovery warning: %v", catalogErr)
	}
	if prosodyPath != "" {
		model, found := catalog.Model(prosodyPath)
		if !found {
			log.Fatalf("prosody model plugin %q is not installed", prosodyPath)
		}
		prosodyPath = model.Path
	}

	if voicebankPath == "" {
		voicebankPath = otoPath
	}
	if voicebankPath == "" || (reading == "" && text == "") || outPath == "" {
		flag.Usage()
		log.Fatal("--voicebank, --out, and either --text or --kana are required")
	}

	pitchFactors, err := loadPitchFactors(pitchContourPath, pitchContourCase)
	if err != nil {
		log.Fatal(err)
	}
	prosodyFeatures, err := loadProsodyFeatures(prosodyFeaturesPath, prosodyFeaturesCase, text, reading)
	if err != nil {
		log.Fatal(err)
	}
	dictionary, err := loadDictionary(dictionaryPath)
	if err != nil {
		log.Fatal(err)
	}
	moraDurations, err := loadMoraDurations(moraDurationsPath)
	if err != nil {
		log.Fatal(err)
	}
	synthConfig := tts.Config{
		VoicebankPath:           voicebankPath,
		Text:                    text,
		Reading:                 reading,
		Dictionary:              synth.DictionaryMap(dictionary),
		Tone:                    tone,
		Color:                   color,
		MoraDurationMS:          moraMS,
		PauseDurationMS:         pauseMS,
		MoraDurationsMS:         moraDurations,
		LeadingPreutteranceMS:   leadingPreutteranceMS,
		ReleaseMS:               releaseMS,
		ReleaseSet:              true,
		ProsodyModelPath:        prosodyPath,
		ManualPitchPath:         manualPitchPath,
		ProsodyFeatures:         prosodyFeatures,
		ProsodyPitchOnly:        prosodyPitchOnly,
		OpenJTalkPath:           openJTalkPath,
		OpenJTalkDictionaryPath: openJTalkDictionaryPath,
		PitchFactors:            pitchFactors,
		ApplyPitch:              applyPitch,
		IntonationStrength:      intonationStrength,
		BoundaryBridgeMS:        boundaryBridgeMS,
		BoundaryBridgeThreshold: boundaryBridgeThreshold,
		CVVCTiming:              cvvcTiming,
		CVVCTransitionGain:      cvvcTransitionGain,
		CVVCPreBoundaryFade:     cvvcPreBoundaryFade,
		SelectionMode:           voicebank.SelectionMode(selectionMode),
		AliasPolicy:             voicebank.AliasPolicy(aliasPolicy),
		AcousticMode:            acousticMode,
		JoinModelPath:           joinModelPath,
		JoinScoreScale:          joinScoreScale,
	}
	if err := tts.ApplyRenderer(&synthConfig, catalog, renderer, worldlinePath, worldlineBridgePath); err != nil {
		log.Fatal(err)
	}
	rendererID := renderer
	if rendererID == "" {
		rendererID = catalog.DefaultRenderer()
	}
	output, err := synth.SynthesizeConfig(synthConfig, rendererID)
	if err != nil {
		log.Fatal(err)
	}
	exportText := text
	if exportText == "" {
		exportText = reading
	}
	if err := synth.WriteFiles(outPath, output, synth.ExportOptions{
		Text: exportText, WriteText: writeText, WriteLab: writeLab, TextEncoding: textEncoding,
	}); err != nil {
		log.Fatal(err)
	}
	if planPath != "" {
		data, err := json.MarshalIndent(output.Plan, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(planPath, data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if ustxOut != "" {
		project := ustxProjectFromSynthesis(synthConfig, output.Plan, filepath.Base(filepath.Clean(voicebankPath)))
		data, err := openutau.ExportUSTX(project, openutau.ExportOptions{
			Curves: ustxFrameCurves(synthConfig, 1),
		})
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(ustxOut, data, 0o644); err != nil {
			log.Fatal(err)
		}
	}

	duration := output.DurationMS / 1000
	fmt.Printf("wrote %s (%.2fs, %d Hz, %d units)\n", outPath, duration, output.Audio.SampleRate, len(output.Plan.Units))
}

func loadDictionary(path string) ([]synth.DictionaryEntry, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []synth.DictionaryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode user dictionary: %w", err)
	}
	for index, entry := range entries {
		if entry.Surface == "" || entry.Reading == "" {
			return nil, fmt.Errorf("dictionary entry %d requires surface and reading", index+1)
		}
	}
	return entries, nil
}

func loadMoraDurations(path string) ([]float64, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var durations []float64
	if err := json.Unmarshal(data, &durations); err == nil {
		return durations, nil
	}
	var value struct {
		MoraDurationsMS []float64 `json:"mora_durations_ms"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode mora durations: %w", err)
	}
	return value.MoraDurationsMS, nil
}

func loadProsodyFeatures(path, caseID, text, reading string) ([]prosody.FeatureFrame, error) {
	if path == "" {
		return nil, nil
	}
	corpus, err := prosody.LoadFeatureCorpus(path)
	if err != nil {
		return nil, err
	}
	item, err := corpus.Select(caseID, text, reading)
	if err != nil {
		return nil, fmt.Errorf("select prosody features from %s: %w", path, err)
	}
	return item.Features, nil
}

func loadPitchFactors(path, caseID string) ([]float64, error) {
	if path == "" {
		return nil, nil
	}
	if caseID == "" {
		return nil, fmt.Errorf("--pitch-case is required with --pitch-contours")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var corpus struct {
		Cases []struct {
			ID           string    `json:"id"`
			PitchFactors []float64 `json:"pitch_factors"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, err
	}
	for _, item := range corpus.Cases {
		if item.ID == caseID {
			return item.PitchFactors, nil
		}
	}
	return nil, fmt.Errorf("pitch contour case %q not found in %s", caseID, path)
}
