package api

import (
	"archive/zip"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/diffsinger"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/sidecar"
	"utautts/internal/synth"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

//go:embed ui/index.html
var uiFiles embed.FS

var uiFS = func() fs.FS {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	return sub
}()

const (
	maxJSONRequestBytes    = 1 << 20
	maxTextRunes           = 500
	maxBatchItems          = 16
	maxBatchWAVBytes       = 256 << 20
	maxManualPitchPoints   = 1000
	maxConcurrentSynthesis = 4
	maxConcurrentBatches   = 2
)

type Config struct {
	VoiceDir, Renderer                            string
	WorldlinePath, WorldlineBridgePath            string
	OpenJTalkPath, OpenJTalkDictionary, AuthToken string
	AllowVoicebankRegistration                    bool
	RendererDirectories, ModelDirectories         []string
}

type Voicebank struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	Path            string                      `json:"path"`
	Kind            string                      `json:"kind,omitempty"`
	Types           []voicebank.SubbankOption   `json:"types,omitempty"`
	OtoFileCount    int                         `json:"oto_file_count"`
	PhonemeCount    int                         `json:"phoneme_count"`
	DiagnosticCount int                         `json:"diagnostic_count"`
	AliasCounts     map[voicebank.AliasKind]int `json:"alias_counts"`
	VCVContexts     map[string]int              `json:"vcv_contexts"`
	VCContexts      map[string]int              `json:"vc_contexts"`
	HasVC           bool                        `json:"has_vc"`
	HasInitialVCV   bool                        `json:"has_initial_vcv"`
	HasNContextVCV  bool                        `json:"has_n_context_vcv"`
}

type Server struct {
	mu                  sync.RWMutex
	voicebanks          map[string]Voicebank
	synthesisSem        chan struct{}
	batchSem            chan struct{}
	renderer            string
	worldlinePath       string
	worldlineBridgePath string
	openJTalkPath       string
	openJTalkDictionary string
	voiceDir            string
	authToken           string
	allowRegistration   bool
	catalog             *plugin.Catalog
}

type apiVoicebankResolver struct {
	server *Server
}

func (r apiVoicebankResolver) Resolve(id string) (string, bool) {
	voicebank, ok := r.server.resolveVoicebank(id)
	return voicebank.Path, ok
}

func New(config Config) (*Server, error) {
	voiceDir := voicebank.ResolveDirectory(config.VoiceDir)
	catalog, err := plugin.DiscoverWithDefaults(config.RendererDirectories, config.ModelDirectories, render.IsKnownRenderer)
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}
	if renderer, ok := catalog.Renderer(config.Renderer); ok {
		config.Renderer = renderer.ID
	}
	srv := &Server{
		voicebanks:   map[string]Voicebank{},
		synthesisSem: make(chan struct{}, maxConcurrentSynthesis),
		batchSem:     make(chan struct{}, maxConcurrentBatches),
		renderer:     config.Renderer, worldlinePath: config.WorldlinePath, worldlineBridgePath: config.WorldlineBridgePath, voiceDir: voiceDir,
		openJTalkPath: config.OpenJTalkPath, openJTalkDictionary: config.OpenJTalkDictionary,
		authToken: config.AuthToken, allowRegistration: config.AllowVoicebankRegistration,
		catalog: catalog,
	}
	if err := srv.loadVoiceDirectory(); err != nil {
		return nil, fmt.Errorf("load voicebanks from %s: %w", voiceDir, err)
	}
	if len(srv.voicebanks) == 0 {
		log.Printf("warning: no voicebanks found in %s", voiceDir)
	}

	return srv, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/voicebanks", s.handleListVoicebanks)
	mux.HandleFunc("POST /api/voicebanks", s.handleRegisterVoicebank)
	mux.HandleFunc("POST /api/voicebanks/reload", s.handleReloadVoicebanks)
	mux.HandleFunc("POST /api/synthesize/audio", s.handleSynthesizeAudio)
	mux.HandleFunc("POST /api/synthesize/label", s.handleSynthesizeLabel)
	mux.HandleFunc("POST /api/synthesize/batch", s.handleSynthesizeBatch)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/renderers", s.handleRenderers)
	mux.HandleFunc("POST /api/analyze", s.handleAnalyze)
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	})
	mux.Handle("GET /", http.FileServer(http.FS(uiFS)))
	return s.authenticate(mux)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if s.authToken != "" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		if s.authToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		authorization := r.Header.Get("Authorization")
		if len(authorization) <= len(prefix) || authorization[:len(prefix)] != prefix || !tokenEqual(authorization[len(prefix):], s.authToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func tokenEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (s *Server) loadVoiceDirectory() error {
	summaries, err := voicebank.Discover(s.voiceDir)
	if err != nil && !errors.Is(err, voicebank.ErrNoOto) && !os.IsNotExist(err) {
		s.mu.Lock()
		s.voicebanks = map[string]Voicebank{}
		s.mu.Unlock()
		return err
	}
	next := make(map[string]Voicebank, len(summaries))
	for _, summary := range summaries {
		id := filepath.Base(summary.Path)
		item := Voicebank{ID: id, Name: summary.Name, Path: summary.Path, Kind: summary.Kind}
		if inspected, inspectErr := inspectVoicebank(summary.Path); inspectErr != nil {
			log.Printf("voicebank metadata: %s: %v", summary.Path, inspectErr)
		} else {
			item = inspected
		}
		next[id] = item
		log.Printf("voicebank: %s (%s)", summary.Name, id)
	}
	s.mu.Lock()
	s.voicebanks = next
	s.mu.Unlock()
	tts.ClearCaches()
	return nil
}

func inspectVoicebank(path string) (Voicebank, error) {
	if diffsinger.IsSinger(path) {
		singer, err := diffsinger.Load(path)
		if err != nil {
			return Voicebank{}, err
		}
		summary, _ := voicebank.InspectSinger(path)
		return Voicebank{
			ID: filepath.Base(singer.Root), Name: summary.Name, Path: singer.Root,
			Kind: diffsinger.SingerKind, PhonemeCount: len(singer.Tokens),
		}, nil
	}
	bank, err := voicebank.Load(path)
	if err != nil {
		return Voicebank{}, err
	}
	capabilities := bank.AliasCapabilities()
	return Voicebank{
		ID:              filepath.Base(bank.Root),
		Name:            bank.Name,
		Path:            bank.Root,
		Kind:            "utau",
		Types:           bank.SubbankOptions(),
		OtoFileCount:    len(bank.OtoFiles),
		PhonemeCount:    bank.EntryCount(),
		DiagnosticCount: len(bank.Diagnostics),
		AliasCounts:     capabilities.Counts,
		VCVContexts:     capabilities.VCVContexts,
		VCContexts:      capabilities.VCContexts,
		HasVC:           capabilities.HasVC,
		HasInitialVCV:   capabilities.HasInitialVCV,
		HasNContextVCV:  capabilities.HasNContextVCV,
	}, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	engine := s.renderer
	if engine == "" {
		engine = s.pluginCatalog().DefaultRenderer()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "engine": engine})
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"models": s.pluginCatalog().Models})
}

func (s *Server) handleRenderers(w http.ResponseWriter, _ *http.Request) {
	catalog := s.pluginCatalog()
	writeJSON(w, http.StatusOK, map[string]any{
		"default_renderer": s.renderer, "renderers": catalog.Renderers,
		"resamplers": catalog.Resamplers, "wavtools": catalog.Wavtools,
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text       string                  `json:"text"`
		Dictionary []synth.DictionaryEntry `json:"dictionary"`
	}
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	if request.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}
	if len([]rune(request.Text)) > maxTextRunes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("text is limited to %d characters", maxTextRunes)})
		return
	}
	reading, err := tts.ConvertToReadingContext(r.Context(), request.Text, synth.DictionaryMap(request.Dictionary), openjtalk.Config{
		HelperPath: s.openJTalkPath, DictionaryPath: s.openJTalkDictionary,
	})
	if err != nil {
		writeJSON(w, contextErrorStatus(err, http.StatusUnprocessableEntity), map[string]string{"error": err.Error()})
		return
	}
	morae, err := frontend.ParseKana(reading)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(morae))
	for index, mora := range morae {
		items = append(items, map[string]any{"position": index, "mora": mora.Text, "consonant": mora.Consonant, "vowel": mora.Vowel, "pause": mora.Pause})
	}
	writeJSON(w, http.StatusOK, map[string]any{"reading": reading, "morae": items})
}

func (s *Server) handleListVoicebanks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"voicebanks": s.voicebankList()})
}

func (s *Server) voicebankList() []Voicebank {
	s.mu.RLock()
	list := make([]Voicebank, 0, len(s.voicebanks))
	for _, vb := range s.voicebanks {
		list = append(list, vb)
	}
	s.mu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

func (s *Server) handleReloadVoicebanks(w http.ResponseWriter, _ *http.Request) {
	if err := s.loadVoiceDirectory(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"voicebanks": s.voicebankList()})
}

func (s *Server) handleRegisterVoicebank(w http.ResponseWriter, r *http.Request) {
	if !s.allowRegistration {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "voicebank registration is disabled"})
		return
	}
	var request struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	path, err := pathWithin(s.voiceDir, request.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	vb, err := inspectVoicebank(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if request.Name != "" {
		vb.Name = request.Name
	}
	s.mu.Lock()
	s.voicebanks[vb.ID] = vb
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, vb)
}

func pathWithin(root, candidate string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve voicebank root: %w", err)
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve voicebank path: %w", err)
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("voicebank path must be inside %s", root)
	}
	return candidate, nil
}

type SynthesisRequest struct {
	Kana                  string                       `json:"kana"`
	Reading               string                       `json:"reading"`
	Text                  string                       `json:"text"`
	Language              string                       `json:"language"`
	Phonemizer            string                       `json:"phonemizer"`
	VoicebankID           string                       `json:"voicebank_id"`
	Tone                  string                       `json:"tone"`
	Color                 string                       `json:"color"`
	MoraDurationMS        float64                      `json:"mora_duration_ms"`
	PauseDurationMS       float64                      `json:"pause_duration_ms"`
	LeadingPreutteranceMS float64                      `json:"leading_preutterance_ms"`
	MoraDurationsMS       []float64                    `json:"mora_durations_ms"`
	IntonationStrength    float64                      `json:"intonation_strength"`
	ApplyPitch            bool                         `json:"apply_pitch"`
	ManualPitch           *prosody.ManualPitchFile     `json:"manual_pitch"`
	ModelID               string                       `json:"model_id"`
	Renderer              string                       `json:"renderer"`
	Resampler             string                       `json:"resampler"`
	Wavtool               string                       `json:"wavtool"`
	AliasPolicy           voicebank.AliasPolicy        `json:"alias_policy"`
	AcousticMode          string                       `json:"acoustic_mode"`
	Dictionary            []synth.DictionaryEntry      `json:"dictionary"`
	ResamplerExpressions  []render.ResamplerExpression `json:"resampler_expressions"`
}

func (s *Server) handleSynthesizeAudio(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-UtauTTS-Engine", result.RendererID)
	w.Header().Set("X-UtauTTS-Reading", result.Plan.Reading)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(audio.PCMToWavBytes(result.Audio)); err != nil {
		log.Printf("write synthesis response: %v", err)
	}
}

func (s *Server) handleSynthesizeLabel(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-UtauTTS-Engine", result.RendererID)
	w.Header().Set("X-UtauTTS-Reading", result.Plan.Reading)
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, result.Lab); err != nil {
		log.Printf("write label response: %v", err)
	}
}

func (s *Server) handleSynthesizeBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WriteText    bool   `json:"write_text"`
		WriteLab     bool   `json:"write_lab"`
		TextEncoding string `json:"text_encoding"`
		Items        []struct {
			Name    string           `json:"name"`
			Request SynthesisRequest `json:"request"`
		} `json:"items"`
	}
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	if len(request.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}
	if len(request.Items) > maxBatchItems {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("batch supports at most %d items", maxBatchItems)})
		return
	}
	if request.WriteText {
		if _, err := sidecar.TextBytes("", request.TextEncoding); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	names := make([]string, len(request.Items))
	seenNames := make(map[string]struct{}, len(request.Items))
	for index, item := range request.Items {
		name := filepath.Base(item.Name)
		if name == "." || name == "" || name == ".." {
			name = fmt.Sprintf("utterance-%d.wav", index+1)
		}
		if !strings.EqualFold(filepath.Ext(name), ".wav") {
			name += ".wav"
		}
		if _, exists := seenNames[name]; exists {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("duplicate batch filename %q", name)})
			return
		}
		seenNames[name] = struct{}{}
		names[index] = name
	}
	if s.batchSem != nil {
		select {
		case s.batchSem <- struct{}{}:
			defer func() { <-s.batchSem }()
		case <-r.Context().Done():
			writeJSON(w, http.StatusRequestTimeout, map[string]string{"error": r.Context().Err().Error()})
			return
		}
	}
	output, err := os.CreateTemp("", "utautts-batch-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	outputPath := output.Name()
	defer func() {
		_ = output.Close()
		_ = os.Remove(outputPath)
	}()
	archive := zip.NewWriter(output)
	totalWAVBytes := 0
	for index, item := range request.Items {
		result, status, err := s.synthesize(r.Context(), item.Request)
		if err != nil {
			_ = archive.Close()
			writeJSON(w, status, map[string]string{"error": fmt.Sprintf("item %d: %v", index+1, err)})
			return
		}
		name := names[index]
		wav := audio.PCMToWavBytes(result.Audio)
		totalWAVBytes += len(wav)
		if totalWAVBytes > maxBatchWAVBytes {
			_ = archive.Close()
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "batch audio exceeds 256 MiB"})
			return
		}
		entry, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if _, err := entry.Write(wav); err != nil {
			_ = archive.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if request.WriteText {
			text := item.Request.Text
			if text == "" {
				text = synthesisReading(item.Request)
			}
			data, err := sidecar.TextBytes(text, request.TextEncoding)
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			entry, err := archive.Create(base + ".txt")
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		if request.WriteLab {
			data, err := sidecar.LabBytes(result.Lab)
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
				return
			}
			entry, err := archive.Create(base + ".lab")
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	}
	if err := archive.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="utautts-audio.zip"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, output); err != nil {
		log.Printf("write batch response: %v", err)
	}
}

func (s *Server) synthesize(ctx context.Context, request SynthesisRequest) (*synth.Result, int, error) {
	if s.synthesisSem != nil {
		select {
		case s.synthesisSem <- struct{}{}:
		case <-ctx.Done():
			return nil, http.StatusRequestTimeout, ctx.Err()
		}
		defer func() { <-s.synthesisSem }()
	}
	if synthesisReading(request) == "" && request.Text == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("text or reading is required")
	}
	if len([]rune(request.Text)) > maxTextRunes || len([]rune(synthesisReading(request))) > maxTextRunes {
		return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("text and reading are limited to %d characters", maxTextRunes)
	}
	if request.MoraDurationMS < 0 || request.MoraDurationMS > 1000 || request.PauseDurationMS < 0 || request.PauseDurationMS > 3000 || request.LeadingPreutteranceMS < 0 || request.LeadingPreutteranceMS > 1000 {
		return nil, http.StatusBadRequest, fmt.Errorf("duration settings are outside the supported range")
	}
	if len(request.MoraDurationsMS) > maxTextRunes {
		return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("mora duration settings contain too many values")
	}
	for _, duration := range request.MoraDurationsMS {
		if duration < 0 || duration > 1000 {
			return nil, http.StatusBadRequest, fmt.Errorf("mora duration settings are outside the supported range")
		}
	}
	if request.IntonationStrength < 0 || request.IntonationStrength > render.MaxIntonationStrength {
		return nil, http.StatusBadRequest, fmt.Errorf("intonation_strength must be between 0 and %.0f", render.MaxIntonationStrength)
	}
	if request.ManualPitch != nil && len(request.ManualPitch.Points) > maxManualPitchPoints {
		return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("manual pitch supports at most %d points", maxManualPitchPoints)
	}
	if len(request.ResamplerExpressions) > maxTextRunes {
		return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("resampler_expressions contains too many values")
	}
	result, err := s.synthesisService().SynthesizeContext(ctx, synth.Request{
		Text: request.Text, Reading: synthesisReading(request), Language: request.Language, Phonemizer: request.Phonemizer, VoicebankID: request.VoicebankID,
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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, http.StatusRequestTimeout, err
		}
		if errors.Is(err, synth.ErrUnavailable) {
			return nil, http.StatusBadRequest, err
		}
		return nil, http.StatusUnprocessableEntity, err
	}
	return result, http.StatusOK, nil
}

func synthesisReading(request SynthesisRequest) string {
	if request.Reading != "" {
		return request.Reading
	}
	return request.Kana
}

func contextErrorStatus(err error, fallback int) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout
	}
	return fallback
}

func (s *Server) synthesisService() *synth.Service {
	return synth.NewService(s.pluginCatalog(), s.renderer, s.worldlinePath, s.worldlineBridgePath,
		s.openJTalkPath, s.openJTalkDictionary, apiVoicebankResolver{server: s})
}

func (s *Server) pluginCatalog() *plugin.Catalog {
	if s.catalog == nil {
		return &plugin.Catalog{}
	}
	return s.catalog
}

func (s *Server) resolveVoicebank(id string) (Voicebank, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id != "" {
		vb, ok := s.voicebanks[id]
		return vb, ok
	}
	first := voicebank.DefaultSortedKey(s.voicebanks)
	vb, ok := s.voicebanks[first]
	return vb, ok
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request must contain one JSON object")
		}
		return err
	}
	return nil
}

func jsonDecodeStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
