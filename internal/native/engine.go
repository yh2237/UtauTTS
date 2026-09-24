package native

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"utautts/internal/appinfo"
	"utautts/internal/aviutl"
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
	var request prosodyPreviewRequest
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

type synthesizeRequest struct {
	synth.Request
	OutputPath string `json:"output_path"`
}

func synthesisUnits(result *synth.Result) []map[string]any {
	if result == nil {
		return nil
	}
	synthesisPlan := result.RenderedPlan()
	if synthesisPlan == nil {
		return nil
	}
	units := make([]map[string]any, 0, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		renderStart := unit.NoteStartMS - math.Max(0, unit.EffectivePreutteranceMS) + synthesisPlan.LeadingMarginMS
		units = append(units, map[string]any{
			"unit_index":                    index,
			"position":                      unit.Position,
			"role":                          unit.Role,
			"mora":                          unit.Mora,
			"alias":                         unit.Alias,
			"source":                        unit.Source,
			"source_name":                   filepath.Base(unit.Source),
			"silent":                        unit.Silent,
			"note_start_ms":                 unit.NoteStartMS,
			"render_start_ms":               renderStart,
			"duration_ms":                   unit.DurationMS,
			"offset_ms":                     unit.OffsetMS,
			"consonant_ms":                  unit.ConsonantMS,
			"cutoff_ms":                     unit.CutoffMS,
			"preutterance_ms":               unit.PreutteranceMS,
			"overlap_ms":                    unit.OverlapMS,
			"effective_preutterance_ms":     unit.EffectivePreutteranceMS,
			"effective_consonant_ms":        unit.EffectiveConsonantMS,
			"effective_overlap_ms":          unit.EffectiveOverlapMS,
			"pitch_factor":                  unit.PitchFactor,
			"energy_factor":                 unit.EnergyFactor,
			"resampler_velocity":            unit.ResamplerVelocity,
			"resampler_volume":              unit.ResamplerVolume,
			"resampler_flags":               unit.ResamplerFlags,
			"resampler_modulation":          unit.ResamplerModulation,
			"resampler_tempo":               unit.ResamplerTempo,
			"resampler_velocity_override":   unit.ResamplerVelocityOverride,
			"resampler_volume_override":     unit.ResamplerVolumeOverride,
			"resampler_flags_override":      unit.ResamplerFlagsOverride,
			"resampler_modulation_override": unit.ResamplerModulationOverride,
			"resampler_tempo_override":      unit.ResamplerTempoOverride,
		})
	}
	return units
}

func pcmPeaks(data []int16, channels int, maximumPoints int) (mins, maxs []float64) {
	if channels <= 0 || len(data) == 0 {
		return nil, nil
	}
	frames := len(data) / channels
	if frames <= 0 {
		return nil, nil
	}
	points := frames
	if maximumPoints > 0 && points > maximumPoints {
		points = maximumPoints
	}
	mins = make([]float64, points)
	maxs = make([]float64, points)
	for point := 0; point < points; point++ {
		start := point * frames / points
		end := (point + 1) * frames / points
		if end <= start {
			end = start + 1
		}
		minimum, maximum := 1.0, -1.0
		for frame := start; frame < end && frame < frames; frame++ {
			for channel := 0; channel < channels; channel++ {
				sample := float64(data[frame*channels+channel]) / 32768.0
				if sample < minimum {
					minimum = sample
				}
				if sample > maximum {
					maximum = sample
				}
			}
		}
		mins[point] = minimum
		maxs[point] = maximum
	}
	return mins, maxs
}

func waveformPeaks(result *synth.Result, maximumPoints int) (mins, maxs []float64) {
	if result == nil || result.Audio == nil {
		return nil, nil
	}
	return pcmPeaks(result.Audio.Data, result.Audio.Channels, maximumPoints)
}

func (e *Engine) synthesize(data []byte) (any, error) {
	var request synthesizeRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode synthesis request: %w", err)
	}
	if request.Text == "" && request.ReadingOrKana() == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	result, err := e.synth.SynthesizeContext(e.ctx, request.Request.Normalized())
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
	renderedPlan := result.RenderedPlan()
	leadingMarginMS := 0.0
	if renderedPlan != nil {
		leadingMarginMS = renderedPlan.LeadingMarginMS
	}
	waveformMin, waveformMax := waveformPeaks(result, 2400)
	return map[string]any{
		"output_path":           outputPath,
		"reading":               result.Plan.Reading,
		"duration_ms":           result.DurationMS,
		"leading_margin_ms":     leadingMarginMS,
		"lab":                   result.Lab,
		"unit_count":            len(result.Plan.Units),
		"engine":                result.RendererID,
		"mora_durations_ms":     result.MoraDurationsMS,
		"mora_positions_ms":     result.MoraPositionsMS,
		"pitch_points":          result.PitchPoints,
		"units":                 synthesisUnits(result),
		"waveform_min":          waveformMin,
		"waveform_max":          waveformMax,
		"waveform_sample_rate":  result.Audio.SampleRate,
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

// exportUstxは現在のプロジェクト設定をOpenUtauのUSTXへ書き出す。
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

// enrichAndCurvesはUSTX出力前に解析と10msピッチ輪郭を補う。
// 輪郭を作れない発話はnilのままにし、出力側でモーラ値へ戻す。
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

// previewMoraeはプロソディプレビューのモーラ列をプロジェクト形式へ変換する。
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
