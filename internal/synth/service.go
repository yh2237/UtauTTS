package synth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauTTS/internal/engine"
	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	_ "github.com/yh2237/UtauTTS/internal/render/worldline" // 同梱の既定Rendererをinitで登録する。
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/tts"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var ErrUnavailable = errors.New("unavailable")

const DefaultApplyPitch = true

// 設定の既定値は設定表（internal/settings）から引き、入口ごとの差を防ぐ。
var (
	DefaultMoraDurationMS          = settings.Number("mora_duration_ms")
	DefaultPauseDurationMS         = settings.Number("pause_duration_ms")
	DefaultIntonationStrength      = settings.Number("intonation_strength")
	DefaultContextDuration         = settings.Bool("context_duration")
	DefaultContextDurationStrength = settings.Number("context_duration_strength")
	DefaultBoundaryTone            = settings.Bool("boundary_tone")
	DefaultBoundaryToneStrength    = settings.Number("boundary_tone_strength")
	DefaultStretchAdapt            = settings.Bool("stretch_adapt")
	DefaultStretchAdaptStrength    = settings.Number("stretch_adapt_strength")
	DefaultPauseContext            = settings.Bool("pause_context")
	DefaultPauseContextStrength    = settings.Number("pause_context_strength")
)

const DefaultReleaseMS = render.DefaultReleaseMS

// DefaultRequestは全入口が共通で使う既定のリクエスト。renderer settingsの既定値と同期させる。
func DefaultRequest() Request {
	return Request{
		MoraDurationMS:          DefaultMoraDurationMS,
		PauseDurationMS:         DefaultPauseDurationMS,
		IntonationStrength:      DefaultIntonationStrength,
		ContextDuration:         DefaultContextDuration,
		ContextDurationStrength: DefaultContextDurationStrength,
		BoundaryTone:            DefaultBoundaryTone,
		BoundaryToneStrength:    DefaultBoundaryToneStrength,
		StretchAdapt:            DefaultStretchAdapt,
		StretchAdaptStrength:    DefaultStretchAdaptStrength,
		PauseContext:            DefaultPauseContext,
		PauseContextStrength:    DefaultPauseContextStrength,
		ApplyPitch:              DefaultApplyPitch,
		Wavtool:                 "builtin",
	}
}

type Request struct {
	PauseDurationSet bool   `json:"-"`
	Text             string `json:"text"`
	Reading          string `json:"reading"`
	Kana             string `json:"kana"`
	Language         string `json:"language"`
	Phonemizer       string `json:"phonemizer"`
	VoicebankID      string `json:"voicebank_id"`
	VoicebankPath    string `json:"-"`
	Tone             string `json:"tone"`
	Color            string `json:"color"`
	ModelID          string `json:"model_id"`
	ModelPath        string `json:"model_path"`
	Renderer         string `json:"renderer"`
	// Methodは合成の方法。空なら言語ごとの既定、"native"はプラン、"reference"は参照TTS。
	Method      string                `json:"method"`
	Resampler   string                `json:"resampler"`
	Wavtool     string                `json:"wavtool"`
	AliasPolicy voicebank.AliasPolicy `json:"alias_policy"`
	Dictionary  []DictionaryEntry     `json:"dictionary"`
	// 互換用の固定フィールド。同じIDがrenderer_settingsにあればそちらを優先する。
	MoraDurationMS        float64                  `json:"mora_duration_ms"`
	PauseDurationMS       float64                  `json:"pause_duration_ms"`
	LeadingPreutteranceMS float64                  `json:"leading_preutterance_ms"`
	MoraDurationsMS       []float64                `json:"mora_durations_ms"`
	UnitOverrides         []plan.UnitOverride      `json:"unit_overrides"`
	ReleaseMS             float64                  `json:"release_ms"`
	ReleaseSet            bool                     `json:"release_set"`
	ManualPitchPath       string                   `json:"-"`
	ManualPitch           *prosody.ManualPitchFile `json:"manual_pitch"`
	// PitchCurveはコーパス指定の固定ピッチ曲線。指定時は自動輪郭より優先される。
	PitchCurve              *render.PitchCurve           `json:"pitch_curve,omitempty"`
	ProsodyFeatures         []prosody.FeatureFrame       `json:"-"`
	ProsodyPitchOnly        bool                         `json:"-"`
	PitchFactors            []float64                    `json:"-"`
	IntonationStrength      float64                      `json:"intonation_strength"`
	ContextDuration         bool                         `json:"context_duration"`
	ContextDurationStrength float64                      `json:"context_duration_strength"`
	BoundaryTone            bool                         `json:"boundary_tone"`
	BoundaryToneStrength    float64                      `json:"boundary_tone_strength"`
	StretchAdapt            bool                         `json:"stretch_adapt"`
	StretchAdaptStrength    float64                      `json:"stretch_adapt_strength"`
	PauseContext            bool                         `json:"pause_context"`
	PauseContextStrength    float64                      `json:"pause_context_strength"`
	ApplyPitch              bool                         `json:"apply_pitch"`
	BoundaryBridgeMS        float64                      `json:"-"`
	BoundaryBridgeThreshold float64                      `json:"-"`
	CVVCTiming              string                       `json:"-"`
	CVVCTransitionGain      float64                      `json:"-"`
	CVVCPreBoundaryFade     bool                         `json:"-"`
	ResamplerExpressions    []render.ResamplerExpression `json:"resampler_expressions"`
	DiffSingerSteps         int64                        `json:"diffsinger_steps"`
	DiffSingerDurationMix   float64                      `json:"diffsinger_duration_mix"`
	DiffSingerPitchMix      float64                      `json:"diffsinger_pitch_mix"`
	DiffSingerExpr          float64                      `json:"diffsinger_expr"`
	// 空文字とnilは既定値を使う。
	Worldline render.WorldlineProviderOptions `json:"worldline,omitempty"`
	// 未知の設定IDもエラーにせず、実装固有値として渡す。
	RendererSettings map[string]json.RawMessage `json:"renderer_settings,omitempty"`
}

func (request Request) Normalized() Request {
	if request.Reading == "" {
		request.Reading = request.Kana
	}
	return request
}

// ResolvedRequestは合成へ渡す解決済み入力。CLIはUSTX出力にConfigを、GUI/HTTPは通常Synthesizeを使う。
type ResolvedRequest struct {
	Config          tts.Config
	RendererID      string
	ProviderOptions render.ProviderOptions
}

type DictionaryEntry struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

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

type Service struct {
	catalog             *plugin.Catalog
	renderer            string
	worldlineBridgePath string
	openJTalkPath       string
	openJTalkDictionary string
	voicebanks          VoicebankResolver
	scope               *lifecycle.Scope
}

func NewService(catalog *plugin.Catalog, renderer, worldlineBridgePath, openJTalkPath, openJTalkDictionary string, voicebanks VoicebankResolver) *Service {
	return &Service{
		catalog: catalog, renderer: renderer,
		worldlineBridgePath: worldlineBridgePath,
		openJTalkPath:       openJTalkPath, openJTalkDictionary: openJTalkDictionary,
		voicebanks: voicebanks,
	}
}

func (s *Service) Synthesize(request Request) (*Result, error) {
	return s.SynthesizeContext(context.Background(), request)
}

func (s *Service) SynthesizeContext(ctx context.Context, request Request) (*Result, error) {
	ctx, release, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	resolved, err := s.ResolveSynthesis(request)
	if err != nil {
		return nil, err
	}
	resolved.Config.Context = s.bind(ctx)
	return SynthesizeResolved(resolved)
}

func (s *Service) ResolveSynthesis(request Request) (ResolvedRequest, error) {
	cfg, rendererID, providerOptions, err := s.config(request, true)
	if err != nil {
		return ResolvedRequest{}, err
	}
	cfg.Context = s.bind(nil)
	return ResolvedRequest{Config: cfg, RendererID: rendererID, ProviderOptions: providerOptions}, nil
}

func SynthesizeResolved(resolved ResolvedRequest) (*Result, error) {
	return SynthesizeConfigWithOptions(resolved.Config, resolved.RendererID, resolved.ProviderOptions)
}

func SynthesizeConfigWithOptions(cfg tts.Config, rendererID string, providerOptions render.ProviderOptions) (*Result, error) {
	if scope := lifecycle.FromContext(cfg.Context); scope != nil {
		ctx, release, err := scope.Begin(cfg.Context)
		if err != nil {
			return nil, err
		}
		defer release()
		cfg.Context = ctx
	}
	result, err := tts.SynthesizeWithOptions(cfg, providerOptions)
	if err != nil {
		return nil, err
	}
	return NewResult(result, rendererID)
}

// 音声・音源を読み込まずに予測する。
func (s *Service) PredictProsody(request Request) (*tts.ProsodyPreview, string, error) {
	return s.PredictProsodyContext(context.Background(), request)
}

func (s *Service) PredictProsodyContext(ctx context.Context, request Request) (*tts.ProsodyPreview, string, error) {
	ctx, release, err := s.begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()
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

func (s *Service) AnalyzeContext(ctx context.Context, request Request) (*tts.ProsodyPreview, error) {
	ctx, release, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	request = request.Normalized()
	dictionary := DictionaryMap(request.Dictionary)
	voicebankPath := request.VoicebankPath
	if voicebankPath == "" && s.voicebanks != nil && request.VoicebankID != "" {
		if path, ok := s.voicebanks.Resolve(request.VoicebankID); ok {
			voicebankPath = path
		}
	}
	if request.Language == "en" && voicebankPath != "" {
		arpasing, _, err := voicebank.LoadARPAsingDictionary(voicebankPath)
		if err != nil {
			return nil, err
		}
		for surface, reading := range arpasing {
			if dictionary[surface] == "" {
				dictionary[surface] = reading
			}
		}
	}
	return tts.Analyze(tts.Config{
		Context: ctx, Text: request.Text, Reading: request.Reading,
		Language: request.Language, Phonemizer: request.Phonemizer,
		VoicebankPath: voicebankPath,
		Dictionary:    dictionary, OpenJTalkPath: s.openJTalkPath,
		OpenJTalkDictionaryPath: s.openJTalkDictionary,
	})
}

func (s *Service) bind(ctx context.Context) context.Context {
	if s.scope != nil {
		return s.scope.Bind(ctx)
	}
	return ctx
}

func (s *Service) begin(ctx context.Context) (context.Context, func(), error) {
	if s.scope != nil {
		return s.scope.Begin(ctx)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx, func() {}, ctx.Err()
}

func (s *Service) config(request Request, requireVoicebank bool) (tts.Config, string, render.ProviderOptions, error) {
	request = request.Normalized()
	modelPath := strings.TrimSpace(request.ModelPath)
	if modelPath == "" {
		var err error
		modelPath, err = s.ResolveModel(request.ModelID)
		if err != nil {
			return tts.Config{}, "", render.ProviderOptions{}, err
		}
	}
	reading := request.Reading
	cfg := tts.Config{
		PauseDurationSet:        request.PauseDurationSet,
		Text:                    request.Text,
		Reading:                 reading,
		Language:                request.Language,
		Phonemizer:              request.Phonemizer,
		Dictionary:              DictionaryMap(request.Dictionary),
		Tone:                    request.Tone,
		Color:                   request.Color,
		AliasPolicy:             request.AliasPolicy,
		MoraDurationsMS:         request.MoraDurationsMS,
		UnitOverrides:           append([]plan.UnitOverride(nil), request.UnitOverrides...),
		ReleaseMS:               request.ReleaseMS,
		ReleaseSet:              request.ReleaseSet,
		ProsodyModelPath:        modelPath,
		ManualPitchPath:         request.ManualPitchPath,
		ManualPitch:             request.ManualPitch,
		PitchCurve:              request.PitchCurve,
		ProsodyFeatures:         append([]prosody.FeatureFrame(nil), request.ProsodyFeatures...),
		ProsodyPitchOnly:        request.ProsodyPitchOnly,
		PitchFactors:            append([]float64(nil), request.PitchFactors...),
		ApplyPitch:              request.ApplyPitch,
		OpenJTalkPath:           s.openJTalkPath,
		OpenJTalkDictionaryPath: s.openJTalkDictionary,
		BoundaryBridgeMS:        request.BoundaryBridgeMS,
		BoundaryBridgeThreshold: request.BoundaryBridgeThreshold,
		CVVCTiming:              request.CVVCTiming,
		CVVCTransitionGain:      request.CVVCTransitionGain,
		CVVCPreBoundaryFade:     request.CVVCPreBoundaryFade,
	}
	providerOptions := render.ProviderOptions{
		Classic: render.ClassicOptions{
			ResamplerExpressions: append([]render.ResamplerExpression(nil), request.ResamplerExpressions...),
		},
	}
	// 型付き設定を先に入れ、renderer_settingsで上書きする。
	providerOptions.Worldline = request.Worldline
	resolution := resolveRendererSettings(request, &cfg, &providerOptions)
	voicebankPath := request.VoicebankPath
	if voicebankPath == "" && s.voicebanks != nil && (requireVoicebank || request.VoicebankID != "") {
		if path, ok := s.voicebanks.Resolve(request.VoicebankID); ok {
			voicebankPath = path
		}
	}
	if requireVoicebank {
		if voicebankPath == "" {
			if s.voicebanks == nil {
				return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: voicebank resolver is not configured", ErrUnavailable)
			}
			return tts.Config{}, "", render.ProviderOptions{}, fmt.Errorf("%w: voicebank not found", ErrUnavailable)
		}
	}
	cfg.VoicebankPath = voicebankPath
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
	cfg.Method = request.Method
	cfg.EnglishReferenceDir = s.englishReferenceDir()
	cfg.WorldEnginePath = resolvedEngine.Resource(engine.ResourceWorldEngine)
	if cfg.WorldEnginePath == "" {
		if worldPhrase, err := s.ResolveRenderer("utautts-world-phrase"); err == nil {
			cfg.WorldEnginePath = worldPhrase.Resource(engine.ResourceWorldEngine)
		}
	}
	if requireVoicebank && resolvedEngine.Provider.ClassicTools {
		tools, toolsErr := s.ResolveClassicTools(
			firstNonEmpty(resolution.Resampler, request.Resampler),
			firstNonEmpty(resolution.Wavtool, request.Wavtool),
		)
		if toolsErr != nil {
			return tts.Config{}, "", render.ProviderOptions{}, toolsErr
		}
		providerOptions.Classic.ResamplerPath = tools.Resampler.Path
		providerOptions.Classic.WavtoolPath = tools.Wavtool.Path
	}
	return cfg, string(resolvedEngine.PublicID()), providerOptions, nil
}

// PrepareEnglishは音源の英語の索引がなければ裏で作り始める。モデルかWORLDエンジンがなければ何もしない。
func (s *Service) PrepareEnglish(bank *voicebank.Bank) {
	worldPhrase, err := s.ResolveRenderer("utautts-world-phrase")
	if err != nil {
		return
	}
	tts.PrepareEnglishIndex(s.englishReferenceDir(), bank, worldPhrase.Resource(engine.ResourceWorldEngine))
}

// englishReferenceDirは、抑揚モデルのあるフォルダのうち、英語の合成のモデルを置いたフォルダを返す。
func (s *Service) englishReferenceDir() string {
	if s.catalog == nil {
		return ""
	}
	for _, model := range s.catalog.Models {
		dir := filepath.Join(filepath.Dir(model.Path), tts.EnglishReferenceDir)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

func firstNonEmpty(override, fallback string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return fallback
}

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

// 資源不足はエラーでなく、一覧表示用の情報として返す。
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
		if availability.Available && resolved.Provider.ClassicTools {
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

type ClassicTools struct {
	Resampler plugin.ClassicTool
	Wavtool   plugin.ClassicTool
	Resources map[engine.ResourceKey]string
}

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

// retiredModelIDsは同梱から外した抑揚モデルと、その代わりに使うモデル。保存済みのプロジェクトや設定、APIの指定を引き継ぐ。
var retiredModelIDs = map[string]string{
	"frame-intonation-tcn-v9.1-t": "frame-intonation-tcn-v12",
	"frame-intonation-tcn-v10":    "frame-intonation-tcn-v12",
	"frame-intonation-tcn-v11":    "frame-intonation-tcn-v12",
}

// currentModelIDは同梱から外したモデルのIDを、代わりのモデルのIDへ置き換える。
func currentModelID(id string) string {
	if replacement, ok := retiredModelIDs[id]; ok {
		return replacement
	}
	return id
}

func (s *Service) ResolveModel(id string) (string, error) {
	if id == "" || id == "none" {
		return "", nil
	}
	if s.catalog == nil {
		return "", fmt.Errorf("%w: model catalog is not initialized", ErrUnavailable)
	}
	id = currentModelID(id)
	model, found := s.catalog.Model(id)
	if !found {
		return "", fmt.Errorf("%w: prosody model %q not found", ErrUnavailable, id)
	}
	return model.Path, nil
}

func (s *Service) ModelAvailable(id string) bool {
	if id == "" || id == "none" {
		return false
	}
	_, found := s.catalog.Model(currentModelID(id))
	return found
}
