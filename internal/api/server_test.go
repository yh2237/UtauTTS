package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plugin"
	"utautts/internal/voicebank"
)

func TestSynthesisWaitHonorsRequestCancellation(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	for range cap(server.synthesisSem) {
		server.synthesisSem <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/api/synthesize/audio", bytes.NewBufferString(`{"kana":"あ"}`)).WithContext(ctx)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
}

func mustNewServer(t *testing.T, config Config) *Server {
	t.Helper()
	server, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestProtectedHandlerRequiresBearerToken(t *testing.T) {
	server := mustNewServer(t, Config{AuthToken: "secret", VoiceDir: t.TempDir()})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestUnknownPathIsNotIndex(t *testing.T) {
	response := httptest.NewRecorder()
	mustNewServer(t, Config{VoiceDir: t.TempDir()}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestConsoleUIDocsAreServedPublicly(t *testing.T) {
	server := mustNewServer(t, Config{AuthToken: "secret", VoiceDir: t.TempDir()})
	handler := server.Handler()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("console status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("content type = %q", contentType)
	}
	for _, want := range []string{"UtauTTS Server Console", "/api/synthesize/audio", "/api/analyze", "syn-alias-policy", "cvvc-enhanced", "payload.alias_policy"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("console page is missing %q", want)
		}
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("/ui status = %d", response.Code)
	}
}

func TestVoicebankRegistrationDisabledByDefault(t *testing.T) {
	response := httptest.NewRecorder()
	mustNewServer(t, Config{VoiceDir: t.TempDir()}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/voicebanks", bytes.NewBufferString(`{"path":"."}`)))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRendererMetadataIncludesConfiguredDefault(t *testing.T) {
	response := httptest.NewRecorder()
	mustNewServer(t, Config{VoiceDir: t.TempDir(), Renderer: "waveform"}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/renderers", nil))
	var payload struct {
		Default string `json:"default_renderer"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &payload)
	if payload.Default != "waveform" {
		t.Fatalf("default = %q", payload.Default)
	}
}

func TestNewReportsInvalidPlugin(t *testing.T) {
	pluginDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDirectory, "plugin.json"), []byte(`{"kind":"renderer"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{VoiceDir: t.TempDir(), RendererDirectories: []string{pluginDirectory}})
	if err != nil {
		t.Fatal(err)
	}
	if len(server.pluginCatalog().Problems) == 0 {
		t.Fatal("invalid plugin was not reported")
	}
}

func TestNewAllowsMissingVoiceDirectory(t *testing.T) {
	server, err := New(Config{VoiceDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	if len(server.voicebankList()) != 0 {
		t.Fatalf("voicebanks=%#v", server.voicebankList())
	}
}

func TestJSONAndBatchLimits(t *testing.T) {
	server := (&Server{voicebanks: map[string]Voicebank{}}).Handler()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/analyze", strings.NewReader(`{"text":"あ","unknown":true}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", response.Code)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/synthesize/audio",
		strings.NewReader(`{"text":"`+strings.Repeat("あ", maxTextRunes+1)+`"}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("long synthesis status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/analyze",
		strings.NewReader(`{"text":"`+strings.Repeat("あ", maxTextRunes+1)+`"}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("long analyze status = %d, body = %s", response.Code, response.Body.String())
	}

	items := strings.Repeat(`{"request":{"kana":"あ"}},`, maxBatchItems) + `{"request":{"kana":"あ"}}`
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[`+items+`]}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large batch status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/analyze", strings.NewReader(strings.Repeat(" ", maxJSONRequestBytes+1)))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large JSON status = %d", response.Code)
	}
}

func TestBatchRejectsDuplicateArchiveNamesBeforeSynthesis(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[{"name":"audio.wav"},{"name":"audio.wav"}]}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "duplicate batch filename") {
		t.Fatalf("duplicate batch response = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[{"name":".."},{"name":"utterance-1.wav"}]}`))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unsafe batch name response = %d %s", response.Code, response.Body.String())
	}
}

func TestSynthesizeEndpointReportsWaveformRenderer(t *testing.T) {
	root := t.TempDir()
	wavPath := filepath.Join(root, "a.wav")
	samples := make([]int16, 400)
	for i := range samples {
		samples[i] = 8000
	}
	if err := audio.WriteWav(wavPath, &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := &Server{renderer: "waveform", catalog: &plugin.Catalog{Renderers: []plugin.Renderer{{ID: "waveform", Backend: "waveform"}}}, voicebanks: map[string]Voicebank{
		"test": {ID: "test", Name: "test", Path: root},
	}}
	body := bytes.NewBufferString(`{"kana":"あ","voicebank_id":"test","mora_duration_ms":100}`)
	request := httptest.NewRequest(http.MethodPost, "/api/synthesize/audio", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if wav := response.Body.Bytes(); len(wav) < 4 || string(wav[:4]) != "RIFF" || response.Header().Get("X-UtauTTS-Engine") != "waveform" {
		t.Fatalf("unexpected audio response: headers=%v bytes=%d", response.Header(), len(wav))
	}
}

func TestLabelAndBatchSidecarEndpoints(t *testing.T) {
	root := t.TempDir()
	wavPath := filepath.Join(root, "a.wav")
	samples := make([]int16, 400)
	for index := range samples {
		samples[index] = 8000
	}
	if err := audio.WriteWav(wavPath, &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := &Server{renderer: "waveform", catalog: &plugin.Catalog{Renderers: []plugin.Renderer{{ID: "waveform", Backend: "waveform"}}}, voicebanks: map[string]Voicebank{
		"test": {ID: "test", Path: root},
	}}
	synthesis := `{"kana":"あ","voicebank_id":"test","mora_duration_ms":100}`

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/synthesize/label", strings.NewReader(synthesis)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), " a\n") {
		t.Fatalf("label response = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	body := `{"write_text":true,"write_lab":true,"items":[{"name":"sample","request":` + synthesis + `}]}`
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("batch response = %d %s", response.Code, response.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
	}
	for _, name := range []string{"sample.wav", "sample.txt", "sample.lab"} {
		if !names[name] {
			t.Fatalf("batch archive is missing %s: %#v", name, names)
		}
	}
}

func TestSynthesizeEndpointRejectsUnknownText(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := &Server{renderer: "waveform", catalog: &plugin.Catalog{Renderers: []plugin.Renderer{{ID: "waveform", Backend: "waveform"}}}, voicebanks: map[string]Voicebank{
		"test": {ID: "test", Path: root},
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/synthesize/audio", bytes.NewBufferString(`{"text":"UtauTTS","voicebank_id":"test"}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHealthReportsConfiguredRenderer(t *testing.T) {
	server := &Server{renderer: "waveform"}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" || payload["engine"] != "waveform" {
		t.Fatalf("unexpected response: %#v", payload)
	}
}

func TestAPIMetadata(t *testing.T) {
	server := mustNewServer(t, Config{Renderer: "waveform", VoiceDir: t.TempDir()})
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/renderers", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("renderers status = %d, body = %s", response.Code, response.Body.String())
	}
	var renderers struct {
		Renderers []struct {
			ID string `json:"id"`
		} `json:"renderers"`
		Wavtools []struct {
			ID string `json:"id"`
		} `json:"wavtools"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &renderers); err != nil {
		t.Fatal(err)
	}
	if len(renderers.Renderers) < 2 {
		t.Fatalf("renderers = %#v", renderers.Renderers)
	}
	foundClassic := false
	for _, renderer := range renderers.Renderers {
		foundClassic = foundClassic || renderer.ID == "classic-utau"
	}
	if !foundClassic || len(renderers.Wavtools) == 0 || renderers.Wavtools[0].ID != "builtin" {
		t.Fatalf("Classic UTAU metadata = %#v", renderers)
	}
}

func TestAnalyzeEndpointReturnsMoraes(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewBufferString(`{"text":"あいうえお"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Reading string `json:"reading"`
		Morae   []struct {
			Mora string `json:"mora"`
		} `json:"morae"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Reading == "" || len(payload.Morae) != 5 {
		t.Fatalf("unexpected analysis: %#v", payload)
	}
}

func TestAnalyzeEndpointUsesUserDictionary(t *testing.T) {
	server := &Server{}
	body := `{"text":"v8","dictionary":[{"surface":"v8","reading":"ぶいはち"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/analyze", strings.NewReader(body)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"reading":"ブイハチ"`) {
		t.Fatalf("dictionary analysis = %d %s", response.Code, response.Body.String())
	}
}

func TestVoicebankEndpointsDiscoverAndReloadDirectory(t *testing.T) {
	root := t.TempDir()
	makeBank := func(directory, name string) {
		dir := filepath.Join(root, directory)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "character.txt"), []byte("name="+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	makeBank("alpha", "アルファ")
	server := &Server{voicebanks: map[string]Voicebank{}, voiceDir: root}
	if err := server.loadVoiceDirectory(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/voicebanks", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	var first struct {
		Voicebanks []Voicebank `json:"voicebanks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Voicebanks) != 1 || first.Voicebanks[0].ID != "alpha" || first.Voicebanks[0].Name != "アルファ" {
		t.Fatalf("voicebanks = %#v", first.Voicebanks)
	}
	for _, field := range []string{`"vcv_contexts":{}`, `"vc_contexts":{}`, `"has_vc":false`, `"has_initial_vcv":false`, `"has_n_context_vcv":false`} {
		if !strings.Contains(response.Body.String(), field) {
			t.Fatalf("voicebank capability field %s was omitted: %s", field, response.Body.String())
		}
	}

	makeBank("beta", "ベータ")
	if first.Voicebanks[0].OtoFileCount != 1 || first.Voicebanks[0].PhonemeCount != 1 || first.Voicebanks[0].DiagnosticCount != 0 || first.Voicebanks[0].AliasCounts[voicebank.AliasCV] != 1 {
		t.Fatalf("voicebank metadata = %#v", first.Voicebanks[0])
	}

	request = httptest.NewRequest(http.MethodPost, "/api/voicebanks/reload", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("reload status = %d, body = %s", response.Code, response.Body.String())
	}
	var second struct {
		Voicebanks []Voicebank `json:"voicebanks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if len(second.Voicebanks) != 2 || second.Voicebanks[1].ID != "beta" {
		t.Fatalf("reloaded voicebanks = %#v", second.Voicebanks)
	}
}
