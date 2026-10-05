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
	if pitchCurve == nil && shouldPredictFrameContour(cfg, loadedProsody) {
		question := finalPhraseIsQuestion(cfg.Text)
		if contour := loadedProsody.PredictFrameContour(morae, prosodyFeatures, curveTimings, curveDurationMS, question); contour != nil {
			pitchCurve = &render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}
			pitchCurve = scaleAutomaticPitchCurve(pitchCurve, cfg.IntonationStrength)
		}
	}
	// 境界音調は自動輪郭だけに加える。
	if cfg.PitchCurve == nil {
		pitchCurve = profile.ApplyBoundaryTone(cfg, pitchCurve, finalPhraseEndMS(morae, curveTimings), finalPhraseIsQuestion(cfg.Text))
	}
	automaticPitchCurve := pitchCurve
	manualPitch := cfg.ManualPitch
	if manualPitch == nil && cfg.ManualPitchPath != "" {
		var err error
		manualPitch, err = prosody.LoadManualPitch(cfg.ManualPitchPath)
		if err != nil {
			return synthesisPitch{}, fmt.Errorf("load manual pitch: %w", err)
		}
	}
	if manualPitch != nil {
		if err := manualPitch.Validate(); err != nil {
			return synthesisPitch{}, fmt.Errorf("validate manual pitch: %w", err)
		}
		if manualPitch.Reading != "" && manualPitch.Reading != reading {
			return synthesisPitch{}, fmt.Errorf("manual pitch reading does not match synthesis reading")
		}
		timings := moraTimings(morae, synthesisPlan)
		manualContour, curveErr := manualPitch.Curve(morae, timings, synthesisPlan.DurationMS+cfg.ReleaseMS)
		if curveErr != nil {
			return synthesisPitch{}, fmt.Errorf("build manual pitch curve: %w", curveErr)
		}
		pitchCurve = mergeManualPitchCurve(pitchCurve, constrainManualPitchContour(manualContour), manualPitch.Mode)
	}
	intonationStrength := rendererIntonationStrength(cfg, automaticPitchCurve)
	return synthesisPitch{Curve: pitchCurve, Automatic: automaticPitchCurve, Apply: applyPitch, RendererStrength: intonationStrength}, nil
}
