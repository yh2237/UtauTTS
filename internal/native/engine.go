package native

import (
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"sort"
	"sync"

	"github.com/yh2237/UtauTTS/internal/appinfo"
	"github.com/yh2237/UtauTTS/internal/entryinput"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/openutau"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/tts"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

type Config struct {
	VoiceDir            string   `json:"voice_dir"`
	Renderer            string   `json:"renderer"`
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
	runtime    *synth.Runtime
}

func New(config Config) (*Engine, error) {
	config.VoiceDir = voicebank.ResolveDirectory(config.VoiceDir)
	engine := &Engine{config: config, voicebanks: voicebank.NewLibrary(config.VoiceDir)}
	engine.ctx, engine.cancel = context.WithCancel(context.Background())
	runtime, err := synth.NewRuntime(synth.RuntimeConfig{
		Renderer:      config.Renderer,
		OpenJTalkPath: config.OpenJTalkPath, OpenJTalkDictionary: config.OpenJTalkDictionary,
		RendererDirectories: config.RendererDirectories, ModelDirectories: config.ModelDirectories,
	}, nativeVoicebankResolver{library: engine.voicebanks})
	if err != nil {
		engine.cancel()
		return nil, err
	}
	engine.config.Renderer, engine.catalog, engine.synth = runtime.Renderer, runtime.Catalog, runtime.Service
	engine.runtime = runtime
	if err := engine.reload(); err != nil {
		engine.cancel()
		_ = runtime.Close()
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
		err = e.runtime.Close()
		result = map[string]bool{"ok": true}
	case "health":
		result = map[string]any{"status": "ok", "engine": e.config.Renderer, "version": appinfo.Version()}
	case "voicebanks":
		result = map[string]any{"voicebanks": e.voicebankList()}
	case "reloadVoicebanks":
		err = e.reload()
		result = map[string]any{"voicebanks": e.voicebankList()}
	case "models":
		result = map[string]any{"models": e.models(), "retired_models": synth.RetiredModelIDs()}
	case "referenceProviders":
		result = map[string]any{"reference_providers": e.synth.ReferenceProviders()}
	case "renderers":
		result = map[string]any{
			"default_renderer": e.config.Renderer, "renderers": e.catalog.Renderers,
			"availability": e.synth.RendererAvailability(),
			"problems":     e.catalog.Problems,
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
	case "referenceIndexes":
		result = tts.ReferenceIndexProgress()
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
	return e.runtime.Reload(func() error {
		render.ClearWAVCache()
		return e.voicebanks.Reload()
	})
}

func (e *Engine) voicebankList() []map[string]any {
	items := e.voicebanks.List()
	entries := make([]map[string]any, len(items))
	// 音源ごとにoto.iniを読むので、音源ごとに並べて読む。
	var wait sync.WaitGroup
	next := make(chan int)
	for range min(goruntime.GOMAXPROCS(0), len(items)) {
		wait.Go(func() {
			for index := range next {
				entries[index] = e.voicebankEntry(items[index])
			}
		})
	}
	for index := range items {
		next <- index
	}
	close(next)
	wait.Wait()
	list := append(make([]map[string]any, 0, len(entries)), entries...)
	sort.Slice(list, func(i, j int) bool { return list[i]["name"].(string) < list[j]["name"].(string) })
	return list
}

func (e *Engine) voicebankEntry(libraryItem voicebank.LibraryItem) map[string]any {
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
		if language == frontend.LanguageEnglish {
			e.synth.PrepareReferenceIndex(bank)
		}
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
	return entry
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
	entryinput.JSONRequest
	RequestID string `json:"request_id"`
}

func decodeProsodyPreviewRequest(data []byte) (prosodyPreviewRequest, error) {
	var request prosodyPreviewRequest
	if err := entryinput.DecodeRequestLenient(data, &request.JSONRequest); err != nil {
		return prosodyPreviewRequest{}, fmt.Errorf("decode prosody preview request: %w", err)
	}
	var envelope struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return prosodyPreviewRequest{}, fmt.Errorf("decode prosody preview request: %w", err)
	}
	request.RequestID = envelope.RequestID
	return request, nil
}

func (e *Engine) predictProsody(data []byte) (any, error) {
	request, err := decodeProsodyPreviewRequest(data)
	if err != nil {
		return nil, err
	}
	if request.Text == "" && request.ReadingOrKana() == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	preview, _, err := e.synth.PredictProsodyContext(e.ctx, request.InternalRequest())
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
