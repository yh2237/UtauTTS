package tts

import (
	"fmt"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

type synthesisPitch struct {
	Curve            *render.PitchCurve
	Automatic        *render.PitchCurve
	Apply            bool
	RendererStrength float64
}

// 自動輪郭に境界音調を加え、最後に手動補正を適用する。
func resolveSynthesisPitch(cfg Config, profile languageProfile, loadedProsody *prosody.Model, morae []frontend.Mora, prosodyFeatures []prosody.FeatureFrame, reading, language string, synthesisPlan *plan.Plan) (synthesisPitch, error) {
	pitchCurve := cfg.PitchCurve
	applyPitch := applyPitchEnabled(cfg)
	curveTimings := moraTimings(morae, synthesisPlan)
	curveDurationMS := synthesisPlan.DurationMS + cfg.ReleaseMS
	if pitchCurve == nil {
		if curve, enablePitch := profile.AutomaticPitchCurve(cfg, loadedProsody, morae, curveTimings, curveDurationMS); curve != nil {
			pitchCurve = curve
			if enablePitch {
				applyPitch = true
			}
		}
	}
	predictFramePitch := pitchCurve == nil && applyPitchEnabled(cfg) && rendererSupportsFramePitch(cfg.RendererCapabilities)
	f0Head := unifiedF0Head(loadedProsody)
	predictEnergy := f0Head != nil && loadedProsody.F0HeadEnergy && !cfg.ProsodyPitchOnly
	var unified unifiedProsodyPrediction
	if f0Head != nil && (predictFramePitch || predictEnergy) {
		unified = predictUnifiedProsody(cfg.Context, f0Head, language, prosodyFeatures, curveTimings, curveDurationMS, synthesisPlan, finalPhraseIsQuestion(cfg.Text), predictEnergy)
	}
	if predictFramePitch {
		pitchCurve = f0HeadPitchCurve(cfg, loadedProsody, morae, prosodyFeatures, curveTimings, curveDurationMS, unified.contour)
		if pitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
			question := finalPhraseIsQuestion(cfg.Text)
			if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, curveTimings, curveDurationMS, question); contour != nil {
				pitchCurve = &render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}
				pitchCurve = scaleAutomaticPitchCurve(pitchCurve, cfg.IntonationStrength)
			}
		}
	}
	unified.applyEnergy(f0Head, synthesisPlan)
	// 境界音調は自動輪郭だけに加える。
	if cfg.PitchCurve == nil {
		pitchCurve = profile.ApplyBoundaryTone(cfg, pitchCurve, finalPhraseEndMS(morae, curveTimings), finalPhraseIsQuestion(cfg.Text))
	}
	automaticPitchCurve := pitchCurve
	manualContour, manualMode, err := resolveManualPitchCurve(cfg, reading, morae, curveTimings, curveDurationMS)
	if err != nil {
		return synthesisPitch{}, err
	}
	if manualContour != nil {
		pitchCurve = mergeManualPitchCurve(pitchCurve, manualContour, manualMode)
	}
	intonationStrength := rendererIntonationStrength(cfg, automaticPitchCurve)
	return synthesisPitch{Curve: pitchCurve, Automatic: automaticPitchCurve, Apply: applyPitch, RendererStrength: intonationStrength}, nil
}

// F0ヘッドの曲線へ強さ、基準モデルとの混合、高さの補正を適用する。
func f0HeadPitchCurve(cfg Config, loadedProsody *prosody.Model, morae []frontend.Mora, prosodyFeatures []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, contour *render.PitchCurve) *render.PitchCurve {
	if contour == nil {
		return nil
	}
	curve := scaleUnifiedPitchCurve(contour, loadedProsody.F0Head, cfg.IntonationStrength)
	if weight := loadedProsody.F0HeadBaseBlend; curve != nil && weight > 0 && shouldPredictFrameContour(cfg, loadedProsody) {
		if base := loadedProsody.PredictFrameContour(morae, prosodyFeatures, timings, durationMS, finalPhraseIsQuestion(cfg.Text)); base != nil {
			baseCurve := scaleAutomaticPitchCurve(&render.PitchCurve{FrameMS: base.FrameMS, Cents: base.Cents}, cfg.IntonationStrength)
			curve = blendPitchCurves(curve, baseCurve, weight)
		}
	}
	return offsetPitchCurve(curve, loadedProsody.F0HeadPitchOffsetCents)
}

// offsetPitchCurveは曲線全体へ一定の高さを足す。
func offsetPitchCurve(curve *render.PitchCurve, cents float64) *render.PitchCurve {
	if curve == nil || cents == 0 {
		return curve
	}
	result := &render.PitchCurve{FrameMS: curve.FrameMS, Cents: make([]float64, len(curve.Cents))}
	for index, value := range curve.Cents {
		result.Cents[index] = value + cents
	}
	return result
}

// resolveManualPitchCurveは手動ピッチを読み込み、検証して制限済みの補正曲線を返す。
func resolveManualPitchCurve(cfg Config, reading string, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*prosody.PitchContour, string, error) {
	manual := cfg.ManualPitch
	if manual == nil && cfg.ManualPitchPath != "" {
		loaded, err := prosody.LoadManualPitch(cfg.ManualPitchPath)
		if err != nil {
			return nil, "", fmt.Errorf("load manual pitch: %w", err)
		}
		manual = loaded
	}
	if manual == nil {
		return nil, "", nil
	}
	if err := manual.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate manual pitch: %w", err)
	}
	if manual.Reading != "" && manual.Reading != reading {
		return nil, "", fmt.Errorf("manual pitch reading does not match synthesis reading")
	}
	contour, err := manual.Curve(morae, timings, durationMS)
	if err != nil {
		return nil, "", fmt.Errorf("build manual pitch curve: %w", err)
	}
	return constrainManualPitchContour(contour), manual.Mode, nil
}
