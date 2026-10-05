package native

import (
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"sort"

	"utautts/internal/appinfo"
	"utautts/internal/frontend"
	"utautts/internal/neural"
	"utautts/internal/openutau"
	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

type Config struct {
	VoiceDir            string   `json:"voice_dir"`
	Renderer            string   `json:"renderer"`
	WorldlineBridgePath string   `json:"worldline_bridge_path"`
	OpenJTalkPath       string   `json:"openjtalk_path"`
	OpenJTalkDictionary string   `json:"openjtalk_dictionary"`
	RendererDirectories []string `json:"renderer_directories,omitempty"`
	ModelDirectories    []string `json:"model_directories,omitempty"`
}

type Engine struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     Config
	voicebanks *voicebank.Library
	catalog    *plugin.Catalog
	synth      *synth.Service
}

func New(config Config) (*Engine, error) {
	config.VoiceDir = voicebank.ResolveDirectory(config.VoiceDir)
	engine := &Engine{config: config, voicebanks: voicebank.NewLibrary(config.VoiceDir)}
	engine.ctx, engine.cancel = context.WithCancel(context.Background())
	runtime, err := synth.NewRuntime(synth.RuntimeConfig{
		Renderer: config.Renderer, WorldlineBridgePath: config.WorldlineBridgePath,
		OpenJTalkPath: config.OpenJTalkPath, OpenJTalkDictionary: config.OpenJTalkDictionary,
		RendererDirectories: config.RendererDirectories, ModelDirectories: config.ModelDirectories,
	}, nativeVoicebankResolver{library: engine.voicebanks})
	if err != nil {
		engine.cancel()
		return nil, err
	}
	engine.config.Renderer, engine.catalog, engine.synth = runtime.Renderer, runtime.Catalog, runtime.Service
	if err := engine.reload(); err != nil {
		engine.cancel()
		return nil, fmt.Errorf("load voicebanks: %w", err)
	}
	// wasm では初回推論が重く起動を遅らせるため、warmUp は行わない。
	if goruntime.GOOS != "js" {
		go engine.warmUp()
	}
	return engine, nil
}

// 初回操作の待ち時間を減らすため、モデルと辞書を事前に準備する。失敗しても起動は続ける。
func (e *Engine) warmUp() {
	modelID := ""
	if e.catalog != nil && len(e.catalog.Models) > 0 {
		modelID = e.catalog.Models[0].ID
	}
	_, _, _ = e.synth.PredictProsodyContext(e.ctx, synth.Request{
		Text: "あ", Language: "ja", ModelID: modelID,
	})
}

func NewJSON(data []byte) (*Engine, error) {
	var config Config
	if len(data) != 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("decode native config: %w", err)
		}
	}
	return New(config)
}

func (e *Engine) Call(method string, requestJSON []byte) ([]byte, error) {
	var result any
	var err error
	switch method {
	case "shutdown":
		e.cancel()
		_ = render.CloseProviderSessions()
		_ = neural.CloseSessions()
		result = map[string]bool{"ok": true}
	case "health":
		result = map[string]any{"status": "ok", "engine": e.config.Renderer, "version": appinfo.Version()}
	case "voicebanks":
		result = map[string]any{"voicebanks": e.voicebankList()}
	case "reloadVoicebanks":
		err = e.reload()
		result = map[string]any{"voicebanks": e.voicebankList()}
	case "models":
		result = map[string]any{"models": e.models()}
	case "renderers":
		result = map[string]any{
			"default_renderer": e.config.Renderer, "renderers": e.catalog.Renderers,
			"availability": e.synth.RendererAvailability(),
			"problems":     e.catalog.Problems,
			"resamplers":   e.catalog.Resamplers, "wavtools": e.catalog.Wavtools,
		}
	case "analyze":
		result, err = e.analyze(requestJSON)
	case "predictProsody":
		result, err = e.predictProsody(requestJSON)
	case "synthesize":
		result, err = e.synthesize(requestJSON)
	case "writeExo":
		result, err = e.writeExo(requestJSON)
	case "exportUstx":
		result, err = e.exportUstx(requestJSON)
	case "writeSidecars":
		result, err = e.writeSidecars(requestJSON)
	case "installVoicebank":
		result, err = e.installVoicebank(requestJSON)
	default:
		err = fmt.Errorf("unknown native method %q", method)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

type nativeVoicebankResolver struct {
	library *voicebank.Library
}

func (r nativeVoicebankResolver) Resolve(id string) (string, bool) {
	summary, ok := r.library.Resolve(id)
	return summary.Path, ok
}

func (e *Engine) reload() error {
	if err := e.voicebanks.Reload(); err != nil {
		return err
	}
	// 音源パス変更時にデコード済みWAVやBankキャッシュが残るのを防ぐ。
	tts.ClearCaches()
	return nil
}

func (e *Engine) voicebankList() []map[string]any {
	items := e.voicebanks.List()
	list := make([]map[string]any, 0, len(items))
	for _, libraryItem := range items {
		id, item := libraryItem.ID, libraryItem.Summary
		presentation, _ := voicebank.LoadPresentation(item)
		entry := map[string]any{
			"id":          id,
			"name":        item.Name,
			"path":        item.Path,
			"kind":        item.Kind,
			"image_path":  item.ImagePath,
			"readme_path": item.ReadmePath,
			"readme_text": presentation.ReadmeText,
		}
		if bank, err := voicebank.Load(item.Path); err == nil {
			capabilities := bank.AliasCapabilities()
			language, phonemizer := bank.SuggestedLanguage()
			entry["suggested_language"] = language
			entry["suggested_phonemizer"] = phonemizer
			entry["types"] = bank.SubbankOptions()
			entry["alias_counts"] = capabilities.Counts
			entry["vcv_contexts"] = capabilities.VCVContexts
			entry["vc_contexts"] = capabilities.VCContexts
			entry["has_vc"] = capabilities.HasVC
			entry["has_initial_vcv"] = capabilities.HasInitialVCV
			entry["has_n_context_vcv"] = capabilities.HasNContextVCV
		} else if item.Kind == "diffsinger" {
			entry["suggested_language"] = "ja"
			entry["suggested_phonemizer"] = "ja-kana"
		}
		list = append(list, entry)
	}
	sort.Slice(list, func(i, j int) bool { return list[i]["name"].(string) < list[j]["name"].(string) })
	return list
}

func (e *Engine) models() []plugin.Model {
	return append([]plugin.Model(nil), e.catalog.Models...)
}

func (e *Engine) analyze(data []byte) (any, error) {
	var request struct {
		Text        string                  `json:"text"`
		Language    string                  `json:"language"`
		Phonemizer  string                  `json:"phonemizer"`
		VoicebankID string                  `json:"voicebank_id"`
		Dictionary  []synth.DictionaryEntry `json:"dictionary"`
	}
	if err := json.Unmarshal(data, &request); err != nil || request.Text == "" {
		return nil, fmt.Errorf("text is required")
	}
	preview, err := e.synth.AnalyzeContext(e.ctx, synth.Request{
		Text: request.Text, Language: request.Language, Phonemizer: request.Phonemizer,
		VoicebankID: request.VoicebankID, Dictionary: request.Dictionary,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(preview.Morae))
	for index, mora := range preview.Morae {
		items = append(items, map[string]any{"position": index, "mora": mora.Text, "consonant": mora.Consonant, "vowel": mora.Vowel, "pause": mora.Pause})
	}
	return map[string]any{"reading": preview.Reading, "morae": items}, nil
}

type prosodyPreviewRequest struct {
	RequestID string `json:"request_id"`
	synth.Request
}

func (request prosodyPreviewRequest) synthRequest() synth.Request {
	return request.Request.Normalized()
}

func (e *Engine) predictProsody(data []byte) (any, error) {
	request := prosodyPreviewRequest{Request: synth.DefaultRequest()}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode prosody preview request: %w", err)
	}
	if request.Text == "" && request.Reading == "" && request.Kana == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	preview, _, err := e.synth.PredictProsodyContext(e.ctx, request.synthRequest())
	if err != nil {
		return nil, err
	}
	morae := make([]map[string]any, len(preview.Morae))
	for index, mora := range preview.Morae {
		morae[index] = map[string]any{"position": index, "mora": mora.Text, "pause": mora.Pause}
	}
	response := map[string]any{
		"request_id":            request.RequestID,
		"reading":               preview.Reading,
		"morae":                 morae,
		"features":              preview.Features,
		"mora_durations_ms":     preview.MoraDurationsMS,
		"mora_positions_ms":     preview.MoraPositionsMS,
		"pitch_points":          preview.PitchPoints,
		"prosody_model_applied": e.synth.ModelAvailable(request.ModelID),
	}
	if preview.FramePitchCurve != nil && preview.FramePitchCurve.FrameMS > 0 {
		response["frame_ms"] = preview.FramePitchCurve.FrameMS
		response["frame_pitch_cents"] = append([]float64(nil), preview.FramePitchCurve.Cents...)
	}
	return response, nil
}

func previewMorae(morae []frontend.Mora) []openutau.UtauTTSMora {
	result := make([]openutau.UtauTTSMora, len(morae))
	for index, mora := range morae {
		result[index] = openutau.UtauTTSMora{
			Position: index, Mora: mora.Text, Pause: mora.Pause,
			Consonant: mora.Consonant, Vowel: mora.Vowel,
		}
	}
	return result
}
