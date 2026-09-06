// synthパッケージはGUIとHTTPサーバで共有する合成処理を提供する。
package synth

import (
	"context"
	"errors"
	"fmt"

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
	modelPath, err := s.modelPath(request.ModelID)
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
	rendererID := s.rendererID(request.Renderer)
	resolvedRendererID, err := tts.ApplyRenderer(&cfg, s.catalog, rendererID, s.worldlinePath, s.worldlineBridgePath)
	if err != nil {
		return tts.Config{}, "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	// Classic UTAUは予約IDではなくbackendで判定する。
	if requireVoicebank && cfg.Renderer == "utau-external-resampler" {
		resampler, found := s.catalog.Resampler(request.Resampler)
		if !found {
			return tts.Config{}, "", fmt.Errorf("%w: classic UTAU resampler %q not found", ErrUnavailable, request.Resampler)
		}
		wavtool, found := s.catalog.Wavtool(request.Wavtool)
		if !found {
			return tts.Config{}, "", fmt.Errorf("%w: classic UTAU wavtool %q not found", ErrUnavailable, request.Wavtool)
		}
		cfg.ExternalResamplerPath = resampler.Path
		cfg.ExternalWavtoolPath = wavtool.Path
	}
	return cfg, resolvedRendererID, nil
}

func (s *Service) rendererID(requested string) string {
	if requested != "" {
		return requested
	}
	return s.renderer
}

// modelPathはモデルIDをパスへ解決する。空または"none"ならモデルを使わない。
func (s *Service) modelPath(id string) (string, error) {
	if id == "" || id == "none" {
		return "", nil
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
