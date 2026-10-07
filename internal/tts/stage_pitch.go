package tts

import (
	"fmt"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
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
	if pitchCurve == nil && applyPitchEnabled(cfg) && rendererSupportsFramePitch(cfg.RendererCapabilities) {
		if f0Head := unifiedF0Head(loadedProsody); f0Head != nil {
			if contour := unifiedProsodyContour(f0Head, language, prosodyFeatures, curveTimings, curveDurationMS, synthesisPlan); contour != nil {
				pitchCurve = scaleAutomaticPitchCurve(contour, cfg.IntonationStrength)
			}
		}
		if pitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
			question := finalPhraseIsQuestion(cfg.Text)
			if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, curveTimings, curveDurationMS, question); contour != nil {
				pitchCurve = &render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}
				pitchCurve = scaleAutomaticPitchCurve(pitchCurve, cfg.IntonationStrength)
			}
		}
	}
	if f0Head := unifiedF0Head(loadedProsody); f0Head != nil {
		applyUnifiedProsodyEnergy(f0Head, language, prosodyFeatures, curveTimings, curveDurationMS, synthesisPlan)
	}
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
