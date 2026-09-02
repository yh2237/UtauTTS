package tts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"utautts/internal/audio"
	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

type Config struct {
	Context                 context.Context
	VoicebankPath           string
	Voicebank               *voicebank.Bank
	Text                    string
	Reading                 string
	Dictionary              map[string]string
	Tone                    string
	Color                   string
	MoraDurationMS          float64
	PauseDurationMS         float64
	MoraDurationsMS         []float64
	ReleaseMS               float64
	ReleaseSet              bool
	LeadingPreutteranceMS   float64
	ProsodyModelPath        string
	ProsodyModel            *prosody.Model
	ManualPitchPath         string
	ManualPitch             *prosody.ManualPitchFile
	ProsodyFeatures         []prosody.FeatureFrame
	ProsodyPitchOnly        bool
	OpenJTalkPath           string
	OpenJTalkDictionaryPath string
	PitchFactors            []float64
	ApplyPitch              bool
	IntonationStrength      float64
	Renderer                string
	RendererCapabilities    *plugin.Capabilities
	WorldlinePath           string
	WorldlineBridgePath     string
	WorldEnginePath         string
	WorldGPUPath            string
	ExternalResamplerPath   string
	BoundaryBridgeMS        float64
	BoundaryBridgeThreshold float64
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	PitchCurve              *render.PitchCurve
	SelectionMode           voicebank.SelectionMode
	AliasPolicy             voicebank.AliasPolicy
	AcousticMode            string
	JoinModelPath           string
	JoinScoreScale          float64
}

type Result struct {
	Voicebank       *voicebank.Bank
	Plan            *plan.Plan
	Audio           *audio.PCM
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
}

type ProsodyPreview struct {
	Reading         string
	Morae           []frontend.Mora
	Features        []prosody.FeatureFrame
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
	// FramePitchCurve is the 10ms frame-level intonation contour (scaled by
	// intonation strength), when the selected model and renderer support it.
	FramePitchCurve *render.PitchCurve
}

// ConvertToReadingは日本語テキストをかなに変換する。内蔵トークナイザが数字やラテン文字などのトークンを読みに変換できない場合はOpen JTalkにフォールバックする。
func ConvertToReading(text string, dictionary map[string]string, openJTalk openjtalk.Config) (string, error) {
	return ConvertToReadingContext(context.Background(), text, dictionary, openJTalk)
}

func ConvertToReadingContext(ctx context.Context, text string, dictionary map[string]string, openJTalk openjtalk.Config) (string, error) {
	if err := synthesisContextError(ctx); err != nil {
		return "", err
	}
	reading, frontendErr := frontend.ToKanaWithDictionary(text, dictionary)
	if frontendErr == nil {
		return reading, nil
	}
	analysis, openJTalkErr := analyzeOpenJTalkCached(ctx, frontend.ApplyDictionaryForAnalysis(text, dictionary), openJTalk)
	if openJTalkErr != nil {
		return "", fmt.Errorf("convert text to reading: %v; Open JTalk fallback: %w", frontendErr, openJTalkErr)
	}
	return analysis.Reading, nil
}

func resolveReading(cfg Config) (string, error) {
	if cfg.Reading != "" {
		return cfg.Reading, nil
	}
	return ConvertToReadingContext(cfg.Context, cfg.Text, cfg.Dictionary, openjtalk.Config{
		HelperPath: cfg.OpenJTalkPath, DictionaryPath: cfg.OpenJTalkDictionaryPath,
	})
}

func resolveProsodyModel(cfg Config) (*prosody.Model, error) {
	if cfg.ProsodyModel != nil {
		return cfg.ProsodyModel, nil
	}
	if cfg.ProsodyModelPath == "" {
		return nil, nil
	}
	return loadProsodyModelCached(cfg.ProsodyModelPath)
}

// resolveProsodyFeaturesは未指定のモーラ単位アクセント特徴をOpen JTalkで補う。
func resolveProsodyFeatures(cfg Config, model *prosody.Model, morae []frontend.Mora, reading string) ([]prosody.FeatureFrame, error) {
	if model == nil || !model.RequiresExternalFeatures() || len(cfg.ProsodyFeatures) > 0 {
		return cfg.ProsodyFeatures, nil
	}
	runtimeText := frontend.ApplyDictionaryForAnalysis(cfg.Text, cfg.Dictionary)
	if strings.TrimSpace(runtimeText) == "" {
		// かなだけの入力では読みを表層テキストとして解析する。
		runtimeText = reading
	}
	runtimeConfig := openjtalk.Config{
		HelperPath: cfg.OpenJTalkPath, DictionaryPath: cfg.OpenJTalkDictionaryPath,
	}
	aligned, alignmentErr := analyzeAndAlignRuntimeFeatures(cfg.Context, morae, runtimeText, runtimeConfig)
	if alignmentErr == nil {
		return aligned, nil
	}
	fallback, fallbackErr := analyzeAndAlignRuntimeFeatures(cfg.Context, morae, reading, runtimeConfig)
	if fallbackErr != nil {
		return nil, fmt.Errorf("align runtime prosody features: %v; Open JTalk fallback: %w", alignmentErr, fallbackErr)
	}
	return fallback, nil
}

func analyzeAndAlignRuntimeFeatures(ctx context.Context, morae []frontend.Mora, text string, cfg openjtalk.Config) ([]prosody.FeatureFrame, error) {
	analysis, err := analyzeOpenJTalkCached(ctx, text, cfg)
	if err != nil {
		return nil, fmt.Errorf("analyze runtime prosody features: %w", err)
	}
	return alignRuntimeProsodyFeatures(morae, analysis)
}

// ApplyRendererはrendererIDを解決してcfgへ反映する。指定パスを同梱資源より優先する。
func ApplyRenderer(cfg *Config, catalog *plugin.Catalog, rendererID, worldlinePath, worldlineBridgePath string) (string, error) {
	if catalog == nil {
		return "", errors.New("renderer catalog is not initialized")
	}
	renderer, found := catalog.Renderer(rendererID)
	if !found {
		return "", errors.New("renderer catalog has no available renderer")
	}
	if !render.IsKnownRenderer(renderer.Backend) {
		return "", fmt.Errorf("renderer plugin %q requires unavailable backend %q", renderer.ID, renderer.Backend)
	}
	ApplyResolvedRenderer(cfg, renderer, worldlinePath, worldlineBridgePath)
	return renderer.ID, nil
}

// ApplyResolvedRendererは、解決済みのレンダラプラグインからcfgのレンダラ依存フィールドを埋める。
func ApplyResolvedRenderer(cfg *Config, renderer plugin.Renderer, worldlinePath, worldlineBridgePath string) {
	cfg.Renderer = renderer.Backend
	cfg.RendererCapabilities = &renderer.Capabilities
	cfg.WorldlinePath = preferExplicit(worldlinePath, renderer.Asset("worldline"))
	cfg.WorldlineBridgePath = preferExplicit(worldlineBridgePath, renderer.Asset("worldline_bridge"))
	cfg.WorldEnginePath = renderer.Asset("world_engine")
	cfg.WorldGPUPath = renderer.Asset("world_gpu")
	cfg.ExternalResamplerPath = renderer.Asset("resampler")
}

func preferExplicit(explicit, manifestValue string) string {
	if explicit != "" {
		return explicit
	}
	return manifestValue
}

func Synthesize(cfg Config) (*Result, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	bank := cfg.Voicebank
	var err error
	if bank == nil {
		bank, err = loadVoicebankCached(cfg.VoicebankPath)
		if err != nil {
			return nil, fmt.Errorf("load voicebank: %w", err)
		}
	}
	requestedAliasPolicy := cfg.AliasPolicy
	if requestedAliasPolicy == "" {
		requestedAliasPolicy = voicebank.AliasPolicyAuto
	}
	applyAliasProfile(bank, &cfg)
	loadedProsody, err := resolveProsodyModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("load prosody model: %w", err)
	}
	reading, err := resolveReading(cfg)
	if err != nil {
		return nil, err
	}
	morae, err := frontend.ParseKana(reading)
	if err != nil {
		return nil, fmt.Errorf("parse reading: %w", err)
	}
	prosodyFeatures, err := resolveProsodyFeatures(cfg, loadedProsody, morae, reading)
	if err != nil {
		return nil, err
	}
	var joinModel *connection.LearnedModel
	joinCostMode := "handcrafted"
	joinModelVersion := 0
	if cfg.JoinModelPath != "" {
		joinModel, err = connection.LoadLearnedModel(cfg.JoinModelPath)
		if err != nil {
			return nil, fmt.Errorf("load join model: %w", err)
		}
		joinCostMode = "learned"
		joinModelVersion = joinModel.Version
		if cfg.JoinScoreScale > 0 {
			joinModel.ScoreScale = cfg.JoinScoreScale
		}
	}
	if cfg.SelectionMode == voicebank.SelectionTargetOnly {
		joinCostMode = "none"
	}
	selections, err := bank.ResolveWithConfig(morae, voicebank.ResolveConfig{
		Tone: cfg.Tone, Color: cfg.Color, Mode: cfg.SelectionMode, AliasPolicy: cfg.AliasPolicy,
		AcousticMode: cfg.AcousticMode, JoinModel: joinModel,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve voicebank units: %w", err)
	}
	var predictions []prosody.Prediction
	if loadedProsody != nil {
		if loadedProsody.RequiresExternalFeatures() && len(prosodyFeatures) != len(morae) {
			return nil, fmt.Errorf("prosody model %d/%s requires %d mora-level accent feature frames, got %d", loadedProsody.Version, loadedProsody.Mode, len(morae), len(prosodyFeatures))
		}
		predictions = loadedProsody.PredictWithFeatures(morae, prosodyFeatures)
		if cfg.ProsodyPitchOnly {
			for i := range predictions {
				predictions[i].DurationMS = 0
				predictions[i].DurationFactor = 1
				predictions[i].EnergyFactor = 1
			}
		}
	}
	if len(cfg.PitchFactors) > 0 {
		if len(cfg.PitchFactors) != len(morae) {
			return nil, fmt.Errorf("pitch factors: got %d values for %d morae", len(cfg.PitchFactors), len(morae))
		}
		if len(predictions) == 0 {
			predictions = make([]prosody.Prediction, len(morae))
			for i := range predictions {
				predictions[i].DurationFactor = 1
				predictions[i].EnergyFactor = 1
			}
		}
		for i, factor := range cfg.PitchFactors {
			if factor <= 0 {
				return nil, fmt.Errorf("pitch factors: value %d is %.4f, want positive", i, factor)
			}
			predictions[i].PitchFactor = factor
		}
	}
	synthesisPlan, err := plan.Build(bank, reading, morae, selections, plan.Config{
		MoraDurationMS:   cfg.MoraDurationMS,
		PauseDurationMS:  cfg.PauseDurationMS,
		MoraDurationsMS:  cfg.MoraDurationsMS,
		Predictions:      predictions,
		SelectionMode:    cfg.SelectionMode,
		AliasPolicy:      cfg.AliasPolicy,
		Tone:             cfg.Tone,
		Color:            cfg.Color,
		AcousticMode:     cfg.AcousticMode,
		JoinCostMode:     joinCostMode,
		JoinModelVersion: joinModelVersion,
		JoinScoreScale:   joinModelScoreScale(joinModel),
	})
	if err != nil {
		return nil, fmt.Errorf("build synthesis plan: %w", err)
	}
	synthesisPlan.Text = cfg.Text
	synthesisPlan.RequestedAliasPolicy = string(requestedAliasPolicy)
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	pitchCurve := cfg.PitchCurve
	applyPitch := applyPitchEnabled(cfg)
	if pitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
		timings := moraTimings(morae, synthesisPlan)
		question := strings.ContainsAny(cfg.Text, "?？")
		if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, timings, synthesisPlan.DurationMS+cfg.ReleaseMS, question); contour != nil {
			pitchCurve = &render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}
			pitchCurve = scaleAutomaticPitchCurve(pitchCurve, cfg.IntonationStrength)
		}
	}
	automaticPitchCurve := pitchCurve
	manualPitch := cfg.ManualPitch
	if manualPitch == nil && cfg.ManualPitchPath != "" {
		manualPitch, err = prosody.LoadManualPitch(cfg.ManualPitchPath)
		if err != nil {
			return nil, fmt.Errorf("load manual pitch: %w", err)
		}
	}
	if manualPitch != nil {
		if err := manualPitch.Validate(); err != nil {
			return nil, fmt.Errorf("validate manual pitch: %w", err)
		}
		if manualPitch.Reading != "" && manualPitch.Reading != reading {
			return nil, fmt.Errorf("manual pitch reading does not match synthesis reading")
		}
		timings := moraTimings(morae, synthesisPlan)
		manualContour, curveErr := manualPitch.Curve(morae, timings, synthesisPlan.DurationMS+cfg.ReleaseMS)
		if curveErr != nil {
			return nil, fmt.Errorf("build manual pitch curve: %w", curveErr)
		}
		pitchCurve = mergeManualPitchCurve(pitchCurve, manualContour, manualPitch.Mode)
		pitchCurve = render.ConstrainPitchCurve(pitchCurve, 20, 8)
	}
	intonationStrength := effectiveIntonationStrength(cfg)
	pcm, err := render.Render(synthesisPlan, render.Config{
		Context:                 cfg.Context,
		ReleaseMS:               cfg.ReleaseMS,
		ReleaseSet:              cfg.ReleaseSet,
		LeadingPreutteranceMS:   cfg.LeadingPreutteranceMS,
		IntonationStrength:      intonationStrength,
		ApplyPitch:              applyPitch,
		Backend:                 cfg.Renderer,
		WorldlinePath:           cfg.WorldlinePath,
		WorldlineBridgePath:     cfg.WorldlineBridgePath,
		WorldEnginePath:         cfg.WorldEnginePath,
		WorldGPUPath:            cfg.WorldGPUPath,
		ExternalResamplerPath:   cfg.ExternalResamplerPath,
		BoundaryBridgeMS:        cfg.BoundaryBridgeMS,
		BoundaryBridgeThreshold: cfg.BoundaryBridgeThreshold,
		CVVCTiming:              cfg.CVVCTiming,
		CVVCTransitionGain:      cfg.CVVCTransitionGain,
		CVVCPreBoundaryFade:     cfg.CVVCPreBoundaryFade,
		PitchCurve:              pitchCurve,
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	timings := moraTimings(morae, synthesisPlan)
	moraDurations := make([]float64, len(timings))
	moraPositions := make([]float64, len(timings))
	pitchPoints := make([]float64, len(timings))
	for index, timing := range timings {
		moraDurations[index] = timing.DurationMS
		moraPositions[index] = timing.StartMS + timing.DurationMS/2
		if automaticPitchCurve != nil && !morae[index].Pause {
			pitchPoints[index] = pitchCurveCentsAt(automaticPitchCurve, moraPositions[index])
		}
	}
	return &Result{
		Voicebank:       bank,
		Plan:            synthesisPlan,
		Audio:           pcm,
		MoraDurationsMS: moraDurations,
		MoraPositionsMS: moraPositions,
		PitchPoints:     pitchPoints,
	}, nil
}

func applyAliasProfile(bank *voicebank.Bank, cfg *Config) {
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = voicebank.AliasPolicyAuto
	}
	switch policy {
	case voicebank.AliasPolicyAuto:
		if bank != nil && bank.RecommendCVVCEnhanced() {
			applyCVVCEnhancedProfile(cfg)
		} else {
			applyLegacyAliasProfile(cfg)
		}
	case voicebank.AliasPolicyLegacy:
		applyLegacyAliasProfile(cfg)
	case voicebank.AliasPolicyEnhanced:
		applyCVVCEnhancedProfile(cfg)
	}
}

func applyLegacyAliasProfile(cfg *Config) {
	cfg.AliasPolicy = voicebank.AliasPolicyAuto
	cfg.CVVCTiming = render.CVVCTimingLegacy
	cfg.CVVCTransitionGain = 1
	cfg.CVVCPreBoundaryFade = false
}

func applyCVVCEnhancedProfile(cfg *Config) {
	cfg.AliasPolicy = voicebank.AliasPolicyCVVCPrefer
	cfg.CVVCTiming = render.CVVCTimingSequential
	cfg.CVVCTransitionGain = 0.35
	cfg.CVVCPreBoundaryFade = false
}

// PredictProsodyは音声合成せずに選択されたプロソディモデルを評価する。手動のモーラ長を尊重するため、プレビューはGUIで編集中の値に従う。
func PredictProsody(cfg Config) (*ProsodyPreview, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.MoraDurationMS <= 0 {
		cfg.MoraDurationMS = 120
	}
	if cfg.PauseDurationMS < 0 {
		cfg.PauseDurationMS = 180
	}
	if cfg.ReleaseMS <= 0 && !cfg.ReleaseSet {
		cfg.ReleaseMS = 20
	}

	loadedProsody, err := resolveProsodyModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("load prosody model: %w", err)
	}
	reading, err := resolveReading(cfg)
	if err != nil {
		return nil, err
	}
	morae, err := frontend.ParseKana(reading)
	if err != nil {
		return nil, fmt.Errorf("parse reading: %w", err)
	}

	prosodyFeatures, err := resolveProsodyFeatures(cfg, loadedProsody, morae, reading)
	if err != nil {
		return nil, err
	}

	var predictions []prosody.Prediction
	if loadedProsody != nil {
		if loadedProsody.RequiresExternalFeatures() && len(prosodyFeatures) != len(morae) {
			return nil, fmt.Errorf("prosody model %d/%s requires %d mora-level accent feature frames, got %d", loadedProsody.Version, loadedProsody.Mode, len(morae), len(prosodyFeatures))
		}
		predictions = loadedProsody.PredictWithFeatures(morae, prosodyFeatures)
	}

	timings := make([]prosody.MoraTiming, len(morae))
	result := &ProsodyPreview{
		Reading: reading, Morae: append([]frontend.Mora(nil), morae...),
		Features:        append([]prosody.FeatureFrame(nil), prosodyFeatures...),
		MoraDurationsMS: make([]float64, len(morae)),
		MoraPositionsMS: make([]float64, len(morae)),
		PitchPoints:     make([]float64, len(morae)),
	}
	cursor := 0.0
	for index, mora := range morae {
		duration, manuallySet := previewConfiguredMoraDuration(index, cfg)
		if !manuallySet {
			if mora.Pause {
				duration = cfg.PauseDurationMS
			} else {
				duration = previewDurationFor(mora, cfg.MoraDurationMS)
				if index < len(predictions) && predictions[index].DurationFactor > 0 {
					duration *= predictions[index].DurationFactor
				}
			}
		}
		duration = math.Max(0, duration)
		timings[index] = prosody.MoraTiming{StartMS: cursor, DurationMS: duration}
		result.MoraDurationsMS[index] = duration
		result.MoraPositionsMS[index] = cursor + duration/2
		cursor += duration
	}

	if shouldPredictFrameContour(cfg, loadedProsody) {
		question := strings.ContainsAny(cfg.Text, "?？")
		if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, timings, cursor+cfg.ReleaseMS, question); contour != nil {
			curve := scaleAutomaticPitchCurve(&render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}, cfg.IntonationStrength)
			result.FramePitchCurve = curve
			for index, mora := range morae {
				if curve != nil && !mora.Pause {
					result.PitchPoints[index] = pitchCurveCentsAt(curve, result.MoraPositionsMS[index])
				}
			}
		}
	}
	return result, nil
}

func synthesisContextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("synthesis canceled: %w", ctx.Err())
	default:
		return nil
	}
}

func previewDurationFor(mora frontend.Mora, base float64) float64 {
	switch mora.Vowel {
	case "cl":
		return base * 0.65
	case "n":
		return base * 0.9
	}
	if mora.Text == "ー" {
		return base * 1.2
	}
	return base
}

func previewConfiguredMoraDuration(position int, cfg Config) (float64, bool) {
	if position < 0 || position >= len(cfg.MoraDurationsMS) {
		return 0, false
	}
	duration := cfg.MoraDurationsMS[position]
	if duration <= 0 {
		return 0, false
	}
	return duration, true
}

func validateConfig(cfg Config) error {
	finite := map[string]float64{
		"mora_duration_ms":          cfg.MoraDurationMS,
		"pause_duration_ms":         cfg.PauseDurationMS,
		"release_ms":                cfg.ReleaseMS,
		"intonation_strength":       cfg.IntonationStrength,
		"boundary_bridge_ms":        cfg.BoundaryBridgeMS,
		"boundary_bridge_threshold": cfg.BoundaryBridgeThreshold,
		"join_score_scale":          cfg.JoinScoreScale,
	}
	for name, value := range finite {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be finite, got %v", name, value)
		}
	}
	for index, factor := range cfg.PitchFactors {
		if math.IsNaN(factor) || math.IsInf(factor, 0) {
			return fmt.Errorf("pitch factors: value %d must be finite, got %v", index, factor)
		}
	}
	for index, duration := range cfg.MoraDurationsMS {
		if math.IsNaN(duration) || math.IsInf(duration, 0) {
			return fmt.Errorf("mora durations: value %d must be finite, got %v", index, duration)
		}
	}
	if cfg.IntonationStrength < 0 || cfg.IntonationStrength > render.MaxIntonationStrength {
		return fmt.Errorf("intonation_strength must be between 0 and %.0f, got %v", render.MaxIntonationStrength, cfg.IntonationStrength)
	}
	if cfg.ReleaseMS < 0 {
		return fmt.Errorf("release_ms must be non-negative, got %v", cfg.ReleaseMS)
	}
	return nil
}

func mergeManualPitchCurve(base *render.PitchCurve, manual *prosody.PitchContour, mode string) *render.PitchCurve {
	if manual == nil || manual.FrameMS <= 0 || len(manual.Cents) == 0 {
		return base
	}
	result := &render.PitchCurve{FrameMS: manual.FrameMS, Cents: make([]float64, len(manual.Cents))}
	for index := range result.Cents {
		manualCents := manual.Cents[index]
		if mode == "replace" {
			result.Cents[index] = manualCents
			continue
		}
		baseCents := 0.0
		if base != nil && len(base.Cents) > 0 {
			baseCents = pitchCurveCentsAt(base, float64(index)*manual.FrameMS)
		}
		result.Cents[index] = baseCents + manualCents
	}
	return result
}

func pitchCurveCentsAt(curve *render.PitchCurve, timeMS float64) float64 {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return 0
	}
	position := math.Max(0, timeMS) / curve.FrameMS
	left := int(math.Floor(position))
	if left >= len(curve.Cents)-1 {
		return curve.Cents[len(curve.Cents)-1]
	}
	progress := position - float64(left)
	return curve.Cents[left]*(1-progress) + curve.Cents[left+1]*progress
}

func alignRuntimeProsodyFeatures(morae []frontend.Mora, analysis *openjtalk.Analysis) ([]prosody.FeatureFrame, error) {
	if analysis == nil {
		return nil, fmt.Errorf("Open JTalk analysis is nil")
	}
	if len(analysis.Morae) != len(analysis.Features) {
		return nil, fmt.Errorf("Open JTalk returned %d morae and %d feature frames", len(analysis.Morae), len(analysis.Features))
	}
	if len(morae) == 0 {
		return nil, nil
	}
	if len(analysis.Morae) == 0 {
		return make([]prosody.FeatureFrame, len(morae)), nil
	}

	indices := alignRuntimeMoraIndices(morae, analysis.Morae)
	aligned := make([]prosody.FeatureFrame, len(morae))
	for index, analyzedIndex := range indices {
		if analyzedIndex >= 0 {
			aligned[index] = cloneFeatureFrame(analysis.Features[analyzedIndex])
			continue
		}
		if morae[index].Pause {
			aligned[index] = prosody.FeatureFrame{}
			continue
		}
		aligned[index] = cloneFeatureFrame(nearestRuntimeFeature(indices, analysis.Features, index))
	}
	return aligned, nil
}

type runtimeMoraAlignmentCell struct {
	cost float64
	op   byte
}

const (
	runtimeAlignmentSkipCost   = 1.1
	runtimeAlignmentChangeCost = 1.8
)

func alignRuntimeMoraIndices(morae []frontend.Mora, analyzed []string) []int {
	rows := len(morae) + 1
	columns := len(analyzed) + 1
	cells := make([][]runtimeMoraAlignmentCell, rows)
	for row := range cells {
		cells[row] = make([]runtimeMoraAlignmentCell, columns)
		for column := range cells[row] {
			cells[row][column].cost = math.Inf(1)
		}
	}
	cells[0][0] = runtimeMoraAlignmentCell{}
	for row := 1; row < rows; row++ {
		cells[row][0] = runtimeMoraAlignmentCell{
			cost: cells[row-1][0].cost + runtimeAlignmentSkipCost,
			op:   'g',
		}
	}
	for column := 1; column < columns; column++ {
		cells[0][column] = runtimeMoraAlignmentCell{
			cost: cells[0][column-1].cost + runtimeAlignmentSkipCost,
			op:   'o',
		}
	}
	for row := 1; row < rows; row++ {
		for column := 1; column < columns; column++ {
			best := runtimeMoraAlignmentCell{
				cost: cells[row-1][column-1].cost + runtimeMoraCost(morae[row-1], analyzed[column-1]),
				op:   'm',
			}
			best = chooseRuntimeAlignment(best, runtimeMoraAlignmentCell{
				cost: cells[row-1][column].cost + runtimeAlignmentSkipCost,
				op:   'g',
			})
			best = chooseRuntimeAlignment(best, runtimeMoraAlignmentCell{
				cost: cells[row][column-1].cost + runtimeAlignmentSkipCost,
				op:   'o',
			})
			cells[row][column] = best
		}
	}

	indices := make([]int, len(morae))
	for index := range indices {
		indices[index] = -1
	}
	row, column := len(morae), len(analyzed)
	for row > 0 || column > 0 {
		if row == 0 {
			column--
			continue
		}
		if column == 0 {
			row--
			continue
		}
		switch cells[row][column].op {
		case 'm':
			indices[row-1] = column - 1
			row--
			column--
		case 'g':
			row--
		case 'o':
			column--
		default:
			row--
			column--
		}
	}
	return indices
}

func chooseRuntimeAlignment(current, candidate runtimeMoraAlignmentCell) runtimeMoraAlignmentCell {
	if candidate.cost < current.cost-1e-9 {
		return candidate
	}
	if math.Abs(candidate.cost-current.cost) <= 1e-9 && runtimeAlignmentPriority(candidate.op) > runtimeAlignmentPriority(current.op) {
		return candidate
	}
	return current
}

func runtimeAlignmentPriority(operation byte) int {
	switch operation {
	case 'm':
		return 3
	case 'o':
		return 2
	case 'g':
		return 1
	default:
		return 0
	}
}

func runtimeMoraCost(mora frontend.Mora, analyzed string) float64 {
	if mora.Pause || analyzed == "" {
		if mora.Pause && analyzed == "" {
			return 0
		}
		return runtimeAlignmentChangeCost + runtimeAlignmentSkipCost
	}
	if mora.Text == analyzed {
		return 0
	}
	if analyzed == "ー" && isRuntimeVowel(mora.Vowel) {
		return 0.25
	}
	analyzedVowel := runtimeAnalyzedMoraVowel(analyzed)
	if analyzedVowel != "" && analyzedVowel == mora.Vowel {
		return 0.6
	}
	return runtimeAlignmentChangeCost
}

func isRuntimeVowel(vowel string) bool {
	switch vowel {
	case "a", "i", "u", "e", "o":
		return true
	default:
		return false
	}
}

func runtimeAnalyzedMoraVowel(analyzed string) string {
	parsed, err := frontend.ParseKana(analyzed)
	if err != nil || len(parsed) != 1 || parsed[0].Pause {
		return ""
	}
	return parsed[0].Vowel
}

func nearestRuntimeFeature(indices []int, features []prosody.FeatureFrame, target int) prosody.FeatureFrame {
	for distance := 1; distance < len(indices)+1; distance++ {
		left := target - distance
		if left >= 0 && indices[left] >= 0 {
			return features[indices[left]]
		}
		right := target + distance
		if right < len(indices) && indices[right] >= 0 {
			return features[indices[right]]
		}
	}
	return prosody.FeatureFrame{}
}

func cloneFeatureFrame(frame prosody.FeatureFrame) prosody.FeatureFrame {
	if len(frame) == 0 {
		return prosody.FeatureFrame{}
	}
	result := make(prosody.FeatureFrame, len(frame))
	for name, value := range frame {
		result[name] = value
	}
	return result
}

func moraTimings(morae []frontend.Mora, synthesisPlan *plan.Plan) []prosody.MoraTiming {
	byPosition := make(map[int]plan.Unit, len(synthesisPlan.Units))
	for _, unit := range synthesisPlan.Units {
		if unit.Role == "transition" {
			continue
		}
		byPosition[unit.Position] = unit
	}
	timings := make([]prosody.MoraTiming, len(morae))
	cursor := 0.0
	for position := 0; position < len(morae); {
		if unit, ok := byPosition[position]; ok {
			cursor = unit.NoteStartMS
			timings[position] = prosody.MoraTiming{StartMS: cursor, DurationMS: unit.DurationMS}
			cursor += unit.DurationMS
			position++
			continue
		}
		nextPosition := position + 1
		for nextPosition < len(morae) {
			if _, ok := byPosition[nextPosition]; ok {
				break
			}
			nextPosition++
		}
		nextStart := synthesisPlan.DurationMS
		if nextPosition < len(morae) {
			nextStart = byPosition[nextPosition].NoteStartMS
		}
		duration := math.Max(0, nextStart-cursor) / float64(nextPosition-position)
		for position < nextPosition {
			timings[position] = prosody.MoraTiming{StartMS: cursor, DurationMS: duration}
			cursor += duration
			position++
		}
	}
	return timings
}

func rendererSupportsFramePitch(renderer string, capabilities *plugin.Capabilities) bool {
	if capabilities != nil {
		return capabilities.FramePitch
	}
	directories, _ := plugin.DefaultDirectories()
	items, _ := plugin.DiscoverRenderers(directories, nil)
	for _, item := range items {
		if item.ID == renderer || item.Backend == renderer {
			return item.Capabilities.FramePitch
		}
	}
	return false
}

func applyPitchEnabled(cfg Config) bool {
	return cfg.ApplyPitch || cfg.ProsodyPitchOnly
}

func shouldPredictFrameContour(cfg Config, model *prosody.Model) bool {
	return applyPitchEnabled(cfg) && model != nil && model.HasFrameContour() &&
		rendererSupportsFramePitch(cfg.Renderer, cfg.RendererCapabilities)
}

func effectiveIntonationStrength(cfg Config) float64 {
	if !applyPitchEnabled(cfg) {
		return 0
	}
	return cfg.IntonationStrength
}

// scaleAutomaticPitchCurveは自動輪郭だけに強度を適用し、手動補正は増幅しない。
func scaleAutomaticPitchCurve(curve *render.PitchCurve, strength float64) *render.PitchCurve {
	if curve == nil || len(curve.Cents) == 0 {
		return curve
	}
	if strength <= 0 {
		return nil
	}
	if strength == 1 {
		return curve
	}
	result := &render.PitchCurve{FrameMS: curve.FrameMS, Cents: make([]float64, len(curve.Cents))}
	for index, cents := range curve.Cents {
		result.Cents[index] = cents * strength
	}
	return result
}

func joinModelScoreScale(model *connection.LearnedModel) float64 {
	if model == nil {
		return 0
	}
	return model.ScoreScale
}
