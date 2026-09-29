//go:build js && wasm

// Command utautts-wasm はUtauTTSのGoエンジンをWebAssemblyとして公開する。
package main

import (
	"encoding/json"
	"os"
	"syscall/js"

	"utautts/internal/frontend"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render/worldline"
	"utautts/internal/tts"
)

// versionはJS側との互換判定に使う公開APIのバージョン。
const version = 1

type moraView struct {
	Text  string `json:"text"`
	Vowel string `json:"vowel"`
	Pause bool   `json:"pause"`
}

type prosodyResponse struct {
	Version         int        `json:"version"`
	Reading         string     `json:"reading"`
	Morae           []moraView `json:"morae"`
	MoraDurationsMS []float64  `json:"moraDurationsMS"`
	MoraPositionsMS []float64  `json:"moraPositionsMS"`
	PitchPoints     []float64  `json:"pitchPoints"`
	FrameMS         float64    `json:"frameMS,omitempty"`
	FramePitchCents []float64  `json:"framePitchCents,omitempty"`
	TotalDurationMS float64    `json:"totalDurationMS"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	api := map[string]any{
		"version":        version,
		"predictProsody": js.FuncOf(predictProsody),
		"synthesize":     js.FuncOf(synthesize),
	}
	js.Global().Set("utauttsWasm", js.ValueOf(api))
	if os.Getenv("UTAUTTS_TTS_PROFILE") != "" {
		log := func(message string) {
			js.Global().Get("console").Call("log", "utautts "+message)
		}
		tts.Trace = log
		worldline.Trace = log
	}
	// wasmのランタイムを生かしたままJSからの呼び出しを待つ。
	keepAlive := make(chan struct{})
	<-keepAlive
}

// predictProsody(options) は抑揚プレビューをJSON文字列で返す。
// options = { text?, kana?, modelJSON, strength? }
// textはOpen JTalk wasmで解析し、kanaはOpen JTalkを介さず直接解析する。
func predictProsody(this js.Value, args []js.Value) (result any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = errorJSON(recovered)
		}
	}()
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return errorJSON("predictProsody requires an options object")
	}
	options := args[0]
	kana := optionString(options, "kana")
	text := optionString(options, "text")
	modelJSON := optionString(options, "modelJSON")
	modelPath := optionString(options, "modelPath")
	strength := optionNumber(options, "strength", 1)
	if modelJSON == "" && modelPath == "" {
		return errorJSON("modelJSON or modelPath is required")
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
	} else {
		parsed, err := prosody.ParseModel([]byte(modelJSON))
		if err != nil {
			return errorJSON("parse prosody model: " + err.Error())
		}
		model = parsed
	}

	cfg := tts.Config{
		Language:             frontend.LanguageJapanese,
		Phonemizer:           frontend.PhonemizerJapanese,
		ProsodyModel:         model,
		IntonationStrength:   strength,
		ApplyPitch:           true,
		Renderer:             "utautts-world-phrase",
		RendererCapabilities: &plugin.Capabilities{FramePitch: true},
	}
	if kana != "" {
		morae, err := frontend.ParseKana(kana)
		if err != nil {
			return errorJSON("parse kana: " + err.Error())
		}
		if len(morae) == 0 {
			return errorJSON("kana produced no morae")
		}
		cfg.Reading = kana
		// Open JTalkを使わないため、モデルが要求するモーラ数ぶんの空フレームを渡す。
		cfg.ProsodyFeatures = make([]prosody.FeatureFrame, len(morae))
	} else {
		cfg.Text = text
	}

	preview, err := tts.PredictProsody(cfg)
	if err != nil {
		return errorJSON("predict prosody: " + err.Error())
	}
	return encodeResponse(preview)
}

func optionString(options js.Value, name string) string {
	value := options.Get(name)
	if value.Type() != js.TypeString {
		return ""
	}
	return value.String()
}

func optionNumber(options js.Value, name string, fallback float64) float64 {
	value := options.Get(name)
	if value.Type() != js.TypeNumber {
		return fallback
	}
	return value.Float()
}

func encodeResponse(preview *tts.ProsodyPreview) string {
	response := prosodyResponse{
		Version:         version,
		Reading:         preview.Reading,
		Morae:           make([]moraView, len(preview.Morae)),
		MoraDurationsMS: preview.MoraDurationsMS,
		MoraPositionsMS: preview.MoraPositionsMS,
		PitchPoints:     preview.PitchPoints,
	}
	for index, mora := range preview.Morae {
		response.Morae[index] = moraView{Text: mora.Text, Vowel: mora.Vowel, Pause: mora.Pause}
	}
	if preview.FramePitchCurve != nil {
		response.FrameMS = preview.FramePitchCurve.FrameMS
		response.FramePitchCents = preview.FramePitchCurve.Cents
	}
	for index, position := range preview.MoraPositionsMS {
		if index < len(preview.MoraDurationsMS) {
			if end := position + preview.MoraDurationsMS[index]/2; end > response.TotalDurationMS {
				response.TotalDurationMS = end
			}
		}
	}
	data, err := json.Marshal(response)
	if err != nil {
		return errorJSON("encode prosody response: " + err.Error())
	}
	return string(data)
}

func errorJSON(value any) string {
	data, err := json.Marshal(errorResponse{Error: toString(value)})
	if err != nil {
		return `{"error":"unknown"}`
	}
	return string(data)
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case error:
		return typed.Error()
	default:
		return "unknown error"
	}
}
