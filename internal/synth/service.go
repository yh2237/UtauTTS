// synthパッケージはGUIとHTTPサーバで共有する合成処理を提供する。
package synth

import (
	"context"
	"errors"
	"fmt"

	"utautts/internal/engine"
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
	Text                  string
	Reading               string
	Kana                  string
	Language              string
	Phonemizer            string
	VoicebankID           string
	Tone                  string
	Color                 string
	ModelID               string
	Renderer              string
	Resampler             string
	Wavtool               string
	AliasPolicy           voicebank.AliasPolicy
	AcousticMode          string
	Dictionary            []DictionaryEntry
	MoraDurationMS        float64
	PauseDurationMS       float64
	LeadingPreutteranceMS float64
	MoraDurationsMS       []float64
	IntonationStrength    float64
	ApplyPitch            bool
	ManualPitch           *prosody.ManualPitchFile
	ResamplerExpressions  []render.ResamplerExpression
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
	worldlinePath       string
	worldlineBridgePath string
	openJTalkPath       string
	openJTalkDictionary string
	voicebanks          VoicebankResolver
}

func NewService(catalog *plugin.Catalog, renderer, worldlinePath, worldlineBridgePath, openJTalkPath, openJTalkDictionary string, voicebanks VoicebankResolver) *Service {
	return &Service{
		catalog: catalog, renderer: renderer,
		worldlinePath: worldlinePath, worldlineBridgePath: worldlineBridgePath,
		openJTalkPath: openJTalkPath, openJTalkDictionary: openJTalkDictionary,
		voicebanks: voicebanks,
	}
}

// Synthesizeはリクエストを解決し、音声・LAB・使用Rendererをまとめて返す。
func (s *Service) Synthesize(request Request) (*Result, error) {
	return s.SynthesizeContext(context.Background(), request)
}

func (s *Service) SynthesizeContext(ctx context.Context, request Request) (*Result, error) {
	cfg, rendererID, err := s.config(request, true)
	if err != nil {
		return nil, err
	}
	cfg.Context = ctx
	return SynthesizeConfig(cfg, rendererID)
}

// SynthesizeConfigは解決済み設定から共通の合成結果を作る。
func SynthesizeConfig(cfg tts.Config, rendererID string) (*Result, error) {
	result, err := tts.Synthesize(cfg)
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
	cfg, rendererID, err := s.config(request, false)
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

func (s *Service) config(request Request, requireVoicebank bool) (tts.Config, string, error) {
	modelPath, err := s.ResolveModel(request.ModelID)
	if err != nil {
		return tts.Config{}, "", err
	}
	reading := request.Reading
	if reading == "" {
		reading = request.Kana
	}
	cfg := tts.Config{
		Text:                         request.Text,
		Reading:                      reading,
		Language:                     request.Language,
		Phonemizer:                   request.Phonemizer,
		Dictionary:                   DictionaryMap(request.Dictionary),
		Tone:                         request.Tone,
		Color:                        request.Color,
		AliasPolicy:                  request.AliasPolicy,
		AcousticMode:                 request.AcousticMode,
		MoraDurationMS:               request.MoraDurationMS,
		PauseDurationMS:              request.PauseDurationMS,
		LeadingPreutteranceMS:        request.LeadingPreutteranceMS,
		MoraDurationsMS:              request.MoraDurationsMS,
		ProsodyModelPath:             modelPath,
		ManualPitch:                  request.ManualPitch,
		IntonationStrength:           request.IntonationStrength,
		ApplyPitch:                   request.ApplyPitch,
		OpenJTalkPath:                s.openJTalkPath,
		OpenJTalkDictionaryPath:      s.openJTalkDictionary,
		ExternalResamplerExpressions: append([]render.ResamplerExpression(nil), request.ResamplerExpressions...),
	}
	if requireVoicebank {
		if s.voicebanks == nil {
			return tts.Config{}, "", fmt.Errorf("%w: voicebank resolver is not configured", ErrUnavailable)
		}
		voicebankPath, ok := s.voicebanks.Resolve(request.VoicebankID)
		if !ok {
			return tts.Config{}, "", fmt.Errorf("%w: voicebank not found", ErrUnavailable)
		}
		cfg.VoicebankPath = voicebankPath
	}
	resolvedEngine, err := s.ResolveRenderer(request.Renderer)
	if err != nil {
		return tts.Config{}, "", err
	}
	if requireVoicebank {
		if availabilityErr := resolvedEngine.RequireAvailable(); availabilityErr != nil {
			return tts.Config{}, "", fmt.Errorf("%w: %v", ErrUnavailable, availabilityErr)
		}
	}
	tts.ApplyResolvedEngine(&cfg, resolvedEngine, s.worldlinePath, s.worldlineBridgePath)
	// Classic UTAUは公開Renderer IDではなく解決済みproviderで判定する。
	if requireVoicebank && resolvedEngine.Provider.ID == "utau-external-resampler" {
		tools, toolsErr := s.ResolveClassicTools(request.Resampler, request.Wavtool)
		if toolsErr != nil {
			return tts.Config{}, "", toolsErr
		}
		cfg.ExternalResamplerPath = tools.Resampler.Path
		cfg.ExternalWavtoolPath = tools.Wavtool.Path
	}
	return cfg, string(resolvedEngine.PublicID()), nil
}

// ResolveRenderer resolves the user-facing Renderer ID to its provider and
// manifest resources. GUI, HTTP, and CLI use this method so they cannot
// diverge on default or missing-ID behavior.
func (s *Service) ResolveRenderer(requested string) (engine.ResolvedEngine, error) {
	resolved, err := tts.ResolveRendererWithOptions(s.catalog, s.rendererID(requested), engine.ResolveOptions{
		ResourceOverrides: map[engine.ResourceKey]string{
			engine.ResourceWorldline:       s.worldlinePath,
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
