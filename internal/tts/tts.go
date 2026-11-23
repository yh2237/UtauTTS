package tts

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/yh2237/UtauTTS/internal/g2p"
	_ "github.com/yh2237/UtauTTS/internal/g2p/all"
	"github.com/yh2237/UtauTTS/internal/g2p/ja"
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/openjtalk"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"github.com/yh2237/audiodsp/wav"
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
	PauseDurationSet bool
	// 補正の有無と強度は、合成側（internal/synth）で既定値を入れてから渡す。強度が0以下なら補正しない。
	ContextDuration         bool
	ContextDurationStrength float64
	// BoundaryToneは境界音調。
	BoundaryTone         bool
	BoundaryToneStrength float64
	// StretchAdaptは伸縮補正。
	StretchAdapt         bool
	StretchAdaptStrength float64
	PauseContext         bool
	PauseContextStrength float64
	// nilは既定で有効。
	Context         context.Context
	VoicebankPath   string
	Voicebank       *voicebank.Bank
	Text            string
	Reading         string
	Language        string
	Phonemizer      string
	Dictionary      map[string]string
	Tone            string
	Color           string
	MoraDurationMS  float64
	PauseDurationMS float64
	MoraDurationsMS []float64
	// PhoneWeightsはモーラ内の音素時間比。参照照合で音素ごとに長さを配分する。
	PhoneWeights [][]float64
	// ConsonantMSとVowelMSは、休止を含む全モーラの子音長・母音長。
	// どちらかを指定すると、その音素の長さを守って残りを母音へ渡す。
	ConsonantMS             []float64
	VowelMS                 []float64
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
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	PitchCurve              *render.PitchCurve
	AliasPolicy             voicebank.AliasPolicy
	World                   render.WorldOptions
	// WorldEnginePathとReferenceDirは参照照合に使うWORLDエンジンとモデルのフォルダ。Rendererに関係なく使う。
	WorldEnginePath string
	ReferenceDir    string
	// MethodはMethodWaveformSynthesisかMethodReferenceMatching。参照照合はRendererを使わない。
	Method string
	// requestedAliasPolicyは、音源に合わせて直す前の指定。計画に残す。
	requestedAliasPolicy voicebank.AliasPolicy
	// Logfは合成の途中経過を伝える。nilなら何も伝えない。
	Logf func(string)
}

type Result struct {
	Voicebank       *voicebank.Bank
	Plan            *plan.Plan
	Audio           *wav.PCM
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
	Morae           []linguistics.Mora
	Features        []prosody.FeatureFrame
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
	// FramePitchCurveは強度適用後の10ms単位のピッチ曲線。
	FramePitchCurve *render.PitchCurve
}

func ConvertToReadingContext(ctx context.Context, text string, dictionary map[string]string, openJTalk openjtalk.Config) (string, error) {
	if err := synthesisContextError(ctx); err != nil {
		return "", err
	}
	reading, frontendErr := ja.ToKanaWithDictionary(text, dictionary)
	if frontendErr == nil {
		return reading, nil
	}
	analysis, openJTalkErr := analyzeOpenJTalkCached(ctx, ja.ApplyDictionaryForAnalysis(text, dictionary), openJTalk)
	if openJTalkErr != nil {
		return "", fmt.Errorf("convert text to reading: %v; Open JTalk fallback: %w", frontendErr, openJTalkErr)
	}
	return analysis.Reading, nil
}

func ResolvePronunciation(cfg Config) (string, string, string, []linguistics.Mora, error) {
	utterance, err := resolvePronunciation(cfg)
	if err != nil {
		return "", "", "", nil, err
	}
	return utterance.Language, utterance.Phonemizer, utterance.Reading, utterance.Morae, nil
}

// 読みとモーラだけを解析する。時間やピッチの予測にはPredictProsodyを使う。
func Analyze(cfg Config) (*ProsodyPreview, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	utterance, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	return &ProsodyPreview{
		Reading: utterance.Reading,
		Morae:   append([]linguistics.Mora(nil), utterance.Morae...),
	}, nil
}

// 明示指定、音源の推定、言語既定の順に選ぶ。言語に合わない推定は使わない。
func resolveLanguagePhonemizer(cfg Config) (string, string, error) {
	if strings.TrimSpace(cfg.Phonemizer) != "" {
		return g2p.ResolveLanguage(cfg.Language, cfg.Phonemizer)
	}
	bank := cfg.Voicebank
	if bank == nil && strings.TrimSpace(cfg.VoicebankPath) != "" {
		// 読込失敗時は推定を諦め、言語既定へ委ねる。
		if loaded, err := loadVoicebankCachedContext(cfg.Context, cfg.VoicebankPath); err == nil {
			bank = loaded
		}
	}
	if bank != nil {
		suggestedLanguage, suggestedPhonemizer := bank.SuggestedLanguage()
		if strings.TrimSpace(cfg.Language) == "" {
			return g2p.ResolveLanguage(suggestedLanguage, suggestedPhonemizer)
		}
		if language, phonemizer, err := g2p.ResolveLanguage(cfg.Language, suggestedPhonemizer); err == nil {
			return language, phonemizer, nil
		}
	}
	return g2p.ResolveLanguage(cfg.Language, "")
}

func resolvePronunciation(cfg Config) (linguistics.Utterance, error) {
	language, phonemizerID, err := resolveLanguagePhonemizer(cfg)
	if err != nil {
		return linguistics.Utterance{}, err
	}
	openJTalk := openjtalk.Config{
		EnginePath: cfg.OpenJTalkPath, DictionaryPath: cfg.OpenJTalkDictionaryPath,
	}
	return g2p.Analyze(cfg.Context, language, g2p.Input{
		Text:       cfg.Text,
		Reading:    cfg.Reading,
		Phonemizer: phonemizerID,
		Dictionary: cfg.Dictionary,
		ReadingFallback: func(ctx context.Context, analyzedText string) (string, error) {
			analysis, err := analyzeOpenJTalkCached(ctx, analyzedText, openJTalk)
			if err != nil {
				return "", fmt.Errorf("Open JTalk fallback: %w", err)
			}
			return analysis.Reading, nil
		},
	})
}

func resolveProsodyModel(cfg Config) (*prosody.Model, error) {
	if cfg.ProsodyModel != nil {
		return cfg.ProsodyModel, nil
	}
	if cfg.ProsodyModelPath == "" {
		return nil, nil
	}
	return loadProsodyModelCached(cfg.Context, cfg.ProsodyModelPath)
}

func resolveProsodyFeatures(cfg Config, model *prosody.Model, morae []linguistics.Mora, reading string) ([]prosody.FeatureFrame, error) {
	if model == nil || !model.RequiresExternalFeatures() || len(cfg.ProsodyFeatures) > 0 {
		return cfg.ProsodyFeatures, nil
	}
	runtimeText := ja.ApplyDictionaryForAnalysis(cfg.Text, cfg.Dictionary)
	if strings.TrimSpace(runtimeText) == "" {
		// かなだけの入力では読みを表層テキストとして解析する。
		runtimeText = reading
	}
	runtimeConfig := openjtalk.Config{
		EnginePath: cfg.OpenJTalkPath, DictionaryPath: cfg.OpenJTalkDictionaryPath,
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

func analyzeAndAlignRuntimeFeatures(ctx context.Context, morae []linguistics.Mora, text string, cfg openjtalk.Config) ([]prosody.FeatureFrame, error) {
	analysis, err := analyzeOpenJTalkCached(ctx, text, cfg)
	if err != nil {
		return nil, fmt.Errorf("analyze runtime prosody features: %w", err)
	}
	return alignRuntimeProsodyFeatures(morae, analysis)
}

func Synthesize(cfg Config) (*Result, error) {
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	start := time.Now()
	bank := cfg.Voicebank
	var err error
	if bank == nil {
		bank, err = loadVoicebankCachedContext(cfg.Context, cfg.VoicebankPath)
		if err != nil {
			return nil, fmt.Errorf("load voicebank: %w", err)
		}
	}
	cfg.Voicebank = bank
	requestedAliasPolicy := cfg.AliasPolicy
	if requestedAliasPolicy == "" {
		requestedAliasPolicy = voicebank.AliasPolicyAuto
	}
	applyAliasProfile(bank, &cfg)
	applyVoicebankDictionary(bank, &cfg)
	traceMark(&start, "setup")
	utterance, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	method, err := lookupMethod(cfg.Method)
	if err != nil {
		return nil, err
	}
	cfg.requestedAliasPolicy = requestedAliasPolicy
	return method.Synthesize(cfg, utterance)
}

func applyVoicebankDictionary(bank *voicebank.Bank, cfg *Config) {
	if len(bank.ARPAsing) == 0 {
		return
	}
	dictionary := make(map[string]string, len(bank.ARPAsing)+len(cfg.Dictionary))
	for key, value := range bank.ARPAsing {
		dictionary[key] = value
	}
	for key, value := range cfg.Dictionary {
		dictionary[key] = value
	}
	cfg.Dictionary = dictionary
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
	if cfg.PauseDurationMS <= 0 && !cfg.PauseDurationSet {
		cfg.PauseDurationMS = plan.DefaultPauseDurationMS
	}
	if cfg.ReleaseMS <= 0 && !cfg.ReleaseSet {
		cfg.ReleaseMS = render.DefaultReleaseMS
	}

	utterance, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize: %w", err)
	}
	method, err := lookupMethod(cfg.Method)
	if err != nil {
		return nil, err
	}
	return method.PredictProsody(cfg, utterance)
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

func moraTimings(morae []linguistics.Mora, synthesisPlan *plan.Plan) []prosody.MoraTiming {
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

// 合成の方式。表示名（波形合成・参照照合）と同じ意味の識別子を使う。
const (
	MethodWaveformSynthesis = "waveform-synthesis"
	MethodReferenceMatching = "reference-matching"
)

// ValidateMethodは、合成の方式の指定を確かめる。方式は必須で、既定値はない。
func ValidateMethod(method string) error {
	switch method {
	case MethodWaveformSynthesis, MethodReferenceMatching:
		return nil
	case "":
		return fmt.Errorf("method is required: %q or %q", MethodWaveformSynthesis, MethodReferenceMatching)
	}
	return fmt.Errorf("unknown method %q: %q or %q", method, MethodWaveformSynthesis, MethodReferenceMatching)
}
