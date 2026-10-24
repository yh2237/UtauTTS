package synth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauTTS/internal/hubert"
	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/tts"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"github.com/yh2237/UtauTTS/internal/worldrender"
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
	// Methodは合成の方法。省略できない。tts.MethodWaveformSynthesisかtts.MethodReferenceMatching。
	Method      string                `json:"method"`
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
	PitchCurve              *render.PitchCurve     `json:"pitch_curve,omitempty"`
	ProsodyFeatures         []prosody.FeatureFrame `json:"-"`
	ProsodyPitchOnly        bool                   `json:"-"`
	PitchFactors            []float64              `json:"-"`
	IntonationStrength      float64                `json:"intonation_strength"`
	ContextDuration         bool                   `json:"context_duration"`
	ContextDurationStrength float64                `json:"context_duration_strength"`
	BoundaryTone            bool                   `json:"boundary_tone"`
	BoundaryToneStrength    float64                `json:"boundary_tone_strength"`
	StretchAdapt            bool                   `json:"stretch_adapt"`
	StretchAdaptStrength    float64                `json:"stretch_adapt_strength"`
	PauseContext            bool                   `json:"pause_context"`
	PauseContextStrength    float64                `json:"pause_context_strength"`
	ApplyPitch              bool                   `json:"apply_pitch"`
	CVVCTiming              string                 `json:"-"`
	CVVCTransitionGain      float64                `json:"-"`
	CVVCPreBoundaryFade     bool                   `json:"-"`
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
	openJTalkPath       string
	openJTalkDictionary string
	voicebanks          VoicebankResolver
	scope               *lifecycle.Scope
}

func NewService(catalog *plugin.Catalog, openJTalkPath, openJTalkDictionary string, voicebanks VoicebankResolver) *Service {
	return &Service{
		catalog:       catalog,
		openJTalkPath: openJTalkPath, openJTalkDictionary: openJTalkDictionary,
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
	cfg, providerOptions, err := s.config(request, true)
	if err != nil {
		return ResolvedRequest{}, err
	}
	cfg.Context = s.bind(nil)
	return ResolvedRequest{Config: cfg, ProviderOptions: providerOptions}, nil
}

func SynthesizeResolved(resolved ResolvedRequest) (*Result, error) {
	return SynthesizeConfigWithOptions(resolved.Config, resolved.ProviderOptions)
}

func SynthesizeConfigWithOptions(cfg tts.Config, providerOptions render.ProviderOptions) (*Result, error) {
	var logs []string
	if cfg.Logf != nil {
		// 呼出し側が受け取りたい場合はそちらへも流す。
		forward := cfg.Logf
		cfg.Logf = func(line string) {
			logs = append(logs, line)
			forward(line)
		}
	} else {
		cfg.Logf = func(line string) { logs = append(logs, line) }
	}
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
	output, err := NewResult(result, cfg.Method)
	if err != nil {
		return nil, err
	}
	output.Logs = logs
	return output, nil
}

// 音声・音源を読み込まずに予測する。
func (s *Service) PredictProsody(request Request) (*tts.ProsodyPreview, error) {
	return s.PredictProsodyContext(context.Background(), request)
}

func (s *Service) PredictProsodyContext(ctx context.Context, request Request) (*tts.ProsodyPreview, error) {
	ctx, release, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	cfg, _, err := s.config(request, false)
	if err != nil {
		return nil, err
	}
	cfg.Context = ctx
	return tts.PredictProsody(cfg)
}

func (s *Service) AnalyzeContext(ctx context.Context, request Request) (*tts.ProsodyPreview, error) {
	ctx, release, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	request = request.Normalized()
	if strings.TrimSpace(request.Language) == "" {
		return nil, fmt.Errorf("language is required: ja, en, zh, es, or pt")
	}
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

func (s *Service) config(request Request, requireVoicebank bool) (tts.Config, render.ProviderOptions, error) {
	request = request.Normalized()
	if err := tts.ValidateMethod(request.Method); err != nil {
		return tts.Config{}, render.ProviderOptions{}, err
	}
	if strings.TrimSpace(request.Language) == "" {
		return tts.Config{}, render.ProviderOptions{}, fmt.Errorf("language is required: ja, en, zh, es, or pt")
	}
	if requireVoicebank && request.VoicebankID == "" && request.VoicebankPath == "" {
		return tts.Config{}, render.ProviderOptions{}, fmt.Errorf("voicebank_id is required")
	}
	modelPath := strings.TrimSpace(request.ModelPath)
	if modelPath == "" {
		var err error
		modelPath, err = s.ResolveModel(request.ModelID)
		if err != nil {
			return tts.Config{}, render.ProviderOptions{}, err
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
		CVVCTiming:              request.CVVCTiming,
		CVVCTransitionGain:      request.CVVCTransitionGain,
		CVVCPreBoundaryFade:     request.CVVCPreBoundaryFade,
	}
	providerOptions := render.ProviderOptions{}
	// 型付き設定を先に入れ、renderer_settingsで上書きする。
	providerOptions.Worldline = request.Worldline
	resolveRendererSettings(request, &cfg, &providerOptions)
	voicebankPath := request.VoicebankPath
	if voicebankPath == "" && s.voicebanks != nil && (requireVoicebank || request.VoicebankID != "") {
		if path, ok := s.voicebanks.Resolve(request.VoicebankID); ok {
			voicebankPath = path
		}
	}
	if requireVoicebank {
		if voicebankPath == "" {
			if s.voicebanks == nil {
				return tts.Config{}, render.ProviderOptions{}, fmt.Errorf("%w: voicebank resolver is not configured", ErrUnavailable)
			}
			return tts.Config{}, render.ProviderOptions{}, fmt.Errorf("%w: voicebank not found", ErrUnavailable)
		}
	}
	cfg.VoicebankPath = voicebankPath
	cfg.Method = request.Method
	cfg.ReferenceDir = s.referenceDir()
	// 見つからなければ空のまま渡し、WORLDを使うときに知らせる。
	cfg.WorldEnginePath, _ = worldrender.ResolveEnginePath()
	return cfg, providerOptions, nil
}

// PrepareReferenceIndexは音源の参照照合の索引がなければ裏で作り始める。モデルかWORLDエンジンがなければ何もしない。
func (s *Service) PrepareReferenceIndex(bank *voicebank.Bank) {
	worldEnginePath, err := worldrender.ResolveEnginePath()
	if err != nil {
		return
	}
	tts.PrepareReferenceIndex(s.referenceDir(), bank, worldEnginePath)
}

// FeatureDeviceは、直近の特徴量計算に使ったデバイスを返す。
func (s *Service) FeatureDevice() string {
	return hubert.LastDevice()
}

// ReferenceProvidersは、使える参照TTSを返す。
func (s *Service) ReferenceProviders() []tts.ReferenceProviderInfo {
	return tts.ReferenceProviders(s.referenceDir())
}

// referenceDirは、抑揚モデルのあるフォルダのうち、参照照合のモデルを置いたフォルダを返す。
func (s *Service) referenceDir() string {
	if s.catalog == nil {
		return ""
	}
	for _, model := range s.catalog.Models {
		dir := filepath.Join(filepath.Dir(model.Path), tts.ReferenceDir)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// retiredModelIDsは同梱から外した抑揚モデルと、その代わりに使うモデル。
var retiredModelIDs = map[string]string{
	"frame-intonation-tcn-v9.1-t": "frame-intonation-tcn-v12",
	"frame-intonation-tcn-v10":    "frame-intonation-tcn-v12",
	"frame-intonation-tcn-v11":    "frame-intonation-tcn-v12",
}

func (s *Service) ResolveModel(id string) (string, error) {
	if id == "" || id == "none" {
		return "", nil
	}
	if s.catalog == nil {
		return "", fmt.Errorf("%w: model catalog is not initialized", ErrUnavailable)
	}
	model, found := s.catalog.Model(id)
	if !found {
		if replacement, retired := retiredModelIDs[id]; retired {
			return "", fmt.Errorf("%w: prosody model %q was retired; use %q", ErrUnavailable, id, replacement)
		}
		return "", fmt.Errorf("%w: prosody model %q not found", ErrUnavailable, id)
	}
	return model.Path, nil
}

func (s *Service) ModelAvailable(id string) bool {
	if id == "" || id == "none" {
		return false
	}
	_, found := s.catalog.Model(id)
	return found
}

// RetiredModelIDsは、保存済みのプロジェクトを読み直す側へ読み替えの表を渡す。
// 読み替えるのはプロジェクトを持つGUIの役目なので、ここでは知らせるだけにする。
func RetiredModelIDs() map[string]string {
	result := make(map[string]string, len(retiredModelIDs))
	for id, replacement := range retiredModelIDs {
		result[id] = replacement
	}
	return result
}
