// synthパッケージはGUIとHTTPサーバで共有する合成処理を提供する。
package synth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"utautts/internal/engine"
	"utautts/internal/jsut"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

// ErrUnavailableは音源・モデル・レンダラープラグインの解決失敗を表す。
var ErrUnavailable = errors.New("unavailable")

// Requestは合成とプレビューで共有する入力。
type Request struct {
	SpeechTiming            bool                         `json:"speech_timing"`
	Text                    string                       `json:"text"`
	Reading                 string                       `json:"reading"`
	Kana                    string                       `json:"kana"`
	Language                string                       `json:"language"`
	Phonemizer              string                       `json:"phonemizer"`
	VoicebankID             string                       `json:"voicebank_id"`
	VoicebankPath           string                       `json:"-"`
	Tone                    string                       `json:"tone"`
	Color                   string                       `json:"color"`
	ModelID                 string                       `json:"model_id"`
	Renderer                string                       `json:"renderer"`
	Resampler               string                       `json:"resampler"`
	Wavtool                 string                       `json:"wavtool"`
	AliasPolicy             voicebank.AliasPolicy        `json:"alias_policy"`
	Dictionary              []DictionaryEntry            `json:"dictionary"`
	MoraDurationMS          float64                      `json:"mora_duration_ms"`
	PauseDurationMS         float64                      `json:"pause_duration_ms"`
	LeadingPreutteranceMS   float64                      `json:"leading_preutterance_ms"`
	MoraDurationsMS         []float64                    `json:"mora_durations_ms"`
	UnitOverrides           []plan.UnitOverride          `json:"unit_overrides"`
	ReleaseMS               float64                      `json:"release_ms"`
	ReleaseSet              bool                         `json:"release_set"`
	ManualPitchPath         string                       `json:"-"`
	ManualPitch             *prosody.ManualPitchFile     `json:"manual_pitch"`
	ProsodyFeatures         []prosody.FeatureFrame       `json:"-"`
	ProsodyPitchOnly        bool                         `json:"-"`
	PitchFactors            []float64                    `json:"-"`
	IntonationStrength      float64                      `json:"intonation_strength"`
	ApplyPitch              bool                         `json:"apply_pitch"`
	BoundaryBridgeMS        float64                      `json:"-"`
	BoundaryBridgeThreshold float64                      `json:"-"`
	CVVCTiming              string                       `json:"-"`
	CVVCTransitionGain      float64                      `json:"-"`
	CVVCPreBoundaryFade     bool                         `json:"-"`
	JoinModelPath           string                       `json:"-"`
	TargetPriorPath         string                       `json:"-"`
	TargetPriorStrength     float64                      `json:"-"`
	TargetPriorMinContext   int                          `json:"-"`
	ResamplerExpressions    []render.ResamplerExpression `json:"resampler_expressions"`
}

// Normalized accepts the historical kana field while keeping the domain
// representation on Reading.
func (request Request) Normalized() Request {
	if request.Reading == "" {
		request.Reading = request.Kana
	}
	return request
}

// ReadingOrKana returns the effective reading without allocating a copy.
func (request Request) ReadingOrKana() string {
	if request.Reading != "" {
		return request.Reading
	}
	return request.Kana
}

// ResolvedRequest is the shared, fully-resolved input passed to synthesis.
// CLI uses Config for USTX export while GUI and HTTP normally call Synthesize.
type ResolvedRequest struct {
	Config          tts.Config
	RendererID      string
	ProviderOptions render.ProviderOptions
}

// DictionaryEntryは表記と読みの対応。
type DictionaryEntry struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

// DictionaryMapは空の項目を除いて合成エンジン用の辞書へ変換する。
func DictionaryMap(entries []DictionaryEntry) map[string]string {
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Surface == "" || entry.Reading == "" {
			continue
		}
		result[entry.Surface] = entry.Reading
	}
	return result
}

// VoicebankResolverは音源IDをルートパスへ解決する。空なら既定音源を選ぶ。
type VoicebankResolver interface {
	Resolve(id string) (path string, ok bool)
}

// Serviceは入力を解決してttsパッケージへ渡す。
type Service struct {
	catalog             *plugin.Catalog
	renderer            string
	worldlineBridgePath string
	openJTalkPath       string
	openJTalkDictionary string
	voicebanks          VoicebankResolver
}

func NewService(catalog *plugin.Catalog, renderer, worldlineBridgePath, openJTalkPath, openJTalkDictionary string, voicebanks VoicebankResolver) *Service {
	return &Service{
		catalog: catalog, renderer: renderer,
		worldlineBridgePath: worldlineBridgePath,
		openJTalkPath:       openJTalkPath, openJTalkDictionary: openJTalkDictionary,
		voicebanks: voicebanks,
	}
}

// Synthesizeはリクエストを解決し、音声・LAB・使用Rendererをまとめて返す。
func (s *Service) Synthesize(request Request) (*Result, error) {
	return s.SynthesizeContext(context.Background(), request)
}

func (s *Service) SynthesizeContext(ctx context.Context, request Request) (*Result, error) {
	resolved, err := s.ResolveSynthesis(request)
	if err != nil {
		return nil, err
	}
	resolved.Config.Context = ctx
	return SynthesizeResolved(resolved)
}

// ResolveSynthesis resolves voicebank, renderer, model, and provider options
// once for every host. It is public for exports that need the resolved config.
func (s *Service) ResolveSynthesis(request Request) (ResolvedRequest, error) {
	cfg, rendererID, providerOptions, err := s.config(request, true)
	if err != nil {
		return ResolvedRequest{}, err
	}
	return ResolvedRequest{Config: cfg, RendererID: rendererID, ProviderOptions: providerOptions}, nil
}

// SynthesizeResolved synthesizes a config created by ResolveSynthesis.
func SynthesizeResolved(resolved ResolvedRequest) (*Result, error) {
	return SynthesizeConfigWithOptions(resolved.Config, resolved.RendererID, resolved.ProviderOptions)
}

// SynthesizeConfigは解決済み設定から共通の合成結果を作る。
func SynthesizeConfig(cfg tts.Config, rendererID string) (*Result, error) {
	return SynthesizeConfigWithOptions(cfg, rendererID, render.ProviderOptions{})
}

// SynthesizeConfigWithOptions runs a resolved config with provider-owned
// options kept outside tts.Config.
func SynthesizeConfigWithOptions(cfg tts.Config, rendererID string, providerOptions render.ProviderOptions) (*Result, error) {
	result, err := tts.SynthesizeWithOptions(cfg, providerOptions)
	if err != nil {
		return nil, err
	}
	return NewResult(result, rendererID)
}

// PredictProsodyは音声や音源を読み込まずにプロソディを返す。
func (s *Service) PredictProsody(request Request) (*tts.ProsodyPreview, string, error) {
	return s.PredictProsodyContext(context.Background(), request)
}

func (s *Service) PredictProsodyContext(ctx context.Context, request Request) (*tts.ProsodyPreview, string, error) {
	cfg, rendererID, _, err := s.config(request, false)
	if err != nil {
		return nil, "", err
	}
	cfg.Context = ctx
	preview, err := tts.PredictProsody(cfg)
	if err != nil {
		return nil, "", err
	}
	return preview, rendererID, nil
}

// AnalyzeContext resolves only the reading and morae required to initialize an
// editor. Prediction is intentionally deferred to PredictProsodyContext.
func (s *Service) AnalyzeContext(ctx context.Context, request Request) (*tts.ProsodyPreview, error) {
	request = request.Normalized()
	dictionary := DictionaryMap(request.Dictionary)
	if request.Language == "en" && request.VoicebankID != "" && s.voicebanks != nil {
		if path, ok := s.voicebanks.Resolve(request.VoicebankID); ok {
			arpasing, _, err := voicebank.LoadARPAsingDictionary(path)
			if err != nil {
				return nil, err
			}
			for surface, reading := range arpasing {
				if dictionary[surface] == "" {
					dictionary[surface] = reading
				}
			}
		}
	}
	return tts.Analyze(tts.Config{
		Context: ctx, Text: request.Text, Reading: request.Reading,
		Language: request.Language, Phonemizer: request.Phonemizer,
		Dictionary: dictionary, OpenJTalkPath: s.openJTalkPath,
		OpenJTalkDictionaryPath: s.openJTalkDictionary,
	})
}

func (s *Service) config(request Request, requireVoicebank bool) (tts.Config, string, render.ProviderOptions, error) {
	request = request.Normalized()
	modelPath, err := s.ResolveModel(request.ModelID)
	if err != nil {
		return tts.Config{}, "", render.ProviderOptions{}, err
	}
	reading := request.Reading
	if reading == "" {
		reading = request.Kana
	}
	cfg := tts.Config{
		SpeechTiming:            request.SpeechTiming,
		Text:                    request.Text,
		Reading:                 reading,
		Language:                request.Language,
		Phonemizer:              request.Phonemizer,
		Dictionary:              DictionaryMap(request.Dictionary),
		Tone:                    request.Tone,
		Color:                   request.Color,
		AliasPolicy:             request.AliasPolicy,
		MoraDurationMS:          request.MoraDurationMS,
		PauseDurationMS:         request.PauseDurationMS,
		LeadingPreutteranceMS:   request.LeadingPreutteranceMS,
		MoraDurationsMS:         request.MoraDurationsMS,
		UnitOverrides:           append([]plan.UnitOverride(nil), request.UnitOverrides...),
		ReleaseMS:               request.ReleaseMS,
		ReleaseSet:              request.ReleaseSet,
		ProsodyModelPath:        modelPath,
		ManualPitchPath:         request.ManualPitchPath,
		ManualPitch:             request.ManualPitch,
		ProsodyFeatures:         append([]prosody.FeatureFrame(nil), request.ProsodyFeatures...),
		ProsodyPitchOnly:        request.ProsodyPitchOnly,
		IntonationStrength:      request.IntonationStrength,
		PitchFactors:            append([]float64(nil), request.PitchFactors...),
		ApplyPitch:              request.ApplyPitch,
		OpenJTalkPath:           s.openJTalkPath,
		OpenJTalkDictionaryPath: s.openJTalkDictionary,
		BoundaryBridgeMS:        request.BoundaryBridgeMS,
		BoundaryBridgeThreshold: request.BoundaryBridgeThreshold,
		CVVCTiming:              request.CVVCTiming,
		CVVCTransitionGain:      request.CVVCTransitionGain,
		CVVCPreBoundaryFade:     request.CVVCPreBoundaryFade,
		JoinModelPath:           request.JoinModelPath,
		TargetPriorPath:         request.TargetPriorPath,
		TargetPriorStrength:     request.TargetPriorStrength,
		TargetPriorMinContext:   request.TargetPriorMinContext,
	}
	providerOptions := render.ProviderOptions{Classic: render.ClassicOptions{
		ResamplerExpressions: append([]render.ResamplerExpression(nil), request.ResamplerExpressions...),
	}}
	if requireVoicebank {
		voicebankPath := request.VoicebankPath
		if voicebankPath == "" {
			if s.voicebanks == nil {
				return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: voicebank resolver is not configured", ErrUnavailable)
			}
			var ok bool
			voicebankPath, ok = s.voicebanks.Resolve(request.VoicebankID)
			if !ok {
				return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: voicebank not found", ErrUnavailable)
			}
		}
		cfg.VoicebankPath = voicebankPath
	}
	resolvedEngine, err := s.ResolveRenderer(request.Renderer)
	if err != nil {
		return tts.Config{}, "", render.ProviderOptions{}, err
	}
	if requireVoicebank {
		if availabilityErr := resolvedEngine.RequireAvailable(); availabilityErr != nil {
			return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: %v", ErrUnavailable, availabilityErr)
		}
	}
	tts.ApplyResolvedEngine(&cfg, resolvedEngine)
	worldlineOptions, worldlineErr := DefaultWorldlineProviderOptions(resolvedEngine)
	if worldlineErr != nil {
		return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: %v", ErrUnavailable, worldlineErr)
	}
	providerOptions.Worldline = worldlineOptions
	// Classic UTAUは公開Renderer IDではなく解決済みproviderで判定する。
	if requireVoicebank && resolvedEngine.Provider.ID == "utau-external-resampler" {
		tools, toolsErr := s.ResolveClassicTools(request.Resampler, request.Wavtool)
		if toolsErr != nil {
			return tts.Config{}, "", render.ProviderOptions{}, toolsErr
		}
		providerOptions.Classic.ResamplerPath = tools.Resampler.Path
		providerOptions.Classic.WavtoolPath = tools.Wavtool.Path
	}
	return cfg, string(resolvedEngine.PublicID()), providerOptions, nil
}

// DefaultWorldlineProviderOptionsは同梱の遷移モデルを各入口で共通に解決する。
func DefaultWorldlineProviderOptions(resolved engine.ResolvedEngine) (render.WorldlineProviderOptions, error) {
	if resolved.Provider.ID != "utautts-world-phrase" {
		return render.WorldlineProviderOptions{}, nil
	}
	modelPath := resolved.Resource(engine.ResourceWorldTransitionModel)
	if modelPath == "" {
		return render.WorldlineProviderOptions{}, nil
	}
	info, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		return render.WorldlineProviderOptions{}, nil
	}
	if err != nil {
		return render.WorldlineProviderOptions{}, fmt.Errorf("stat WORLD transition model: %w", err)
	}
	if info.IsDir() {
		return render.WorldlineProviderOptions{}, fmt.Errorf("WORLD transition model must be a file")
	}
	if _, err := jsut.LoadTransitionTCN(modelPath); err != nil {
		return render.WorldlineProviderOptions{}, fmt.Errorf("load WORLD transition model: %w", err)
	}
	return render.WorldlineProviderOptions{TransitionModelPath: modelPath, TransitionStrength: .20}, nil
}

// ResolveRenderer resolves the user-facing Renderer ID to its provider and
// manifest resources. GUI, HTTP, and CLI use this method so they cannot
// diverge on default or missing-ID behavior.
func (s *Service) ResolveRenderer(requested string) (engine.ResolvedEngine, error) {
	resolved, err := tts.ResolveRendererWithOptions(s.catalog, s.rendererID(requested), engine.ResolveOptions{
		ResourceOverrides: map[engine.ResourceKey]string{
			engine.ResourceWorldlineBridge: s.worldlineBridgePath,
		},
	})
	if err != nil {
		return engine.ResolvedEngine{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return resolved, nil
}

// RendererAvailability returns preflight information for every discovered
// renderer. It is intentionally non-fatal so listing endpoints can explain
// missing runtimes before a user starts synthesis.
func (s *Service) RendererAvailability() map[string]engine.Availability {
	result := make(map[string]engine.Availability)
	if s.catalog == nil {
		return result
	}
	for _, renderer := range s.catalog.Renderers {
		resolved, err := s.ResolveRenderer(renderer.ID)
		if err != nil {
			result[renderer.ID] = engine.Availability{
				Available: false,
				Issues:    []engine.AvailabilityIssue{{Message: err.Error()}},
			}
			continue
		}
		availability := resolved.Availability
		if availability.Available && resolved.Provider.ID == "utau-external-resampler" {
			if _, toolsErr := s.ResolveClassicTools("", ""); toolsErr != nil {
				availability = engine.Availability{
					Available: false,
					Issues:    []engine.AvailabilityIssue{{Message: toolsErr.Error()}},
				}
			}
		}
		result[renderer.ID] = availability
	}
	return result
}

// ClassicTools are the resolved external tools selected for Classic UTAU.
type ClassicTools struct {
	Resampler plugin.ClassicTool
	Wavtool   plugin.ClassicTool
	Resources map[engine.ResourceKey]string
}

// ResolveClassicTools resolves Classic UTAU tool IDs with the same catalog
// used by every entry point.
func (s *Service) ResolveClassicTools(resamplerID, wavtoolID string) (ClassicTools, error) {
	if s.catalog == nil {
		return ClassicTools{}, fmt.Errorf("%w: renderer catalog is not initialized", ErrUnavailable)
	}
	resampler, found := s.catalog.Resampler(resamplerID)
	if !found {
		return ClassicTools{}, fmt.Errorf("%w: classic UTAU resampler %q not found", ErrUnavailable, resamplerID)
	}
	wavtool, found := s.catalog.Wavtool(wavtoolID)
	if !found {
		return ClassicTools{}, fmt.Errorf("%w: classic UTAU wavtool %q not found", ErrUnavailable, wavtoolID)
	}
	resources := map[engine.ResourceKey]string{
		engine.ResourceClassicResampler: resampler.Path,
	}
	requirements := []engine.ResourceRequirement{{
		Key: engine.ResourceClassicResampler, Required: true, Executable: true,
	}}
	if wavtool.Path != "" {
		resources[engine.ResourceClassicWavtool] = wavtool.Path
		requirements = append(requirements, engine.ResourceRequirement{
			Key: engine.ResourceClassicWavtool, Required: true, Executable: true,
		})
	}
	availability := engine.CheckResources(resources, requirements...)
	if !availability.Available {
		return ClassicTools{}, fmt.Errorf("%w: classic UTAU tools: %s", ErrUnavailable, availability.Error())
	}
	return ClassicTools{Resampler: resampler, Wavtool: wavtool, Resources: resources}, nil
}

func (s *Service) rendererID(requested string) string {
	if requested != "" {
		return requested
	}
	return s.renderer
}

// ResolveModel resolves a model ID or catalogued path to a runtime path.
func (s *Service) ResolveModel(id string) (string, error) {
	if id == "" || id == "none" {
		return "", nil
	}
	if s.catalog == nil {
		return "", fmt.Errorf("%w: model catalog is not initialized", ErrUnavailable)
	}
	model, found := s.catalog.Model(id)
	if !found {
		return "", fmt.Errorf("%w: prosody model %q not found", ErrUnavailable, id)
	}
	return model.Path, nil
}

// ModelAvailableはリクエストがプロソディモデルを選択するかを返す。
func (s *Service) ModelAvailable(id string) bool {
	if id == "" || id == "none" {
		return false
	}
	_, found := s.catalog.Model(id)
	return found
}
