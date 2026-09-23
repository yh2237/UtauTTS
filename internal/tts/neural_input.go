package tts

import (
	"fmt"

	"utautts/internal/frontend"
	"utautts/internal/neural"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// neuralInputFromConfigは共通Configをprovider非依存のニューラル入力へ変換する。
// 発音解析とプロソディ予測はtts側で行い、provider実装はスコア構築とbridge呼び出しだけを担う。
func neuralInputFromConfig(cfg Config) (neural.Input, error) {
	language, phonemizer, reading, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return neural.Input{}, fmt.Errorf("phonemize neural input: %w", err)
	}
	japaneseSpeechPhones(morae)
	preview, err := PredictProsody(cfg)
	if err != nil {
		return neural.Input{}, fmt.Errorf("predict neural speech prosody: %w", err)
	}
	curve, err := neuralPitchCurve(cfg, reading, preview, morae)
	if err != nil {
		return neural.Input{}, err
	}
	automaticPitch := curve != nil && cfg.ManualPitch == nil && cfg.ManualPitchPath == ""
	return neural.Input{
		Context:         cfg.Context,
		VoicebankPath:   cfg.VoicebankPath,
		Text:            cfg.Text,
		Tone:            cfg.Tone,
		Language:        language,
		Phonemizer:      phonemizer,
		Reading:         reading,
		Morae:           morae,
		Features:        preview.Features,
		MoraDurationsMS: preview.MoraDurationsMS,
		PitchPoints:     preview.PitchPoints,
		PitchCurve:      curve,
		AutomaticPitch:  automaticPitch,
		PhoneWeights:    languagePhoneWeights(language, morae),
		ProviderOptions: cfg.ProviderOptions,
		Engine:          cfg.Engine,
	}, nil
}

// neuralPitchCurveは自動輪郭へ手動ピッチをマージしたprovider向けのピッチ曲線を返す。
// 先頭パディングの移動はprovider側で行う。
func neuralPitchCurve(cfg Config, reading string, preview *ProsodyPreview, morae []frontend.Mora) (*render.PitchCurve, error) {
	curve := cfg.PitchCurve
	if curve == nil {
		curve = preview.FramePitchCurve
	}
	timings := make([]prosody.MoraTiming, len(morae))
	cursor := 0.0
	for i, duration := range preview.MoraDurationsMS {
		timings[i] = prosody.MoraTiming{StartMS: cursor, DurationMS: duration}
		cursor += duration
	}
	manual := cfg.ManualPitch
	if manual == nil && cfg.ManualPitchPath != "" {
		loaded, err := prosody.LoadManualPitch(cfg.ManualPitchPath)
		if err != nil {
			return nil, err
		}
		manual = loaded
	}
	if manual != nil {
		if err := manual.Validate(); err != nil {
			return nil, err
		}
		if manual.Reading != "" && manual.Reading != reading {
			return nil, fmt.Errorf("manual pitch reading does not match synthesis reading")
		}
		contour, err := manual.Curve(morae, timings, cursor)
		if err != nil {
			return nil, err
		}
		curve = render.ConstrainPitchCurve(mergeManualPitchCurve(curve, contour, manual.Mode), 20, 8)
	}
	return curve, nil
}
