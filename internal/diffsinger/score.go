package diffsinger

import (
	"context"
	"fmt"
	"math"
	"strings"

	"utautts/internal/audio"
	"utautts/internal/engine"
)

// Score is kept as a provider-package alias so existing DiffSinger callers
// remain source-compatible while the actual contract lives in engine.
type Score = engine.NeuralScore

// RenderScore renders a score using the selected singer and a resident bridge
// session when the bridge supports the provider protocol.
func RenderScore(ctx context.Context, bridgePath string, singer *Singer, score Score) (*audio.PCM, error) {
	request, err := RequestFromScore(singer, score)
	if err != nil {
		return nil, err
	}
	return renderSession(ctx, bridgePath, score, request)
}

// RequestFromScore converts a score into the current bridge request format.
// The conversion is intentionally kept inside the DiffSinger provider so the
// tts package does not need to know about model-specific request fields.
func RequestFromScore(singer *Singer, score Score) (Request, error) {
	if singer == nil {
		return Request{}, fmt.Errorf("DiffSinger singer is nil")
	}
	if len(score.Symbols) != len(score.Durations) {
		return Request{}, fmt.Errorf("DiffSinger score symbols and durations have different lengths")
	}

	tokens := make([]int64, 0, len(score.Symbols))
	for _, symbol := range score.Symbols {
		token, err := singer.Token(symbol)
		if err != nil {
			return Request{}, err
		}
		tokens = append(tokens, token)
	}

	const steps = int64(20)
	depth, err := scoreDepth(singer.Config)
	if err != nil {
		return Request{}, err
	}
	request := Request{
		AcousticPath:              singer.AcousticPath,
		VocoderPath:               singer.VocoderPath,
		Tokens:                    tokens,
		Durations:                 score.Durations,
		F0:                        score.F0,
		SampleRate:                singer.Config.SampleRate,
		Steps:                     steps,
		Speedup:                   scoreDiffusionSpeedup(steps),
		PitchControllable:         singer.Vocoder.PitchControllable,
		UseContinuousAcceleration: singer.Config.UseContinuousAcceleration,
		UseVariableDepth:          scoreUsesVariableDepth(singer.Config),
		Depth:                     depth,
		UseGender:                 singer.Config.UseKeyShiftEmbed,
		UseVelocity:               singer.Config.UseSpeedEmbed,
		UseEnergy:                 singer.Config.UseEnergyEmbed,
		UseBreathiness:            singer.Config.UseBreathinessEmbed,
		UseVoicing:                singer.Config.UseVoicingEmbed,
		UseTension:                singer.Config.UseTensionEmbed,
		SpeakerEmbed:              singer.SpeakerEmbed,
		MelScale:                  scoreMelScale(singer.Config.MelBase, singer.Vocoder.MelBase),
	}
	if singer.Config.UseLangID {
		request.Languages = scoreLanguages(score.Symbols, singer.LanguageIDs)
	}

	if singer.Duration != nil {
		request.DurationLinguisticPath = singer.Duration.LinguisticPath
		request.DurationPredictorPath = singer.Duration.PredictorPath
		request.DurationTokens, err = modelTokens(singer.Duration.Tokens, score.Symbols, "duration")
		if err != nil {
			return Request{}, err
		}
		request.WordDiv = score.WordDiv
		request.WordDur = score.WordDur
		request.PhMIDI = repeatedMIDI(score.MIDI, len(score.Symbols))
		request.DurationSpeakerEmbed = singer.Duration.SpeakerEmbed
		request.DurationPredictorMix = 0.2
		if singer.Duration.Config.UseLangID {
			request.DurationLanguages = scoreLanguages(score.Symbols, singer.Duration.LanguageIDs)
		}
	}
	if singer.Pitch != nil && score.UsePitchPredictor {
		request.PitchLinguisticPath = singer.Pitch.LinguisticPath
		request.PitchPredictorPath = singer.Pitch.PredictorPath
		request.PitchTokens, err = modelTokens(singer.Pitch.Tokens, score.Symbols, "pitch")
		if err != nil {
			return Request{}, err
		}
		request.WordDiv = score.WordDiv
		request.WordDur = score.WordDur
		request.PitchSpeakerEmbed = singer.Pitch.SpeakerEmbed
		request.PitchPredictsDur = singer.Pitch.Config.PredictDur == nil || *singer.Pitch.Config.PredictDur
		request.PitchContinuous = singer.Pitch.Config.UseContinuousAcceleration
		request.PitchUseExpr = singer.Pitch.Config.UseExpr
		request.PitchUseNoteRest = singer.Pitch.Config.UseNoteRest
		request.PitchPredictorMix = 0.03
		request.NoteMIDI = repeatedMIDIFloat32(float32(score.MIDI), len(score.WordDiv))
		request.NoteRest = append([]bool(nil), score.NoteRest...)
		if singer.Pitch.Config.UseLangID {
			request.PitchLanguages = scoreLanguages(score.Symbols, singer.Pitch.LanguageIDs)
		}
	}
	if singer.Variance != nil {
		request.VarianceLinguisticPath = singer.Variance.LinguisticPath
		request.VariancePredictorPath = singer.Variance.PredictorPath
		request.VarianceTokens, err = modelTokens(singer.Variance.Tokens, score.Symbols, "variance")
		if err != nil {
			return Request{}, err
		}
		request.WordDiv = score.WordDiv
		request.WordDur = score.WordDur
		request.VarianceSpeakerEmbed = singer.Variance.SpeakerEmbed
		request.VariancePredictsDur = singer.Variance.Config.PredictDur
		request.VariancePredictsEnergy = singer.Variance.Config.PredictEnergy
		request.VariancePredictsBreath = singer.Variance.Config.PredictBreathiness
		request.VariancePredictsVoicing = singer.Variance.Config.PredictVoicing
		request.VariancePredictsTension = singer.Variance.Config.PredictTension
		request.VarianceContinuous = singer.Variance.Config.UseContinuousAcceleration
		if singer.Variance.Config.UseLangID {
			request.VarianceLanguages = scoreLanguages(score.Symbols, singer.Variance.LanguageIDs)
		}
	}
	return request, nil
}

func modelTokens(tokens map[string]int64, symbols []string, model string) ([]int64, error) {
	result := make([]int64, len(symbols))
	for index, symbol := range symbols {
		token, found := tokens[symbol]
		if !found {
			return nil, fmt.Errorf("%s model has no phoneme %q", model, symbol)
		}
		result[index] = token
	}
	return result, nil
}

func scoreLanguages(symbols []string, languageIDs map[string]int64) []int64 {
	result := make([]int64, len(symbols))
	for index, symbol := range symbols {
		if slash := strings.IndexByte(symbol, '/'); slash > 0 {
			result[index] = languageIDs[symbol[:slash]]
		}
	}
	return result
}

func repeatedMIDI(midi, length int) []int64 {
	result := make([]int64, length)
	for index := range result {
		result[index] = int64(midi)
	}
	return result
}

func repeatedMIDIFloat32(midi float32, length int) []float32 {
	result := make([]float32, length)
	for index := range result {
		result[index] = midi
	}
	return result
}

func scoreDiffusionSpeedup(steps int64) int64 {
	value := int64(1000) / steps
	if value < 1 {
		value = 1
	}
	for value > 1 && 1000%value != 0 {
		value--
	}
	return value
}

func scoreUsesVariableDepth(cfg Config) bool {
	if cfg.UseVariableDepth != nil {
		return *cfg.UseVariableDepth
	}
	return cfg.UseShallowDiffusion != nil && *cfg.UseShallowDiffusion
}

func scoreDepth(cfg Config) (float32, error) {
	if !scoreUsesVariableDepth(cfg) {
		return 1, nil
	}
	maximum := cfg.MaxDepth
	if !cfg.UseContinuousAcceleration {
		maximum /= 1000
	}
	if maximum < 0 {
		return 0, fmt.Errorf("DiffSinger max_depth must not be negative")
	}
	return float32(math.Min(1, maximum)), nil
}

func scoreMelScale(acousticBase, vocoderBase string) float32 {
	if acousticBase == vocoderBase {
		return 1
	}
	if acousticBase == "10" && vocoderBase == "e" {
		return 2.30259
	}
	return 0.434294
}
