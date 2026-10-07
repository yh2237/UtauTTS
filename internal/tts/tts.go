package tts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"utautts/internal/audio"
	"utautts/internal/connection"
	"utautts/internal/engine"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

// nilなら計測ログを出さない。
var Trace func(message string)

func traceMark(start *time.Time, label string) {
	if Trace == nil {
		return
	}
	now := time.Now()
	Trace(fmt.Sprintf("%s: %.1fms", label, float64(now.Sub(*start).Microseconds())/1000))
	*start = now
}

type Config struct {
	// nilは無効。
	ContextDuration *bool
	// ContextDurationStrengthは文脈連動の強度。0は既定1.0。
	ContextDurationStrength float64
	// nilは既定で有効。境界音調。
	BoundaryTone *bool
	// BoundaryToneStrengthは境界音調の強度。0は既定1.0。
	BoundaryToneStrength float64
	// nilは既定で有効。伸縮補正。
	StretchAdapt *bool
	// StretchAdaptStrengthは伸縮補正の強度。0は既定1.0。
	StretchAdaptStrength float64
	// nilは既定で有効。
	PauseContext *bool
	// PauseContextStrengthはポーズ長補正の強度。0は既定1.0。
	PauseContextStrength float64
	// nilは既定で有効。
	EnglishWeakForm         *bool
	Context                 context.Context
	Engine                  engine.ResolvedEngine
	VoicebankPath           string
	Voicebank               *voicebank.Bank
	Text                    string
	Reading                 string
	Language                string
	Phonemizer              string
	Dictionary              map[string]string
	Tone                    string
	Color                   string
	MoraDurationMS          float64
	PauseDurationMS         float64
	MoraDurationsMS         []float64
	UnitOverrides           []plan.UnitOverride
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
	BoundaryBridgeMS        float64
	BoundaryBridgeThreshold float64
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	PitchCurve              *render.PitchCurve
	AliasPolicy             voicebank.AliasPolicy
	JoinModelPath           string
	JoinModel               *connection.JoinModel
	ProviderOptions         render.ProviderOptions
}

type Result struct {
	Voicebank       *voicebank.Bank
	Plan            *plan.Plan
	Audio           *audio.PCM
	RenderReport    *render.RenderReport
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
}

// RenderedPlanはレンダラー診断を含む出力用コピーを返す。元の選択計画は変えない。
func (result *Result) RenderedPlan() *plan.Plan {
	if result == nil {
		return nil
	}
	rendered := plan.Clone(result.Plan)
	if rendered != nil && result.RenderReport != nil {
		result.RenderReport.ApplyTo(rendered)
	}
	return rendered
}

type ProsodyPreview struct {
	Reading         string
	Morae           []frontend.Mora
	Features        []prosody.FeatureFrame
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
	// FramePitchCurveは強度適用後の10ms単位のピッチ曲線。
	FramePitchCurve *render.PitchCurve
}

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

func ResolvePronunciation(cfg Config) (string, string, string, []frontend.Mora, error) {
	return resolvePronunciation(cfg)
}

// 読みとモーラだけを解析する。時間やピッチの予測にはPredictProsodyを使う。
func Analyze(cfg Config) (*ProsodyPreview, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	_, _, reading, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	return &ProsodyPreview{
		Reading: reading,
		Morae:   append([]frontend.Mora(nil), morae...),
	}, nil
}

// 明示指定、音源の推定、言語既定の順に選ぶ。言語に合わない推定は使わない。
func resolveLanguagePhonemizer(cfg Config) (string, string, error) {
	if strings.TrimSpace(cfg.Phonemizer) != "" {
		return frontend.ResolveLanguage(cfg.Language, cfg.Phonemizer)
	}
	bank := cfg.Voicebank
	if bank == nil && strings.TrimSpace(cfg.VoicebankPath) != "" {
		// 読込失敗時は推定を諦め、言語既定へ委ねる。
		if loaded, err := loadVoicebankCached(cfg.VoicebankPath); err == nil {
			bank = loaded
		}
	}
	if bank != nil {
		suggestedLanguage, suggestedPhonemizer := bank.SuggestedLanguage()
		if strings.TrimSpace(cfg.Language) == "" {
			return frontend.ResolveLanguage(suggestedLanguage, suggestedPhonemizer)
		}
		if language, phonemizer, err := frontend.ResolveLanguage(cfg.Language, suggestedPhonemizer); err == nil {
			return language, phonemizer, nil
		}
	}
	return frontend.ResolveLanguage(cfg.Language, "")
}

func resolvePronunciation(cfg Config) (string, string, string, []frontend.Mora, error) {
	language, phonemizer, err := resolveLanguagePhonemizer(cfg)
	if err != nil {
		return "", "", "", nil, err
	}
	reading, morae, err := languageProfileFor(language).ParsePronunciation(cfg, phonemizer)
	if err != nil {
		return "", "", "", nil, err
	}
	return language, phonemizer, reading, morae, nil
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

func resolveProsodyModelForLanguage(cfg Config, language string) (*prosody.Model, error) {
	return resolveProsodyModelForProfile(cfg, languageProfileFor(language))
}

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

func ResolveRenderer(catalog *plugin.Catalog, rendererID string) (engine.ResolvedEngine, error) {
	return ResolveRendererWithOptions(catalog, rendererID, engine.ResolveOptions{})
}

func ResolveRendererWithOptions(catalog *plugin.Catalog, rendererID string, options engine.ResolveOptions) (engine.ResolvedEngine, error) {
	if catalog == nil {
		return engine.ResolvedEngine{}, errors.New("renderer catalog is not initialized")
	}
	resolver := engine.NewResolver(engine.BuiltinRegistry())
	return resolver.ResolveWithOptions(engine.DefinitionsFromCatalog(catalog), rendererID, options)
}

func ApplyRenderer(cfg *Config, catalog *plugin.Catalog, rendererID, worldlineBridgePath string) (string, error) {
	resolved, err := ResolveRendererWithOptions(catalog, rendererID, engine.ResolveOptions{
		ResourceOverrides: map[engine.ResourceKey]string{
			engine.ResourceWorldlineBridge: worldlineBridgePath,
		},
	})
	if err != nil {
		return "", err
	}
	ApplyResolvedEngine(cfg, resolved)
	return string(resolved.PublicID()), nil
}

func ApplyResolvedEngine(cfg *Config, resolved engine.ResolvedEngine) {
	capabilities := resolved.Definition.Capabilities
	cfg.Engine = resolved
	cfg.Renderer = string(resolved.Provider.ID)
	cfg.RendererCapabilities = &capabilities
}

func Synthesize(cfg Config) (*Result, error) {
	return SynthesizeWithOptions(cfg, render.ProviderOptions{})
}

func SynthesizeWithOptions(cfg Config, providerOptions render.ProviderOptions) (*Result, error) {
	cfg.ProviderOptions = providerOptions
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	start := time.Now()
	providerID := engine.ProviderID(cfg.Renderer)
	if cfg.Engine.Provider.ID != "" {
		providerID = cfg.Engine.Provider.ID
	}
	if synthesizer, found := neuralSynthesizerForProvider(providerID); found {
		return synthesizer.Synthesize(cfg)
	}
	bank := cfg.Voicebank
	var err error
	if bank == nil {
		bank, err = loadVoicebankCached(cfg.VoicebankPath)
		if err != nil {
			return nil, fmt.Errorf("load voicebank: %w", err)
		}
	}
	cfg.Voicebank = bank
	joinModel, err := resolveJoinModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("load join model: %w", err)
	}
	requestedAliasPolicy := cfg.AliasPolicy
	if requestedAliasPolicy == "" {
		requestedAliasPolicy = voicebank.AliasPolicyAuto
	}
	applyAliasProfile(bank, &cfg)
	if len(bank.ARPAsing) > 0 {
		dictionary := make(map[string]string, len(bank.ARPAsing)+len(cfg.Dictionary))
		for key, value := range bank.ARPAsing {
			dictionary[key] = value
		}
		for key, value := range cfg.Dictionary {
			dictionary[key] = value
		}
		cfg.Dictionary = dictionary
	}
	traceMark(&start, "setup")
	language, phonemizer, reading, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	profile := languageProfileFor(language)
	profile.ApplySpeechProfile(&cfg)
	traceMark(&start, "phonemize")
	loadedProsody, err := resolveProsodyModelForProfile(cfg, profile)
	if err != nil {
		return nil, fmt.Errorf("load prosody model: %w", err)
	}
	traceMark(&start, "prosodyModel")
	prosodyFeatures, predictions, err := resolveProsodyComputation(cfg, profile, loadedProsody, morae, reading)
	if err != nil {
		return nil, err
	}
	traceMark(&start, "prosody")
	selections, err := bank.ResolveWithConfig(morae, voicebank.ResolveConfig{
		Tone: cfg.Tone, Color: cfg.Color, AliasPolicy: cfg.AliasPolicy, JoinModel: joinModel,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve voicebank units: %w", err)
	}
	traceMark(&start, "select")
	synthesisPlan, stretchAdapt, err := buildSynthesisPlan(cfg, profile, bank, reading, language, phonemizer, morae, selections, predictions, requestedAliasPolicy, joinModel)
	if err != nil {
		return nil, err
	}
	traceMark(&start, "plan")
	pitch, err := resolveSynthesisPitch(cfg, profile, loadedProsody, morae, prosodyFeatures, reading, language, synthesisPlan)
	if err != nil {
		return nil, err
	}
	traceMark(&start, "pitch")
	rendered, err := render.RenderWithReport(synthesisPlan, render.Config{
		Context:                 cfg.Context,
		Engine:                  cfg.Engine,
		ReleaseMS:               cfg.ReleaseMS,
		ReleaseSet:              cfg.ReleaseSet,
		LeadingPreutteranceMS:   cfg.LeadingPreutteranceMS,
		IntonationStrength:      pitch.RendererStrength,
		ApplyPitch:              pitch.Apply,
		Backend:                 cfg.Renderer,
		ProviderOptions:         providerOptions,
		BoundaryBridgeMS:        cfg.BoundaryBridgeMS,
		BoundaryBridgeThreshold: cfg.BoundaryBridgeThreshold,
		CVVCTiming:              cfg.CVVCTiming,
		CVVCTransitionGain:      cfg.CVVCTransitionGain,
		CVVCPreBoundaryFade:     cfg.CVVCPreBoundaryFade,
		PitchCurve:              pitch.Curve,
		StretchAdapt:            stretchAdapt,
		StretchAdaptStrength:    stretchAdaptStrength(cfg),
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	traceMark(&start, "render")
	pcm := rendered.Audio
	timings := moraTimings(morae, synthesisPlan)
	moraDurations := make([]float64, len(timings))
	moraPositions := make([]float64, len(timings))
	pitchPoints := make([]float64, len(timings))
	for index, timing := range timings {
		moraDurations[index] = timing.DurationMS
		moraPositions[index] = timing.StartMS + timing.DurationMS/2
		if pitch.Automatic != nil && !morae[index].Pause {
			pitchPoints[index] = pitchCurveCentsAt(pitch.Automatic, moraPositions[index])
		}
	}
	return &Result{
		Voicebank:       bank,
		Plan:            synthesisPlan,
		Audio:           pcm,
		RenderReport:    &rendered.Report,
		MoraDurationsMS: moraDurations,
		MoraPositionsMS: moraPositions,
		PitchPoints:     pitchPoints,
	}, nil
}

func resolveJoinModel(cfg Config) (*connection.JoinModel, error) {
	if cfg.JoinModel != nil {
		if err := cfg.JoinModel.Validate(); err != nil {
			return nil, err
		}
		return cfg.JoinModel, nil
	}
	if strings.TrimSpace(cfg.JoinModelPath) == "" {
		return nil, nil
	}
	return connection.LoadJoinModel(cfg.JoinModelPath)
}

func applyAliasProfile(bank *voicebank.Bank, cfg *Config) {
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = voicebank.AliasPolicyAuto
	}
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = render.CVVCTimingSequential
	}
	switch policy {
	case voicebank.AliasPolicyAuto:
		if bank != nil && bank.RecommendCVVCEnhanced() {
			applyCVVCEnhancedProfile(cfg)
		}
	case voicebank.AliasPolicyEnhanced:
		applyCVVCEnhancedProfile(cfg)
	}
}

func applyCVVCEnhancedProfile(cfg *Config) {
	cfg.AliasPolicy = voicebank.AliasPolicyCVVCPrefer
	cfg.CVVCTiming = render.CVVCTimingSequential
	cfg.CVVCTransitionGain = 0.35
	cfg.CVVCPreBoundaryFade = false
}

func applyLanguageSpeechProfile(language string, cfg *Config) {
	languageProfileFor(language).ApplySpeechProfile(cfg)
}

// 音声を合成せず韻律を予測する。プレビューでも手動のモーラ長を尊重する。
func PredictProsody(cfg Config) (*ProsodyPreview, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.MoraDurationMS <= 0 {
		cfg.MoraDurationMS = plan.DefaultMoraDurationMS
	}
	if cfg.PauseDurationMS <= 0 {
		cfg.PauseDurationMS = plan.DefaultPauseDurationMS
	}
	if cfg.ReleaseMS <= 0 && !cfg.ReleaseSet {
		cfg.ReleaseMS = render.DefaultReleaseMS
	}

	language, _, reading, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	profile := languageProfileFor(language)
	loadedProsody, err := resolveProsodyModelForProfile(cfg, profile)
	if err != nil {
		return nil, fmt.Errorf("load prosody model: %w", err)
	}

	prosodyFeatures, predictions, err := resolveProsodyComputation(cfg, profile, loadedProsody, morae, reading)
	if err != nil {
		return nil, err
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
		duration, manuallySet := plan.ConfiguredMoraDuration(index, cfg.MoraDurationsMS)
		if !manuallySet {
			if mora.Pause {
				duration = cfg.PauseDurationMS
			} else {
				duration = plan.DurationFor(mora, cfg.MoraDurationMS)
				if index < len(predictions) && predictions[index].DurationFactor > 0 {
					duration *= predictions[index].DurationFactor
				}
				if index < len(predictions) && predictions[index].DurationMS > 0 && (language == frontend.LanguageEnglish || language == frontend.LanguageChinese) {
					duration = predictions[index].DurationMS
				}
			}
		}
		duration = math.Max(0, duration)
		timings[index] = prosody.MoraTiming{StartMS: cursor, DurationMS: duration}
		result.MoraDurationsMS[index] = duration
		result.MoraPositionsMS[index] = cursor + duration/2
		cursor += duration
	}

	totalDurationMS := cursor + cfg.ReleaseMS
	if curve, _ := profile.AutomaticPitchCurve(cfg, loadedProsody, morae, timings, totalDurationMS); curve != nil {
		result.FramePitchCurve = curve
	}
	if result.FramePitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
		question := finalPhraseIsQuestion(cfg.Text)
		if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, timings, totalDurationMS, question); contour != nil {
			curve := scaleAutomaticPitchCurve(&render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}, cfg.IntonationStrength)
			result.FramePitchCurve = curve
		}
	}
	if result.FramePitchCurve != nil {
		for index, mora := range morae {
			if !mora.Pause {
				result.PitchPoints[index] = pitchCurveCentsAt(result.FramePitchCurve, result.MoraPositionsMS[index])
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

func moraTimings(morae []frontend.Mora, synthesisPlan *plan.Plan) []prosody.MoraTiming {
	byPosition := make(map[int]plan.Unit, len(synthesisPlan.Units))
	for _, unit := range synthesisPlan.Units {
		if unit.Role == "transition" || unit.Role == "ending" {
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
