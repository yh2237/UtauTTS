package api

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yh2237/UtauIDX/audio"
	utautts "github.com/yh2237/UtauTTS"
	"github.com/yh2237/UtauTTS/internal/appinfo"
	"github.com/yh2237/UtauTTS/internal/entryinput"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/tts"
	"github.com/yh2237/UtauTTS/internal/voicebank"
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
	maxTextRunes           = entryinput.MaxTextRunes
	maxBatchItems          = 16
	maxBatchWAVBytes       = 256 << 20
	maxManualPitchPoints   = 1000
	maxConcurrentSynthesis = 4
	maxConcurrentBatches   = 2
	defaultQueueTimeout    = 30 * time.Second
)

type Config struct {
	VoiceDir                           string
	OpenJTalkPath, OpenJTalkDictionary string
	ModelDirectories                   []string
	// MaxConcurrentSynthesisは同時に実行する合成と読み解析の上限。0なら既定値。
	MaxConcurrentSynthesis int
	// MaxConcurrentBatchesは同時に実行する一括合成の上限。0なら既定値。
	MaxConcurrentBatches int
	// QueueTimeoutは実行枠が空くのを待つ上限。0なら既定値、負なら要求の切断まで待つ。
	QueueTimeout time.Duration
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
	synthesisWaiting    atomic.Int64
	batchWaiting        atomic.Int64
	queueTimeout        time.Duration
	jobs                *jobStore
	jobCtx              context.Context
	jobCancel           context.CancelFunc
	openJTalkPath       string
	openJTalkDictionary string
	voiceDir            string
	catalog             *plugin.Catalog
	service             *synth.Service
	voiceLibrary        *voicebank.Library
	runtime             *synth.Runtime
}

type apiVoicebankResolver struct {
	server *Server
}

func (r apiVoicebankResolver) Resolve(id string) (string, bool) {
	if r.server.voiceLibrary != nil {
		summary, ok := r.server.voiceLibrary.Resolve(id)
		return summary.Path, ok
	}
	voicebank, ok := r.server.resolveVoicebank(id)
	return voicebank.Path, ok
}

func New(config Config) (*Server, error) {
	voiceDir := voicebank.ResolveDirectory(config.VoiceDir)
	srv := &Server{
		voicebanks:    map[string]Voicebank{},
		synthesisSem:  make(chan struct{}, concurrencyLimit(config.MaxConcurrentSynthesis, maxConcurrentSynthesis)),
		batchSem:      make(chan struct{}, concurrencyLimit(config.MaxConcurrentBatches, maxConcurrentBatches)),
		queueTimeout:  config.QueueTimeout,
		jobs:          newJobStore(),
		voiceDir:      voiceDir,
		openJTalkPath: config.OpenJTalkPath, openJTalkDictionary: config.OpenJTalkDictionary,
		voiceLibrary: voicebank.NewLibrary(voiceDir),
	}
	if srv.queueTimeout == 0 {
		srv.queueTimeout = defaultQueueTimeout
	}
	runtime, err := synth.NewRuntime(synth.RuntimeConfig{
		OpenJTalkPath: config.OpenJTalkPath, OpenJTalkDictionary: config.OpenJTalkDictionary,
		ModelDirectories: config.ModelDirectories,
	}, apiVoicebankResolver{server: srv})
	if err != nil {
		return nil, err
	}
	srv.catalog, srv.service = runtime.Catalog, runtime.Service
	srv.runtime = runtime
	if err := srv.loadVoiceDirectory(); err != nil {
		_ = runtime.Close()
		return nil, fmt.Errorf("load voicebanks from %s: %w", voiceDir, err)
	}
	if len(srv.voicebanks) == 0 {
		log.Printf("warning: no voicebanks found in %s", voiceDir)
	}
	srv.jobCtx, srv.jobCancel = context.WithCancel(context.Background())

	return srv, nil
}

// concurrencyLimitは、設定値が正ならそれを、そうでなければ既定値を使う。
func concurrencyLimit(configured, fallback int) int {
	if configured > 0 {
		return configured
	}
	return fallback
}

func (s *Server) Close() error {
	if s.jobCancel != nil {
		s.jobCancel()
	}
	if s.runtime == nil {
		return nil
	}
	return s.runtime.Close()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/openapi.yaml", s.handleOpenAPI)
	mux.HandleFunc("GET /api/voicebanks", s.handleListVoicebanks)
	mux.HandleFunc("POST /api/voicebanks/reload", s.handleReloadVoicebanks)
	mux.HandleFunc("POST /api/synthesize/audio", s.handleSynthesizeAudio)
	mux.HandleFunc("POST /api/synthesize/label", s.handleSynthesizeLabel)
	mux.HandleFunc("POST /api/synthesize/report", s.handleSynthesizeReport)
	mux.HandleFunc("POST /api/synthesize/batch", s.handleSynthesizeBatch)
	mux.HandleFunc("POST /api/synthesize/jobs", s.handleCreateSynthesisJob)
	mux.HandleFunc("GET /api/synthesize/jobs/{job_id}", s.handleSynthesisJob)
	mux.HandleFunc("GET /api/synthesize/jobs/{job_id}/audio", s.handleSynthesisJobAudio)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/analyze", s.handleAnalyze)
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	})
	mux.Handle("GET /", http.FileServer(http.FS(uiFS)))
	return localOnly(jsonRouterErrors(mux))
}

func (s *Server) loadVoiceDirectory() error {
	if s.runtime != nil {
		return s.runtime.Reload(s.reloadVoiceDirectory)
	}
	return s.reloadVoiceDirectory()
}

func (s *Server) reloadVoiceDirectory() error {
	if s.voiceLibrary == nil {
		s.voiceLibrary = voicebank.NewLibrary(s.voiceDir)
	}
	if err := s.voiceLibrary.Reload(); err != nil {
		s.mu.Lock()
		s.voicebanks = map[string]Voicebank{}
		s.mu.Unlock()
		return err
	}
	items := s.voiceLibrary.List()
	next := make(map[string]Voicebank, len(items))
	for _, entry := range items {
		id, summary := entry.ID, entry.Summary
		item := Voicebank{ID: id, Name: summary.Name, Path: summary.Path, Kind: summary.Kind}
		if inspected, inspectErr := inspectVoicebank(summary.Path); inspectErr != nil {
			log.Printf("voicebank metadata: %s: %v", summary.Path, inspectErr)
		} else {
			item = inspected
		}
		item.ID = id
		next[id] = item
		log.Printf("voicebank: %s (%s)", summary.Name, id)
	}
	s.mu.Lock()
	s.voicebanks = next
	s.mu.Unlock()
	if s.runtime != nil {
		render.ClearWAVCache()
	} else {
		tts.ClearCaches()
	}
	return nil
}

func inspectVoicebank(path string) (Voicebank, error) {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": appinfo.Version(),
		"synthesis": s.concurrencyStatus(s.synthesisSem, &s.synthesisWaiting),
		"batch":     s.concurrencyStatus(s.batchSem, &s.batchWaiting),
	})
}

// concurrencyStatusは、実行枠の上限と使用中・待機中の件数を返す。
func (s *Server) concurrencyStatus(sem chan struct{}, waiting *atomic.Int64) map[string]int64 {
	return map[string]int64{"limit": int64(cap(sem)), "active": int64(len(sem)), "queued": waiting.Load()}
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"models": s.pluginCatalog().Models})
}

func (s *Server) handleSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings.Visible(), "problems": s.pluginCatalog().Problems})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Language   string                    `json:"language"`
		Text       string                    `json:"text"`
		Dictionary []utautts.DictionaryEntry `json:"dictionary"`
	}
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	if request.Text == "" {
		writeAPIError(w, http.StatusBadRequest, invalidInput("text", "1-500 characters", "text is required"))
		return
	}
	if len([]rune(request.Text)) > maxTextRunes {
		writeAPIError(w, http.StatusRequestEntityTooLarge, invalidInput("text", fmt.Sprintf("at most %d characters", maxTextRunes), fmt.Sprintf("text is limited to %d characters", maxTextRunes)))
		return
	}
	if strings.TrimSpace(request.Language) == "" {
		writeAPIError(w, http.StatusBadRequest, invalidInput("language", "ja, en, zh, es, or pt", "language is required"))
		return
	}
	release, status, err := s.acquire(r.Context(), s.synthesisSem, &s.synthesisWaiting)
	if err != nil {
		writeAPIError(w, status, err)
		return
	}
	defer release()
	preview, err := s.synthesisService().AnalyzeContext(r.Context(), entryinput.Apply(utautts.Request{
		Text: request.Text, Language: request.Language, Dictionary: request.Dictionary,
	}, synth.DefaultRequest()))
	if err != nil {
		writeAPIError(w, contextErrorStatus(err, http.StatusUnprocessableEntity), err)
		return
	}
	items := make([]map[string]any, 0, len(preview.Morae))
	for index, mora := range preview.Morae {
		items = append(items, map[string]any{"position": index, "mora": mora.Text, "consonant": mora.Consonant, "vowel": mora.Vowel, "pause": mora.Pause})
	}
	writeJSON(w, http.StatusOK, map[string]any{"reading": preview.Reading, "morae": items})
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
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"voicebanks": s.voicebankList()})
}

func (s *Server) handleSynthesizeAudio(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeAPIError(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-UtauTTS-Method", result.Method)
	w.Header().Set("X-UtauTTS-Reading", result.Plan.Reading)
	w.Header().Set("X-UtauTTS-Diagnostic-Count", strconv.Itoa(len(result.Diagnostics())))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(audio.PCMToWavBytes(result.Audio)); err != nil {
		log.Printf("write synthesis response: %v", err)
	}
}

func (s *Server) handleSynthesizeLabel(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeAPIError(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-UtauTTS-Method", result.Method)
	w.Header().Set("X-UtauTTS-Reading", result.Plan.Reading)
	w.Header().Set("X-UtauTTS-Diagnostic-Count", strconv.Itoa(len(result.Diagnostics())))
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, result.Lab); err != nil {
		log.Printf("write label response: %v", err)
	}
}

func (s *Server) handleSynthesizeReport(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeAPIError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result.Report())
}

func (s *Server) synthesize(ctx context.Context, request SynthesisRequest) (*synth.Result, int, error) {
	if status, err := validateSynthesisRequest(request); err != nil {
		return nil, status, err
	}
	release, status, err := s.acquire(ctx, s.synthesisSem, &s.synthesisWaiting)
	if err != nil {
		return nil, status, err
	}
	defer release()
	return s.synthesizeAdmitted(ctx, request)
}

// synthesizeAdmittedは、入力の検証と実行枠の取得を済ませたあとの合成。
func (s *Server) synthesizeAdmitted(ctx context.Context, request SynthesisRequest) (*synth.Result, int, error) {
	result, err := s.synthesisService().SynthesizeContext(ctx, request.InternalRequest())
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

// validateSynthesisRequestは、合成の前に要求の形を確かめる。
// 必須項目の欠落と範囲外の値は400、上限超過は413で返す。
func validateSynthesisRequest(request SynthesisRequest) (int, error) {
	reading := request.ReadingOrKana()
	if reading == "" && request.Text == "" {
		return http.StatusBadRequest, invalidInput("text", "text, reading, or kana with 1-500 characters", "text or reading is required")
	}
	if len([]rune(request.Text)) > maxTextRunes || len([]rune(reading)) > maxTextRunes {
		return http.StatusRequestEntityTooLarge, invalidInput("text", fmt.Sprintf("at most %d characters", maxTextRunes), fmt.Sprintf("text and reading are limited to %d characters", maxTextRunes))
	}
	if err := tts.ValidateMethod(request.Method); err != nil {
		return http.StatusBadRequest, &inputError{
			field: "method", expected: fmt.Sprintf("%q or %q", tts.MethodWaveformSynthesis, tts.MethodReferenceMatching), err: err,
		}
	}
	if strings.TrimSpace(request.Language) == "" {
		return http.StatusBadRequest, invalidInput("language", "ja, en, zh, es, or pt", "language is required")
	}
	if strings.TrimSpace(request.VoicebankID) == "" {
		return http.StatusBadRequest, invalidInput("voicebank_id", "a registered voicebank ID from GET /api/voicebanks", "voicebank_id is required")
	}
	for _, duration := range []struct {
		field string
		value float64
		max   float64
	}{
		{"mora_duration_ms", request.MoraDurationMS, 1000},
		{"pause_duration_ms", request.PauseDurationMS, 3000},
		{"leading_preutterance_ms", request.LeadingPreutteranceMS, 1000},
	} {
		if duration.value < 0 || duration.value > duration.max {
			return http.StatusBadRequest, invalidInput(duration.field, fmt.Sprintf("0..%g", duration.max), fmt.Sprintf("%s must be between 0 and %g", duration.field, duration.max))
		}
	}
	if len(request.MoraDurationsMS) > maxTextRunes {
		return http.StatusRequestEntityTooLarge, invalidInput("mora_durations_ms", fmt.Sprintf("at most %d values", maxTextRunes), "mora duration settings contain too many values")
	}
	for _, duration := range request.MoraDurationsMS {
		if duration < 0 || duration > 1000 {
			return http.StatusBadRequest, invalidInput("mora_durations_ms", "each value 0..1000", "mora duration settings are outside the supported range")
		}
	}
	if request.IntonationStrength < 0 || request.IntonationStrength > tts.MaxIntonationStrength {
		return http.StatusBadRequest, invalidInput("intonation_strength", fmt.Sprintf("0..%g", tts.MaxIntonationStrength), fmt.Sprintf("intonation_strength must be between 0 and %.0f", tts.MaxIntonationStrength))
	}
	if request.ManualPitch != nil && len(request.ManualPitch.Points) > maxManualPitchPoints {
		return http.StatusRequestEntityTooLarge, invalidInput("manual_pitch.points", fmt.Sprintf("at most %d points", maxManualPitchPoints), fmt.Sprintf("manual pitch supports at most %d points", maxManualPitchPoints))
	}
	return http.StatusOK, nil
}

// acquireは実行枠を取る。枠が空かないままQueueTimeoutを過ぎたら429を返す。
func (s *Server) acquire(ctx context.Context, sem chan struct{}, waiting *atomic.Int64) (func(), int, error) {
	if sem == nil {
		return func() {}, http.StatusOK, nil
	}
	waiting.Add(1)
	defer waiting.Add(-1)
	var expired <-chan time.Time
	if s.queueTimeout > 0 {
		timer := time.NewTimer(s.queueTimeout)
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, http.StatusOK, nil
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, ctx.Err()
	case <-expired:
		return nil, http.StatusTooManyRequests, errors.New("synthesis queue is full; retry later")
	}
}

func contextErrorStatus(err error, fallback int) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout
	}
	return fallback
}

func (s *Server) synthesisService() *synth.Service {
	if s.service != nil {
		return s.service
	}
	return synth.NewService(s.pluginCatalog(),
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
