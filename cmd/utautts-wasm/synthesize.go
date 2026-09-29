//go:build js && wasm

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall/js"

	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	_ "utautts/internal/render/worldline"
	"utautts/internal/tts"
)

type synthesisResponse struct {
	Version    int     `json:"version"`
	OutputPath string  `json:"outputPath"`
	Reading    string  `json:"reading"`
	DurationMS float64 `json:"durationMS"`
	SampleRate int     `json:"sampleRate"`
	Channels   int     `json:"channels"`
	MoraCount  int     `json:"moraCount"`
}

// synthesize(options) は既定Rendererで音声を合成し、WAVを仮想FSへ書き出す。
// options = { text|kana, modelJSON|modelPath, voicebankPath, outputPath?, strength? }
func synthesize(this js.Value, args []js.Value) (result any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = errorJSON(recovered)
		}
	}()
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return errorJSON("synthesize requires an options object")
	}
	options := args[0]
	kana := optionString(options, "kana")
	text := optionString(options, "text")
	modelJSON := optionString(options, "modelJSON")
	modelPath := optionString(options, "modelPath")
	voicebankPath := optionString(options, "voicebankPath")
	outputPath := optionString(options, "outputPath")
	strength := optionNumber(options, "strength", 1)
	if voicebankPath == "" {
		return errorJSON("voicebankPath is required")
	}
	if kana == "" && text == "" {
		return errorJSON("text or kana is required")
	}
	var model *prosody.Model
	if modelPath != "" {
		loaded, err := prosody.LoadModel(modelPath)
		if err != nil {
			return errorJSON("load prosody model: " + err.Error())
		}
		model = loaded
	} else if modelJSON != "" {
		parsed, err := prosody.ParseModel([]byte(modelJSON))
		if err != nil {
			return errorJSON("parse prosody model: " + err.Error())
		}
		model = parsed
	} else {
		return errorJSON("modelJSON or modelPath is required")
	}

	cfg := tts.Config{
		Text:                 text,
		Reading:              kana,
		VoicebankPath:        voicebankPath,
		Language:             frontend.LanguageJapanese,
		Phonemizer:           frontend.PhonemizerJapanese,
		ProsodyModel:         model,
		IntonationStrength:   strength,
		ApplyPitch:           true,
		Renderer:             "utautts-world-phrase",
		RendererCapabilities: &plugin.Capabilities{FramePitch: true},
	}
	synthesis, err := tts.SynthesizeWithOptions(cfg, render.ProviderOptions{})
	if err != nil {
		return errorJSON("synthesize: " + err.Error())
	}
	if outputPath == "" {
		outputPath = "/out/utautts.wav"
	}
	if directory := filepath.Dir(outputPath); directory != "" && directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return errorJSON("create output directory: " + err.Error())
		}
	}
	if err := audio.WriteWav(outputPath, synthesis.Audio); err != nil {
		return errorJSON("write wav: " + err.Error())
	}

	response := synthesisResponse{
		Version:    version,
		OutputPath: outputPath,
		SampleRate: synthesis.Audio.SampleRate,
		Channels:   synthesis.Audio.Channels,
		MoraCount:  len(synthesis.MoraDurationsMS),
	}
	if synthesis.Plan != nil {
		response.Reading = synthesis.Plan.Reading
	}
	if synthesis.Audio.Channels > 0 && synthesis.Audio.SampleRate > 0 {
		frames := len(synthesis.Audio.Data) / synthesis.Audio.Channels
		response.DurationMS = float64(frames) / float64(synthesis.Audio.SampleRate) * 1000
	}
	data, err := json.Marshal(response)
	if err != nil {
		return errorJSON("encode synthesis response: " + err.Error())
	}
	return string(data)
}
