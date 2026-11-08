package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func decodeAPIError(t *testing.T, response *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("error is not JSON: %v, body=%q", err, response.Body.String())
	}
	return body
}

func TestMissingRequiredFieldsReturnInvalidRequest(t *testing.T) {
	server := newWaveformTestServer(t)
	for _, test := range []struct{ name, body, field string }{
		{"no text", `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test"}`, "text"},
		{"no method", `{"language":"ja","voicebank_id":"test","reading":"あ"}`, "method"},
		{"unknown method", `{"method":"sing","language":"ja","voicebank_id":"test","reading":"あ"}`, "method"},
		{"no language", `{"method":"waveform-synthesis","voicebank_id":"test","reading":"あ"}`, "language"},
		{"no voicebank", `{"method":"waveform-synthesis","language":"ja","reading":"あ"}`, "voicebank_id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/audio", strings.NewReader(test.body)))
			assertAPIError(t, response, http.StatusBadRequest, "invalid_request")
			if body := decodeAPIError(t, response); body.Field != test.field || body.Expected == "" {
				t.Fatalf("field=%q expected=%q", body.Field, body.Expected)
			}
		})
	}
}

func TestInputErrorsNameFieldAndExpected(t *testing.T) {
	server := newWaveformTestServer(t)
	base := `"method":"waveform-synthesis","language":"ja","voicebank_id":"test","reading":"あ"`
	for _, test := range []struct{ name, body, field, expected string }{
		{"mora duration", `{` + base + `,"mora_duration_ms":1001}`, "mora_duration_ms", "0..1000"},
		{"pause duration", `{` + base + `,"pause_duration_ms":3001}`, "pause_duration_ms", "0..3000"},
		{"mora durations", `{` + base + `,"mora_durations_ms":[1001]}`, "mora_durations_ms", "each value 0..1000"},
		{"intonation", `{` + base + `,"intonation_strength":9}`, "intonation_strength", "0..8"},
		{"unknown field", `{"unknown":1}`, "unknown", "a documented field"},
		{"wrong type", `{"method":1}`, "method", "string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/audio", strings.NewReader(test.body)))
			assertAPIError(t, response, http.StatusBadRequest, "invalid_request")
			if body := decodeAPIError(t, response); body.Field != test.field || body.Expected != test.expected {
				t.Fatalf("field=%q expected=%q, want %q %q", body.Field, body.Expected, test.field, test.expected)
			}
		})
	}
}

func TestUnknownPathAndMethodReturnJSONErrors(t *testing.T) {
	handler := newWaveformTestServer(t).Handler()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, localRequest(http.MethodGet, "/api/missing", nil))
	assertAPIError(t, response, http.StatusNotFound, "not_found")

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, localRequest(http.MethodPost, "/api/health", nil))
	assertAPIError(t, response, http.StatusMethodNotAllowed, "method_not_allowed")
	if response.Header().Get("Allow") == "" {
		t.Fatal("405 response has no Allow header")
	}
}

func TestSynthesisReportReturnsDiagnostics(t *testing.T) {
	server := newWaveformTestServer(t)
	body := `{"method":"waveform-synthesis","language":"ja","kana":"あ","voicebank_id":"test","settings":{"unknown_option":1}}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/report", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var report struct {
		Reading     string `json:"reading"`
		Diagnostics []struct {
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Reading == "" || len(report.Diagnostics) != 1 || report.Diagnostics[0].Message == "" {
		t.Fatalf("report = %#v", report)
	}
}

func TestSynthesisJobPolling(t *testing.T) {
	server := newWaveformTestServer(t)
	body := `{"method":"waveform-synthesis","language":"ja","kana":"あ","voicebank_id":"test","mora_duration_ms":100}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/jobs", strings.NewReader(body)))
	if response.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var created struct {
		JobID     string `json:"job_id"`
		Status    string `json:"status"`
		StatusURL string `json:"status_url"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.JobID == "" || created.Status != jobQueued || created.StatusURL != "/api/synthesize/jobs/"+created.JobID {
		t.Fatalf("created = %#v", created)
	}

	deadline := time.Now().Add(30 * time.Second)
	var job synthesisJobResponse
	for {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, localRequest(http.MethodGet, created.StatusURL, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.Status == jobSucceeded || job.Status == jobFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not finish: %#v", job)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.Status != jobSucceeded || job.Result == nil || job.Result.Reading == "" {
		t.Fatalf("job = %#v", job)
	}

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, created.StatusURL+"/audio", nil))
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Body.String(), "RIFF") {
		t.Fatalf("audio = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/synthesize/jobs/unknown", nil))
	assertAPIError(t, response, http.StatusNotFound, "not_found")
}

func TestSynthesisJobRejectsInvalidRequestBeforeAccepting(t *testing.T) {
	server := newWaveformTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/jobs", strings.NewReader(`{"language":"ja"}`)))
	assertAPIError(t, response, http.StatusBadRequest, "invalid_request")
}

func TestQueueTimeoutReturnsTooManyRequests(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir(), QueueTimeout: 20 * time.Millisecond})
	for range cap(server.synthesisSem) {
		server.synthesisSem <- struct{}{}
	}
	body := `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test","reading":"あ"}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/audio", strings.NewReader(body)))
	assertAPIError(t, response, http.StatusTooManyRequests, "too_many_requests")
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("429 response has no Retry-After header")
	}
}

func TestConfiguredConcurrencyLimits(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir(), MaxConcurrentSynthesis: 1, MaxConcurrentBatches: 3})
	if cap(server.synthesisSem) != 1 || cap(server.batchSem) != 3 {
		t.Fatalf("limits = %d %d", cap(server.synthesisSem), cap(server.batchSem))
	}
}
