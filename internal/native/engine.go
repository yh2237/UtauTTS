package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"utautts/internal/appinfo"
	"utautts/internal/aviutl"
	"utautts/internal/frontend"
	"utautts/internal/openutau"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

type Config struct {
	VoiceDir            string   `json:"voice_dir"`
	Renderer            string   `json:"renderer"`
	WorldlinePath       string   `json:"worldline_path"`
	WorldlineBridgePath string   `json:"worldline_bridge_path"`
	OpenJTalkPath       string   `json:"openjtalk_path"`
	OpenJTalkDictionary string   `json:"openjtalk_dictionary"`
	RendererDirectories []string `json:"renderer_directories,omitempty"`
	ModelDirectories    []string `json:"model_directories,omitempty"`
}

type Engine struct {
	config     Config
	mu         sync.RWMutex
	voicebanks map[string]voicebank.Summary
	catalog    *plugin.Catalog
	synth      *synth.Service
}

func New(config Config) (*Engine, error) {
	config.VoiceDir = voicebank.ResolveDirectory(config.VoiceDir)
	catalog, err := plugin.DiscoverWithDefaults(config.RendererDirectories, config.ModelDirectories, render.IsKnownRenderer)
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}
	if renderer, ok := catalog.Renderer(config.Renderer); ok {
		config.Renderer = renderer.ID
	}
	engine := &Engine{config: config, voicebanks: make(map[string]voicebank.Summary), catalog: catalog}
	engine.synth = synth.NewService(catalog, config.Renderer, config.WorldlinePath, config.WorldlineBridgePath, config.OpenJTalkPath, config.OpenJTalkDictionary, nativeVoicebankResolver{engine: engine})
	if err := engine.reload(); err != nil {
		return nil, fmt.Errorf("load voicebanks: %w", err)
	}
	return engine, nil
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
			"resamplers": e.catalog.Resamplers, "wavtools": e.catalog.Wavtools,
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
	default:
		err = fmt.Errorf("unknown native method %q", method)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

type nativeVoicebankResolver struct {
	engine *Engine
}

func (r nativeVoicebankResolver) Resolve(id string) (string, bool) {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()
	if id != "" {
		summary, ok := r.engine.voicebanks[id]
		return summary.Path, ok
	}
	first := voicebank.DefaultSortedKey(r.engine.voicebanks)
	summary, ok := r.engine.voicebanks[first]
	return summary.Path, ok
}

func (e *Engine) reload() error {
	summaries, err := voicebank.Discover(e.config.VoiceDir)
	if err != nil && !errors.Is(err, voicebank.ErrNoOto) && !os.IsNotExist(err) {
		return err
	}
	next := make(map[string]voicebank.Summary, len(summaries))
	for _, summary := range summaries {
		next[filepath.Base(summary.Path)] = summary
	}
	e.mu.Lock()
	e.voicebanks = next
	e.mu.Unlock()
	tts.ClearCaches()
	return nil
}

func (e *Engine) voicebankList() []map[string]any {
	e.mu.RLock()
	list := make([]map[string]any, 0, len(e.voicebanks))
	for id, item := range e.voicebanks {
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
	e.mu.RUnlock()
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
	dictionary := synth.DictionaryMap(request.Dictionary)
	if request.Language == frontend.LanguageEnglish && request.VoicebankID != "" {
		e.mu.RLock()
		summary, found := e.voicebanks[request.VoicebankID]
		e.mu.RUnlock()
		if found {
			bankDictionary, _, loadErr := voicebank.LoadARPAsingDictionary(summary.Path)
			if loadErr != nil {
				return nil, loadErr
			}
			for key, value := range bankDictionary {
				if dictionary[key] == "" {
					dictionary[key] = value
				}
			}
		}
	}
	preview, err := tts.PredictProsody(tts.Config{
		Text: request.Text, Language: request.Language, Phonemizer: request.Phonemizer,
		Dictionary: dictionary, OpenJTalkPath: e.config.OpenJTalkPath,
		OpenJTalkDictionaryPath: e.config.OpenJTalkDictionary,
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
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	Kana       string `json:"kana"`
	Reading    string `json:"reading"`
	Language   string `json:"language"`
	Phonemizer string `json:"phonemizer"`
	ModelID    string `json:"model_id"`
	Renderer   string `json:"renderer"`

	MoraDurationMS     float64                 `json:"mora_duration_ms"`
	PauseDurationMS    float64                 `json:"pause_duration_ms"`
	MoraDurationsMS    []float64               `json:"mora_durations_ms"`
	IntonationStrength float64                 `json:"intonation_strength"`
	ApplyPitch         bool                    `json:"apply_pitch"`
	Dictionary         []synth.DictionaryEntry `json:"dictionary"`
}

func (e *Engine) predictProsody(data []byte) (any, error) {
	var request prosodyPreviewRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode prosody preview request: %w", err)
	}
	if request.Text == "" && request.Reading == "" && request.Kana == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	reading := request.Reading
	if reading == "" {
		reading = request.Kana
	}
	preview, _, err := e.synth.PredictProsody(synth.Request{
		Text: request.Text, Reading: reading, Language: request.Language, Phonemizer: request.Phonemizer, Dictionary: request.Dictionary,
		ModelID: request.ModelID, Renderer: request.Renderer,
		MoraDurationMS: request.MoraDurationMS, PauseDurationMS: request.PauseDurationMS,
		MoraDurationsMS: request.MoraDurationsMS, IntonationStrength: request.IntonationStrength,
		ApplyPitch: request.ApplyPitch,
	})
	if err != nil {
		return nil, err
	}
	morae := make([]map[string]any, len(preview.Morae))
	for index, mora := range preview.Morae {
		morae[index] = map[string]any{"position": index, "mora": mora.Text, "pause": mora.Pause}
	}
	return map[string]any{
		"request_id":            request.RequestID,
		"reading":               preview.Reading,
		"morae":                 morae,
		"features":              preview.Features,
		"mora_durations_ms":     preview.MoraDurationsMS,
		"mora_positions_ms":     preview.MoraPositionsMS,
		"pitch_points":          preview.PitchPoints,
		"prosody_model_applied": e.synth.ModelAvailable(request.ModelID),
	}, nil
}

type synthesizeRequest struct {
	Text, Reading, Language, Phonemizer, VoicebankID, Tone, Color, ModelID, Renderer, Resampler, Wavtool, OutputPath string
	AliasPolicy                                                                                                      voicebank.AliasPolicy
	AcousticMode                                                                                                     string
	MoraDurationMS, PauseDurationMS, LeadingPreutteranceMS, IntonationStrength                                       float64
	MoraDurationsMS                                                                                                  []float64
	ApplyPitch                                                                                                       bool
	ManualPitch                                                                                                      *prosody.ManualPitchFile
	Dictionary                                                                                                       []synth.DictionaryEntry
	ResamplerExpressions                                                                                             []render.ResamplerExpression
}

func (r *synthesizeRequest) UnmarshalJSON(data []byte) error {
	type wire struct {
		Text                  string                       `json:"text"`
		Kana                  string                       `json:"kana"`
		Reading               string                       `json:"reading"`
		Language              string                       `json:"language"`
		Phonemizer            string                       `json:"phonemizer"`
		VoicebankID           string                       `json:"voicebank_id"`
		Tone                  string                       `json:"tone"`
		Color                 string                       `json:"color"`
		ModelID               string                       `json:"model_id"`
		Renderer              string                       `json:"renderer"`
		Resampler             string                       `json:"resampler"`
		Wavtool               string                       `json:"wavtool"`
		AliasPolicy           voicebank.AliasPolicy        `json:"alias_policy"`
		AcousticMode          string                       `json:"acoustic_mode"`
		OutputPath            string                       `json:"output_path"`
		MoraDurationMS        float64                      `json:"mora_duration_ms"`
		PauseDurationMS       float64                      `json:"pause_duration_ms"`
		LeadingPreutteranceMS float64                      `json:"leading_preutterance_ms"`
		MoraDurationsMS       []float64                    `json:"mora_durations_ms"`
		IntonationStrength    float64                      `json:"intonation_strength"`
		ApplyPitch            bool                         `json:"apply_pitch"`
		ManualPitch           *prosody.ManualPitchFile     `json:"manual_pitch"`
		Dictionary            []synth.DictionaryEntry      `json:"dictionary"`
		ResamplerExpressions  []render.ResamplerExpression `json:"resampler_expressions"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	reading := value.Reading
	if reading == "" {
		reading = value.Kana
	}
	*r = synthesizeRequest{Text: value.Text, Reading: reading, Language: value.Language, Phonemizer: value.Phonemizer, VoicebankID: value.VoicebankID, Tone: value.Tone, Color: value.Color, ModelID: value.ModelID, Renderer: value.Renderer, Resampler: value.Resampler, Wavtool: value.Wavtool, AliasPolicy: value.AliasPolicy, AcousticMode: value.AcousticMode, OutputPath: value.OutputPath, MoraDurationMS: value.MoraDurationMS, PauseDurationMS: value.PauseDurationMS, LeadingPreutteranceMS: value.LeadingPreutteranceMS, MoraDurationsMS: value.MoraDurationsMS, IntonationStrength: value.IntonationStrength, ApplyPitch: value.ApplyPitch, ManualPitch: value.ManualPitch, Dictionary: value.Dictionary, ResamplerExpressions: value.ResamplerExpressions}
	return nil
}

func (e *Engine) synthesize(data []byte) (any, error) {
	var request synthesizeRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode synthesis request: %w", err)
	}
	if request.Text == "" && request.Reading == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	result, err := e.synth.Synthesize(synth.Request{
		Text: request.Text, Reading: request.Reading, Language: request.Language, Phonemizer: request.Phonemizer, VoicebankID: request.VoicebankID,
		Tone: request.Tone, Color: request.Color, ModelID: request.ModelID, Renderer: request.Renderer,
		Resampler: request.Resampler, Wavtool: request.Wavtool,
		AliasPolicy: request.AliasPolicy, AcousticMode: request.AcousticMode,
		Dictionary:     request.Dictionary,
		MoraDurationMS: request.MoraDurationMS, PauseDurationMS: request.PauseDurationMS,
		LeadingPreutteranceMS: request.LeadingPreutteranceMS,
		MoraDurationsMS:       request.MoraDurationsMS, IntonationStrength: request.IntonationStrength,
		ApplyPitch: request.ApplyPitch, ManualPitch: request.ManualPitch,
		ResamplerExpressions: request.ResamplerExpressions,
	})
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, err
	}
	if err := synth.WriteFiles(outputPath, result, synth.ExportOptions{}); err != nil {
		return nil, err
	}
	return map[string]any{
		"output_path":           outputPath,
		"reading":               result.Plan.Reading,
		"duration_ms":           result.DurationMS,
		"leading_margin_ms":     result.Plan.LeadingMarginMS,
		"lab":                   result.Lab,
		"unit_count":            len(result.Plan.Units),
		"engine":                result.RendererID,
		"mora_durations_ms":     result.MoraDurationsMS,
		"mora_positions_ms":     result.MoraPositionsMS,
		"pitch_points":          result.PitchPoints,
		"prosody_model_applied": e.synth.ModelAvailable(request.ModelID),
	}, nil
}

func (e *Engine) writeSidecars(data []byte) (any, error) {
	var request struct {
		WAVPath   string `json:"wav_path"`
		Text      string `json:"text"`
		Lab       string `json:"lab"`
		Encoding  string `json:"encoding"`
		WriteText bool   `json:"write_text"`
		WriteLab  bool   `json:"write_lab"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode sidecar request: %w", err)
	}
	if request.WAVPath == "" {
		return nil, fmt.Errorf("wav_path is required")
	}
	if err := synth.WriteSidecars(request.WAVPath, synth.ExportOptions{
		WriteText: request.WriteText, WriteLab: request.WriteLab,
		TextEncoding: request.Encoding, Text: request.Text,
	}, request.Lab); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

func (e *Engine) writeExo(data []byte) (any, error) {
	var request struct {
		OutputPath string   `json:"output_path"`
		Files      []string `json:"files"`
		FrameRate  int      `json:"frame_rate"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode write exo request: %w", err)
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	if len(request.Files) == 0 {
		return nil, fmt.Errorf("files are required")
	}
	if request.FrameRate <= 0 {
		request.FrameRate = 60
	}
	for _, file := range request.Files {
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("WAV file not found: %s", file)
		}
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	if err := aviutl.WriteExo(outputPath, request.Files, request.FrameRate); err != nil {
		return nil, err
	}
	return map[string]any{"exo_path": outputPath}, nil
}

// exportUstx writes the current project parameters to an OpenUtau USTX file.
func (e *Engine) exportUstx(data []byte) (any, error) {
	var request struct {
		OutputPath string          `json:"output_path"`
		Project    json.RawMessage `json:"project"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode export ustx request: %w", err)
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	if len(request.Project) == 0 {
		return nil, fmt.Errorf("project is required")
	}
	project, err := openutau.ParseUtauTTSProject(request.Project)
	if err != nil {
		return nil, err
	}
	options := openutau.ExportOptions{}
	options.Curves = e.enrichAndCurves(project)
	output, err := openutau.ExportUSTX(project, options)
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputPath, output, 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"ustx_path": outputPath}, nil
}

// enrichAndCurves prepares a project for USTX export:
//   - utterances without a cached analysis (no reading/morae) are analyzed
//     on the fly so the export does not silently drop them
//   - utterances with a prosody model get their frame-level intonation
//     contour recomputed for smooth 10ms pitch curves
//
// Both jobs share one PredictProsody pass per utterance. Entries for
// utterances without a contour stay nil (the exporter falls back to
// mora-level data).
func (e *Engine) enrichAndCurves(project *openutau.UtauTTSProject) []openutau.FrameCurve {
	curves := make([]openutau.FrameCurve, len(project.Utterances))
	for index := range project.Utterances {
		utterance := &project.Utterances[index]
		if utterance.Text == "" && utterance.AnalysisCache.Reading == "" {
			continue
		}
		needsAnalysis := len(utterance.AnalysisCache.Morae) == 0
		needsCurve := utterance.ModelID != "" && !needsAnalysis
		if !needsAnalysis && !needsCurve {
			continue
		}
		strength := utterance.Intonation
		if strength <= 0 {
			strength = 1
		}
		renderer := utterance.RendererID
		if renderer == "" {
			renderer = "waveform"
		}
		preview, _, err := e.synth.PredictProsody(synth.Request{
			Text:               utterance.Text,
			Kana:               utterance.AnalysisCache.Reading,
			ModelID:            utterance.ModelID,
			Renderer:           renderer,
			MoraDurationMS:     utterance.MoraDurationMS,
			PauseDurationMS:    utterance.PauseDurationMS,
			MoraDurationsMS:    utterance.MoraDurationsMS,
			IntonationStrength: strength,
			ApplyPitch:         true,
		})
		if err != nil || preview == nil {
			continue
		}
		if needsAnalysis {
			utterance.AnalysisCache.Reading = preview.Reading
			utterance.AnalysisCache.Morae = previewMorae(preview.Morae)
		}
		if preview.FramePitchCurve != nil {
			curves[index] = openutau.FrameCurve{
				FrameMS: preview.FramePitchCurve.FrameMS,
				Cents:   preview.FramePitchCurve.Cents,
			}
		}
	}
	return curves
}

// previewMorae converts a prosody preview mora list into the project format.
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
