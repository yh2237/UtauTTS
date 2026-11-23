package tts

import (
	"fmt"
	"math"
	"time"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/phonemizer"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// waveformMethodは、音源の原音をつないで合成する方式。
type waveformMethod struct{}

func init() { RegisterMethod(MethodWaveformSynthesis, waveformMethod{}) }

func (waveformMethod) Synthesize(cfg Config, utterance linguistics.Utterance) (*Result, error) {
	start := time.Now()
	bank := cfg.Voicebank
	requestedAliasPolicy := cfg.requestedAliasPolicy
	ph, err := phonemizer.Resolve(utterance.Phonemizer)
	if err != nil {
		return nil, fmt.Errorf("language %q does not support method %q: %w", utterance.Language, MethodWaveformSynthesis, err)
	}
	aliases, err := ph.Aliases(utterance, bank.Presamp.Config())
	if err != nil {
		return nil, fmt.Errorf("phonemize aliases: %w", err)
	}
	language, phonemizerID, reading, morae := utterance.Language, utterance.Phonemizer, utterance.Reading, utterance.Morae
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
		Tone: cfg.Tone, Color: cfg.Color, AliasPolicy: cfg.AliasPolicy, Aliases: aliases,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve voicebank units: %w", err)
	}
	traceMark(&start, "select")
	synthesisPlan, stretchAdapt, err := buildSynthesisPlan(cfg, profile, bank, reading, language, phonemizerID, morae, selections, predictions, requestedAliasPolicy)
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
		Context:               cfg.Context,
		WorldEnginePath:       cfg.WorldEnginePath,
		ReleaseMS:             cfg.ReleaseMS,
		ReleaseSet:            cfg.ReleaseSet,
		LeadingPreutteranceMS: cfg.LeadingPreutteranceMS,
		IntonationStrength:    pitch.RendererStrength,
		ApplyPitch:            pitch.Apply,
		World:                 cfg.World,
		CVVCTiming:            cfg.CVVCTiming,
		CVVCTransitionGain:    cfg.CVVCTransitionGain,
		CVVCPreBoundaryFade:   cfg.CVVCPreBoundaryFade,
		PitchCurve:            pitch.Curve,
		StretchAdapt:          stretchAdapt,
		StretchAdaptStrength:  cfg.StretchAdaptStrength,
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

func (waveformMethod) PredictProsody(cfg Config, utterance linguistics.Utterance) (*ProsodyPreview, error) {
	if _, err := phonemizer.Resolve(utterance.Phonemizer); err != nil {
		return nil, fmt.Errorf("language %q does not support method %q: %w", utterance.Language, MethodWaveformSynthesis, err)
	}
	language, reading, morae := utterance.Language, utterance.Reading, utterance.Morae
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
		Reading: reading, Morae: append([]linguistics.Mora(nil), morae...),
		Features:        append([]prosody.FeatureFrame(nil), prosodyFeatures...),
		MoraDurationsMS: make([]float64, len(morae)),
		MoraPositionsMS: make([]float64, len(morae)),
		PitchPoints:     make([]float64, len(morae)),
	}
	cursor := 0.0
	for index, mora := range morae {
		duration, manuallySet := plan.ConfiguredMoraDuration(index, cfg.MoraDurationsMS)
		if !manuallySet {
			var prediction prosody.Prediction
			if index < len(predictions) {
				prediction = predictions[index]
			}
			if mora.Pause {
				duration = plan.PredictedDuration(cfg.PauseDurationMS*plan.PauseContextFactor(morae, index, cfg.PauseContext, cfg.PauseContextStrength), prediction)
			} else {
				duration = plan.PredictedDuration(plan.DurationFor(mora, cfg.MoraDurationMS), prediction)
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
	if result.FramePitchCurve == nil && applyPitchEnabled(cfg) {
		unified := predictUnifiedProsody(cfg.Context, unifiedF0Head(loadedProsody), language, prosodyFeatures, timings, totalDurationMS, previewPlan(morae, timings, cursor), finalPhraseIsQuestion(cfg.Text), false)
		result.FramePitchCurve = f0HeadPitchCurve(cfg, loadedProsody, morae, prosodyFeatures, timings, totalDurationMS, unified.contour)
	}
	if result.FramePitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
		question := finalPhraseIsQuestion(cfg.Text)
		if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, timings, totalDurationMS, question); contour != nil {
			curve := scaleAutomaticPitchCurve(&render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}, cfg.IntonationStrength)
			result.FramePitchCurve = curve
		}
	}
	if result.FramePitchCurve != nil {
		// 合成と同じく、自動輪郭へ境界音調を加える。
		result.FramePitchCurve = profile.ApplyBoundaryTone(cfg, result.FramePitchCurve, finalPhraseEndMS(morae, timings), finalPhraseIsQuestion(cfg.Text))
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
