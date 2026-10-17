package main

import (
	"context"
	"encoding/json"
	"errors"
	flagpkg "flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	utautts "github.com/yh2237/UtauTTS"
	"github.com/yh2237/UtauTTS/internal/appinfo"
	"github.com/yh2237/UtauTTS/internal/entryinput"
	"github.com/yh2237/UtauTTS/internal/openutau"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/sidecar"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func main() {
	os.Exit(executeCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func executeCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := runCLI(args, stdin, stdout, stderr)
	if err != nil {
		log.New(stderr, "", log.LstdFlags).Print(err)
	}
	return cliExitCode(err)
}

func runCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) (runErr error) {
	flag := flagpkg.NewFlagSet("utautts-cli", flagpkg.ContinueOnError)
	flag.SetOutput(stderr)
	var (
		voicebankPath            string
		reading                  string
		text                     string
		textFile                 string
		readingFile              string
		requestFile              string
		tone                     string
		color                    string
		outPath                  string
		planPath                 string
		ustxOut                  string
		dictionaryPath           string
		moraDurationsPath        string
		resamplerExpressionsPath string
		moraMS                   float64
		pauseMS                  float64
		leadingPreutteranceMS    float64
		releaseMS                float64
		prosodyPath              string
		manualPitchPath          string
		prosodyFeaturesPath      string
		prosodyFeaturesCase      string
		prosodyPitchOnly         bool
		openJTalkPath            string
		openJTalkDictionaryPath  string
		pitchContourPath         string
		pitchContourCase         string
		applyPitch               bool
		intonationStrength       float64
		contextDuration          bool
		contextDurationStrength  float64
		boundaryTone             bool
		boundaryToneStrength     float64
		stretchAdapt             bool
		stretchAdaptStrength     float64
		pauseContext             bool
		pauseContextStrength     float64
		timingWarp               bool
		microprosody             bool
		renderer                 string
		resampler                string
		wavtool                  string
		worldlineBridgePath      string
		cvvcTiming               string
		cvvcTransitionGain       float64
		cvvcPreBoundaryFade      bool
		language                 string
		phonemizer               string
		aliasPolicy              string
		diffSingerSteps          int64
		diffSingerDurationMix    float64
		diffSingerPitchMix       float64
		diffSingerExpr           float64
		rendererDirectories      []string
		modelDirectories         []string
		writeText                bool
		writeLab                 bool
		textEncoding             string
		showVersion              bool
		jsonOutput               bool
		listKind                 string
		doctor                   bool
		timeout                  time.Duration
	)
	flag.BoolVar(&showVersion, "version", false, "print application version")
	flag.BoolVar(&jsonOutput, "json", false, "print result, catalog, doctor, version or error JSON to standard output")
	flag.StringVar(&listKind, "list", "", "list voicebanks, models, renderers, resamplers or wavtools without synthesizing")
	flag.BoolVar(&doctor, "doctor", false, "inspect selected resources without starting helpers or synthesizing")
	flag.DurationVar(&timeout, "timeout", 0, "synthesis timeout after input and initialization (e.g. 30s; 0 disables)")
	flag.StringVar(&voicebankPath, "voicebank", "", "path to a UTAU voicebank directory")
	flag.StringVar(&reading, "kana", "", "kana reading to synthesize")
	flag.StringVar(&reading, "reading", "", "pronunciation to synthesize (kana, ARPAbet, or Pinyin)")
	flag.StringVar(&text, "text", "", "text to synthesize")
	flag.StringVar(&textFile, "text-file", "", "UTF-8 text file (- reads standard input; exclusive with --text)")
	flag.StringVar(&readingFile, "reading-file", "", "UTF-8 pronunciation file (- reads standard input; exclusive with --reading/--kana)")
	flag.StringVar(&requestFile, "request", "", "synthesis request JSON file (- reads standard input; exclusive with utterance flags)")
	flag.StringVar(&language, "language", "ja", "language: ja, en, or zh")
	flag.StringVar(&phonemizer, "phonemizer", "", "phonemizer ID (default: selected by language)")
	flag.StringVar(&tone, "tone", "", "base pitch such as C4, also used with prefix.map (empty: the voicebank's recorded pitch)")
	flag.StringVar(&color, "color", "", "voicebank subbank/color (character.yaml)")
	flag.StringVar(&outPath, "out", "", "output WAV path (- writes standard output)")
	flag.StringVar(&planPath, "plan-out", "", "optional synthesis plan JSON path")
	flag.StringVar(&ustxOut, "ustx-out", "", "optional OpenUtau USTX project path")
	flag.StringVar(&dictionaryPath, "dictionary", "", "optional user dictionary JSON path")
	flag.StringVar(&moraDurationsPath, "mora-durations", "", "optional mora duration JSON path")
	flag.StringVar(&resamplerExpressionsPath, "resampler-expressions", "", "optional per-mora resampler expression JSON path")
	flag.Float64Var(&moraMS, "mora-ms", synth.DefaultMoraDurationMS, "base mora duration in milliseconds")
	flag.Float64Var(&pauseMS, "pause-ms", synth.DefaultPauseDurationMS, "punctuation pause in milliseconds")
	flag.Float64Var(&leadingPreutteranceMS, "leading-preutterance-ms", 0, "leading preutterance in milliseconds (0 uses oto.ini)")
	flag.Float64Var(&releaseMS, "release-ms", synth.DefaultReleaseMS, "unit release envelope in milliseconds")
	flag.StringVar(&prosodyPath, "prosody", "", "optional prosody model plugin ID")
	flag.StringVar(&manualPitchPath, "manual-pitch", "", "optional mora pitch edit JSON")
	flag.StringVar(&prosodyFeaturesPath, "prosody-features", "", "optional per-case mora-level accent feature JSON")
	flag.StringVar(&prosodyFeaturesCase, "prosody-feature-case", "", "case ID in --prosody-features")
	flag.BoolVar(&prosodyPitchOnly, "prosody-pitch-only", false, "apply only learned pitch and keep fixed duration/energy")
	flag.StringVar(&openJTalkPath, "openjtalk-features", "", "path to the Open JTalk feature helper (default: runtime directory)")
	flag.StringVar(&openJTalkDictionaryPath, "openjtalk-dictionary", "", "path to the Open JTalk dictionary (default: runtime directory)")
	flag.StringVar(&pitchContourPath, "pitch-contours", "", "optional per-case pitch contour JSON (recorded in the plan; use --apply-pitch to render it)")
	flag.StringVar(&pitchContourCase, "pitch-case", "", "case ID in --pitch-contours")
	flag.BoolVar(&applyPitch, "apply-pitch", synth.DefaultApplyPitch, "apply the pitch curve")
	flag.Float64Var(&intonationStrength, "intonation-strength", synth.DefaultIntonationStrength, "intonation strength (0..8; above 2, larger contour movements are widened more)")
	flag.BoolVar(&contextDuration, "context-duration", synth.DefaultContextDuration, "context-aware Japanese mora duration (C1)")
	flag.Float64Var(&contextDurationStrength, "context-duration-strength", synth.DefaultContextDurationStrength, "context-aware duration strength (0 uses the default 1.0)")
	flag.BoolVar(&boundaryTone, "boundary-tone", synth.DefaultBoundaryTone, "Japanese phrase-final boundary tone (C2)")
	flag.Float64Var(&boundaryToneStrength, "boundary-tone-strength", synth.DefaultBoundaryToneStrength, "boundary tone strength (0 uses the default 1.0)")
	flag.BoolVar(&stretchAdapt, "stretch-adapt", synth.DefaultStretchAdapt, "adapt Japanese mora stretching to voicebank measurements (C3a)")
	flag.Float64Var(&stretchAdaptStrength, "stretch-adapt-strength", synth.DefaultStretchAdaptStrength, "stretch adaptation strength (0 uses the default 1.0)")
	flag.BoolVar(&pauseContext, "pause-context", synth.DefaultPauseContext, "context-aware punctuation pause length (B5)")
	flag.Float64Var(&pauseContextStrength, "pause-context-strength", synth.DefaultPauseContextStrength, "pause context strength (0 uses the default 1.0)")
	flag.BoolVar(&microprosody, "microprosody", render.WorldlineProviderOptions{}.MicroprosodyEnabled(), "add measured consonant-induced F0 microprosody to Japanese WORLD output (utautts-world-phrase)")
	flag.BoolVar(&timingWarp, "timing-warp", render.WorldlineProviderOptions{}.TimingWarpEnabled(), "warp Japanese WORLD output timing toward learned speech movement (utautts-world-phrase)")
	flag.StringVar(&renderer, "renderer", "", "renderer ID (default: highest configured priority)")
	flag.StringVar(&resampler, "resampler", "", "Classic UTAU resampler ID from Resamplers")
	flag.StringVar(&wavtool, "wavtool", "builtin", "Classic UTAU wavtool ID from Wavtools")
	flag.StringVar(&worldlineBridgePath, "worldline-bridge", "", "path to utautts-worldline-bridge executable")
	flag.StringVar(&cvvcTiming, "cvvc-timing", render.CVVCTimingSequential, "CVVC timing: sequential")
	flag.Float64Var(&cvvcTransitionGain, "cvvc-transition-gain", 1, "CVVC transition volume multiplier (0..1)")
	flag.BoolVar(&cvvcPreBoundaryFade, "cvvc-pre-boundary-fade", false, "fade CVVC transitions out before the following CV consonant")
	flag.StringVar(&aliasPolicy, "alias-policy", string(voicebank.AliasPolicyAuto), "voicebank mode: auto, cvvc-enhanced, vcv-prefer, cvvc-prefer, or cv-only")
	flag.Int64Var(&diffSingerSteps, "diffsinger-steps", 0, "DiffSinger diffusion steps (0 uses the default)")
	flag.Float64Var(&diffSingerDurationMix, "diffsinger-duration-mix", 0, "DiffSinger duration predictor mix 0..1 (0 uses the default)")
	flag.Float64Var(&diffSingerPitchMix, "diffsinger-pitch-mix", 0, "DiffSinger pitch predictor mix 0..1 (0 uses the default)")
	flag.Float64Var(&diffSingerExpr, "diffsinger-expr", 0, "DiffSinger expressiveness 0..2 (0 uses the default 1.0)")
	flag.BoolVar(&writeText, "write-text", false, "write a text file next to the WAV")
	flag.BoolVar(&writeLab, "write-lab", false, "write an HTK label file next to the WAV")
	flag.StringVar(&textEncoding, "text-encoding", sidecar.EncodingUTF8, "text sidecar encoding: utf-8 or shift_jis")
	flag.Func("renderer-dir", "renderer plugin directory (repeatable)", func(value string) error { rendererDirectories = append(rendererDirectories, value); return nil })
	flag.Func("model-dir", "prosody model directory (repeatable)", func(value string) error { modelDirectories = append(modelDirectories, value); return nil })
	if err := flag.Parse(args); err != nil {
		if errors.Is(err, flagpkg.ErrHelp) {
			return nil
		}
		return cliFailure("flag_parse_error", err)
	}
	var report cliResponse
	if jsonOutput {
		// Runtimeの終了まで待ち、終了・出力の失敗もJSON結果へ反映する。
		defer func() {
			report.OK = runErr == nil
			if runErr != nil {
				report = cliResponse{Doctor: report.Doctor, Error: &cliErrorReport{Code: cliErrorCode(runErr), Message: runErr.Error()}}
			}
			if err := json.NewEncoder(stdout).Encode(report); err != nil {
				runErr = errors.Join(runErr, cliFailure("output_error", fmt.Errorf("write result JSON: %w", err)))
			}
		}()
	}
	if showVersion {
		if jsonOutput {
			report.Version = &cliVersionReport{Name: appinfo.Name(), Version: appinfo.Version()}
			return nil
		}
		_, err := fmt.Fprintf(stdout, "%s %s\n", appinfo.Name(), appinfo.Version())
		return cliFailure("output_error", err)
	}
	if timeout < 0 {
		return cliFailure("invalid_arguments", errors.New("--timeout must not be negative"))
	}
	runtimeConfig := synth.RuntimeConfig{
		Renderer: renderer, WorldlineBridgePath: worldlineBridgePath,
		OpenJTalkPath: openJTalkPath, OpenJTalkDictionary: openJTalkDictionaryPath,
		RendererDirectories: rendererDirectories, ModelDirectories: modelDirectories,
	}
	if doctor {
		if conflict := flagOutside(flag, doctorFlags); conflict != "" {
			return cliFailure("invalid_arguments", fmt.Errorf("--doctor cannot be combined with --%s", conflict))
		}
		diagnosis, err := inspectEnvironment(voicebankPath, prosodyPath, resampler, wavtool, runtimeConfig)
		if jsonOutput {
			report.Doctor = diagnosis
			return err
		}
		if diagnosis != nil {
			err = errors.Join(err, cliFailure("output_error", writeDoctor(stdout, diagnosis)))
		}
		return err
	}
	if listKind != "" {
		if conflict := flagOutside(flag, catalogFlags); conflict != "" {
			return cliFailure("invalid_arguments", fmt.Errorf("--list cannot be combined with --%s", conflict))
		}
		catalog, err := loadCatalog(listKind, voicebankPath, runtimeConfig)
		if err != nil {
			return err
		}
		if jsonOutput {
			report.Catalog = catalog
			return nil
		}
		return cliFailure("output_error", writeCatalog(stdout, catalog))
	}
	if voicebankPath == "" || outPath == "" {
		if !jsonOutput {
			flag.Usage()
		}
		return cliFailure("invalid_arguments", errors.New("--voicebank and --out are required"))
	}
	var textSpecified, readingSpecified bool
	flag.Visit(func(option *flagpkg.Flag) {
		switch option.Name {
		case "text":
			textSpecified = true
		case "reading", "kana":
			readingSpecified = true
		}
	})
	if requestFile != "" {
		if conflict := flagOutside(flag, requestTransportFlags); conflict != "" {
			return cliFailure("invalid_arguments", fmt.Errorf("--request cannot be combined with --%s; put utterance settings in JSON", conflict))
		}
	}
	if textFile != "" && textSpecified || readingFile != "" && readingSpecified {
		return cliFailure("invalid_arguments", errors.New("--text-file conflicts with --text; --reading-file conflicts with --reading/--kana"))
	}
	if textFile == "-" && readingFile == "-" {
		return cliFailure("invalid_arguments", errors.New("only one input can read standard input"))
	}
	if outPath == "-" && (writeText || writeLab) {
		return cliFailure("invalid_arguments", errors.New("--write-text and --write-lab require a WAV file path"))
	}
	if outPath == "-" && jsonOutput {
		return cliFailure("invalid_arguments", errors.New("--json cannot be combined with WAV output to standard output"))
	}
	if textFile != "" {
		var err error
		text, err = readUtteranceFile(textFile, stdin)
		if err != nil {
			return cliFailure("input_error", fmt.Errorf("read text input: %w", err))
		}
	}
	if readingFile != "" {
		var err error
		reading, err = readUtteranceFile(readingFile, stdin)
		if err != nil {
			return cliFailure("input_error", fmt.Errorf("read pronunciation input: %w", err))
		}
	}
	var jsonRequest *entryinput.JSONRequest
	if requestFile != "" {
		var err error
		jsonRequest, err = readRequestFile(requestFile, stdin)
		if err != nil {
			return cliFailure("input_error", fmt.Errorf("read request JSON: %w", err))
		}
		if jsonRequest.VoicebankID != "" {
			return cliFailure("invalid_arguments", errors.New("CLI request cannot select voicebank_id; use --voicebank"))
		}
		text, reading = jsonRequest.Text, jsonRequest.ReadingOrKana()
	}
	if reading == "" && text == "" {
		if !jsonOutput {
			flag.Usage()
		}
		return cliFailure("invalid_arguments", errors.New("either --text, --reading, --text-file, --reading-file or --request is required"))
	}
	runtime, err := synth.NewRuntime(runtimeConfig, nil)
	if err != nil {
		return cliFailure("initialization_error", err)
	}
	defer func() { runErr = errors.Join(runErr, cliFailure("shutdown_error", runtime.Close())) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ctx, release, err := runtime.Begin(ctx)
	if err != nil {
		return cliFailure("synthesis_error", err)
	}
	defer release()
	resolver := runtime.Service

	var synthesisRequest synth.Request
	if jsonRequest == nil {
		pitchFactors, err := loadPitchFactors(pitchContourPath, pitchContourCase)
		if err != nil {
			return cliFailure("input_error", err)
		}
		prosodyFeatures, err := loadProsodyFeatures(prosodyFeaturesPath, prosodyFeaturesCase, text, reading)
		if err != nil {
			return cliFailure("input_error", err)
		}
		dictionary, err := loadDictionary(dictionaryPath)
		if err != nil {
			return cliFailure("input_error", err)
		}
		moraDurations, err := loadMoraDurations(moraDurationsPath)
		if err != nil {
			return cliFailure("input_error", err)
		}
		resamplerExpressions, err := loadResamplerExpressions(resamplerExpressionsPath)
		if err != nil {
			return cliFailure("input_error", err)
		}
		request := utautts.Request{
			Text: text, Reading: reading, Language: language, Phonemizer: phonemizer,
			Tone: tone, Color: color, ModelID: prosodyPath, Dictionary: dictionary,
			Options: utautts.Options{
				MoraDurationMS: &moraMS, PauseDurationMS: &pauseMS, ReleaseMS: &releaseMS,
				IntonationStrength: &intonationStrength, ApplyPitch: &applyPitch,
				ContextDuration: &contextDuration, BoundaryTone: &boundaryTone,
				StretchAdapt: &stretchAdapt, PauseContext: &pauseContext,
			},
		}
		// 配置、詳細編集、評価用入力は通常の発話要求に含めない。
		synthesisRequest = entryinput.Apply(request, synth.Request{
			VoicebankPath:           voicebankPath,
			Resampler:               resampler,
			Wavtool:                 wavtool,
			MoraDurationsMS:         moraDurations,
			LeadingPreutteranceMS:   leadingPreutteranceMS,
			ManualPitchPath:         manualPitchPath,
			ProsodyFeatures:         prosodyFeatures,
			ProsodyPitchOnly:        prosodyPitchOnly,
			PitchFactors:            pitchFactors,
			ContextDurationStrength: contextDurationStrength,
			BoundaryToneStrength:    boundaryToneStrength,
			StretchAdaptStrength:    stretchAdaptStrength,
			PauseContextStrength:    pauseContextStrength,
			CVVCTiming:              cvvcTiming,
			CVVCTransitionGain:      cvvcTransitionGain,
			CVVCPreBoundaryFade:     cvvcPreBoundaryFade,
			AliasPolicy:             voicebank.AliasPolicy(aliasPolicy),
			Worldline:               render.WorldlineProviderOptions{TimingWarp: &timingWarp, Microprosody: &microprosody},
			DiffSingerSteps:         diffSingerSteps,
			DiffSingerDurationMix:   diffSingerDurationMix,
			DiffSingerPitchMix:      diffSingerPitchMix,
			DiffSingerExpr:          diffSingerExpr,
			ResamplerExpressions:    resamplerExpressions,
		})
	} else {
		synthesisRequest = jsonRequest.InternalRequest()
		synthesisRequest.VoicebankPath = voicebankPath
		// CLI固有の時間制御は共通JSONに含めない。
		synthesisRequest.CVVCTiming = cvvcTiming
		synthesisRequest.CVVCTransitionGain = cvvcTransitionGain
	}
	if err := ctx.Err(); err != nil {
		return cliFailure("synthesis_error", err)
	}
	resolvedSynthesis, err := resolver.ResolveSynthesis(synthesisRequest)
	if err != nil {
		return cliFailure("invalid_arguments", err)
	}
	resolvedSynthesis.Config.Context = ctx
	output, err := synth.SynthesizeResolved(resolvedSynthesis)
	if err != nil {
		return cliFailure("synthesis_error", err)
	}
	if err := ctx.Err(); err != nil {
		return cliFailure("synthesis_error", err)
	}
	exportText := text
	if exportText == "" {
		exportText = reading
	}
	if outPath != "-" {
		if err := synth.WriteFiles(outPath, output, synth.ExportOptions{
			Text: exportText, WriteText: writeText, WriteLab: writeLab, TextEncoding: textEncoding,
		}); err != nil {
			return cliFailure("output_error", err)
		}
	}
	if planPath != "" {
		data, err := json.MarshalIndent(output.RenderedPlan(), "", "  ")
		if err != nil {
			return cliFailure("output_error", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(planPath, data, 0o644); err != nil {
			return cliFailure("output_error", err)
		}
	}
	if ustxOut != "" {
		project := ustxProjectFromSynthesis(resolvedSynthesis.Config, output.Plan, filepath.Base(filepath.Clean(voicebankPath)))
		curves := ustxFrameCurves(resolvedSynthesis.Config, 1)
		if err := ctx.Err(); err != nil {
			return cliFailure("synthesis_error", err)
		}
		data, err := openutau.ExportUSTX(project, openutau.ExportOptions{
			Curves: curves,
		})
		if err != nil {
			return cliFailure("output_error", err)
		}
		if err := os.WriteFile(ustxOut, data, 0o644); err != nil {
			return cliFailure("output_error", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return cliFailure("synthesis_error", err)
	}
	duration := output.DurationMS / 1000
	summary := stdout
	if outPath == "-" {
		audio := utautts.Audio{SampleRate: output.Audio.SampleRate, Channels: output.Audio.Channels, Samples: output.Audio.Data}
		if err := audio.WriteWAV(stdout); err != nil {
			return cliFailure("output_error", fmt.Errorf("write WAV to standard output: %w", err))
		}
		summary = stderr
	}
	if jsonOutput {
		report.Result = synthesisReport(outPath, output)
		return nil
	}
	_, err = fmt.Fprintf(summary, "wrote %s (%.2fs, %d Hz, %d units)\n", outPath, duration, output.Audio.SampleRate, len(output.Plan.Units))
	return cliFailure("output_error", err)
}

func flagOutside(flags *flagpkg.FlagSet, allowed map[string]bool) string {
	var conflict string
	flags.Visit(func(option *flagpkg.Flag) {
		if !allowed[option.Name] && conflict == "" {
			conflict = option.Name
		}
	})
	return conflict
}

func loadResamplerExpressions(path string) ([]render.ResamplerExpression, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values []render.ResamplerExpression
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("decode resampler expressions: %w", err)
	}
	return values, nil
}

func loadDictionary(path string) ([]utautts.DictionaryEntry, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []utautts.DictionaryEntry
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
