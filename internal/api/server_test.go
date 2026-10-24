package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func localRequest(method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	request.Host = "127.0.0.1"
	return request
}

func TestSynthesisWaitHonorsRequestCancellation(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	for range cap(server.synthesisSem) {
		server.synthesisSem <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := localRequest(http.MethodPost, "/api/synthesize/audio", bytes.NewBufferString(`{"method":"waveform-synthesis","language":"ja","kana":"あ"}`)).WithContext(ctx)
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
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close server: %v", err)
		}
	})
	return server
}

func TestUnknownPathIsNotIndex(t *testing.T) {
	response := httptest.NewRecorder()
	mustNewServer(t, Config{VoiceDir: t.TempDir()}).Handler().ServeHTTP(response, localRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestConsoleUIIsServedLocally(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	handler := server.Handler()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, localRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("console status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("content type = %q", contentType)
	}
	if !strings.Contains(response.Body.String(), "UtauTTS Server Console") {
		t.Fatal("console page is missing")
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, localRequest(http.MethodGet, "/ui", nil))
	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("/ui status = %d", response.Code)
	}
}

func TestNewAllowsMissingVoiceDirectory(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: filepath.Join(t.TempDir(), "missing")})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/voicebanks", nil))
	var result struct {
		Voicebanks []Voicebank `json:"voicebanks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Voicebanks) != 0 {
		t.Fatalf("voicebanks=%s error=%v", response.Body.String(), err)
	}
}

func TestJSONAndBatchLimits(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()}).Handler()

	response := httptest.NewRecorder()
	request := localRequest(http.MethodPost, "/api/analyze",
		strings.NewReader(`{"text":"`+strings.Repeat("あ", maxTextRunes+1)+`"}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("long analyze status = %d, body = %s", response.Code, response.Body.String())
	}

	items := strings.Repeat(`{"request":{"method":"waveform-synthesis","language":"ja","kana":"あ"}},`, maxBatchItems) + `{"request":{"method":"waveform-synthesis","language":"ja","kana":"あ"}}`
	response = httptest.NewRecorder()
	request = localRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[`+items+`]}`))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large batch status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request = localRequest(http.MethodPost, "/api/analyze", strings.NewReader(strings.Repeat(" ", maxJSONRequestBytes+1)))
	server.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large JSON status = %d", response.Code)
	}
}

func TestBatchRejectsDuplicateArchiveNamesBeforeSynthesis(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	request := localRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[{"name":"audio.wav"},{"name":"audio.wav"}]}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "duplicate batch filename") {
		t.Fatalf("duplicate batch response = %d %s", response.Code, response.Body.String())
	}

	request = localRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(`{"items":[{"name":".."},{"name":"utterance-1.wav"}]}`))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unsafe batch name response = %d %s", response.Code, response.Body.String())
	}
}

func TestSynthesizeEndpointReportsWaveformSynthesis(t *testing.T) {
	server := newWaveformTestServer(t)
	body := bytes.NewBufferString(`{"method":"waveform-synthesis","language":"ja","kana":"あ","voicebank_id":"test","mora_duration_ms":100}`)
	request := localRequest(http.MethodPost, "/api/synthesize/audio", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if wav := response.Body.Bytes(); len(wav) < 4 || string(wav[:4]) != "RIFF" || response.Header().Get("X-UtauTTS-Method") != "waveform-synthesis" {
		t.Fatalf("unexpected audio response: headers=%v bytes=%d", response.Header(), len(wav))
	}
}

func TestSynthesizeEndpointCountsIgnoredSettings(t *testing.T) {
	server := newWaveformTestServer(t)
	body := bytes.NewBufferString(`{"method":"waveform-synthesis","language":"ja","kana":"あ","voicebank_id":"test","settings":{"unknown_option":1}}`)
	request := localRequest(http.MethodPost, "/api/synthesize/audio", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-UtauTTS-Diagnostic-Count") != "1" {
		t.Fatalf("status = %d, diagnostics = %q", response.Code, response.Header().Get("X-UtauTTS-Diagnostic-Count"))
	}
}

func newWaveformTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	bank := filepath.Join(root, "test")
	if err := os.Mkdir(bank, 0700); err != nil {
		t.Fatal(err)
	}
	wavPath := filepath.Join(bank, "a.wav")
	samples := make([]int16, 400)
	for i := range samples {
		samples[i] = 8000
	}
	if err := audio.WriteWav(wavPath, &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bank, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return mustNewServer(t, Config{VoiceDir: root})
}

func TestLabelAndBatchSidecarEndpoints(t *testing.T) {
	server := newWaveformTestServer(t)
	synthesis := `{"method":"waveform-synthesis","language":"ja","kana":"あ","voicebank_id":"test","mora_duration_ms":100}`

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/label", strings.NewReader(synthesis)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), " a\n") {
		t.Fatalf("label response = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	body := `{"write_text":true,"write_lab":true,"items":[{"name":"sample","request":` + synthesis + `}]}`
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/batch", strings.NewReader(body)))
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

func TestSynthesisInputCompatibility(t *testing.T) {
	server := newWaveformTestServer(t)
	synthesize := func(t *testing.T, fields string) *httptest.ResponseRecorder {
		t.Helper()
		body := `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test","apply_pitch":false,` + fields + `}`
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/audio", strings.NewReader(body)))
		if response.Code != http.StatusOK {
			t.Fatalf("synthesis response = %d %s", response.Code, response.Body.String())
		}
		return response
	}
	for _, test := range []struct {
		name, input, equivalent, different string
	}{
		{"legacy kana", `"kana":"あ"`, `"reading":"あ"`, ""},
		{"reading precedence", `"reading":"あ","kana":"unknown"`, `"reading":"あ"`, ""},
		{"dictionary", `"text":"token","dictionary":[{"surface":"token","reading":"あ"}]`, `"reading":"ア"`, ""},
		{"zero pause uses default", `"reading":"あ、あ","pause_duration_ms":0`, `"reading":"あ、あ"`, `"reading":"あ、あ","pause_duration_ms":40`},
		{"map overrides typed duration", `"reading":"あ","mora_duration_ms":80,"settings":{"mora_duration_ms":160}`, `"reading":"あ","mora_duration_ms":160`, `"reading":"あ","mora_duration_ms":80`},
		{"edited duration", `"reading":"あ","mora_duration_ms":80,"mora_durations_ms":[160]`, `"reading":"あ","mora_duration_ms":160`, `"reading":"あ","mora_duration_ms":80`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, want := synthesize(t, test.input), synthesize(t, test.equivalent)
			if !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) || got.Header().Get("X-UtauTTS-Reading") != want.Header().Get("X-UtauTTS-Reading") {
				t.Fatal("compatible inputs produced different audio or reading")
			}
			if test.different != "" && bytes.Equal(got.Body.Bytes(), synthesize(t, test.different).Body.Bytes()) {
				t.Fatal("duration setting did not affect the output")
			}
		})
	}
}

func TestHealthReportsStatus(t *testing.T) {
	server := newWaveformTestServer(t)
	request := localRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" || payload["version"] == "" {
		t.Fatalf("unexpected response: %#v", payload)
	}
}

func TestAPIMetadata(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	request := localRequest(http.MethodGet, "/api/settings", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Settings []struct {
			ID string `json:"id"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Settings) == 0 || payload.Settings[0].ID != "mora_duration_ms" {
		t.Fatalf("settings = %#v", payload.Settings)
	}
}

func TestAnalyzeEndpointReturnsMoraes(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	request := localRequest(http.MethodPost, "/api/analyze", bytes.NewBufferString(`{"language":"ja","text":"あいうえお"}`))
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
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	body := `{"language":"ja","text":"v8","dictionary":[{"surface":"v8","reading":"ぶいはち"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/analyze", strings.NewReader(body)))
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
	server := mustNewServer(t, Config{VoiceDir: root})

	request := localRequest(http.MethodGet, "/api/voicebanks", nil)
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

	request = localRequest(http.MethodPost, "/api/voicebanks/reload", nil)
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
