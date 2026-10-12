package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfiguredLimitsAreExposedAndApplied(t *testing.T) {
	limits := Limits{MaxJSONRequestBytes: 256, MaxTextRunes: 2, MaxBatchItems: 1, MaxBatchWAVBytes: 20, MaxManualPitchPoints: 1, MaxConcurrentSynthesis: 1, MaxConcurrentBatches: 1}
	server := mustNewServer(t, Config{VoiceDir: t.TempDir(), Limits: limits})
	if cap(server.synthesisSem) != 1 || cap(server.batchSem) != 1 {
		t.Fatal("configured concurrency was not applied")
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/limits", nil))
	var reported map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &reported); err != nil {
		t.Fatal(err)
	}
	if reported["max_json_request_bytes"] != float64(256) || reported["max_text_runes"] != float64(2) || reported["queue_timeout"] != "0s" || reported["synthesis_timeout"] != "0s" {
		t.Fatalf("limits=%#v", reported)
	}
	// Use an artificial bank for the endpoint that checks output WAV size.
	server = newWaveformTestServer(t)
	server.limits, _ = limits.normalized()
	for _, test := range []struct{ path, body string }{
		{"/api/analyze", strings.Repeat(" ", 257)},
		{"/api/analyze", `{"text":"あいう"}`},
		{"/api/synthesize/audio", `{"reading":"あいう"}`},
		{"/api/synthesize/label", `{"reading":"あ","mora_durations_ms":[100,100,100]}`},
		{"/api/synthesize/result", `{"reading":"あ","resampler_expressions":[{},{},{}]}`},
		{"/api/synthesize/audio", `{"reading":"あ","manual_pitch":{"points":[{},{}]}}`},
		{"/api/synthesize/batch", `{"items":[{},{}]}`},
		{"/api/synthesize/batch", `{"items":[{"request":{"reading":"あ","apply_pitch":false}}]}`},
	} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, localRequest(http.MethodPost, test.path, strings.NewReader(test.body)))
		assertAPIError(t, response, 413, "limit_exceeded")
	}
}

func TestQueueAndSynthesisTimeoutsReleaseSlots(t *testing.T) {
	for _, synthesisDeadline := range []bool{false, true} {
		limits := Limits{MaxConcurrentSynthesis: 1, MaxConcurrentBatches: 1, QueueTimeout: time.Millisecond}
		code, status := "queue_timeout", 503
		if synthesisDeadline {
			limits.SynthesisTimeout = time.Nanosecond
			code, status = "deadline_exceeded", 408
		}
		server := mustNewServer(t, Config{VoiceDir: t.TempDir(), Limits: limits})
		server.synthesisSem <- struct{}{}
		server.batchSem <- struct{}{}
		for _, path := range []string{"/api/analyze", "/api/synthesize/audio", "/api/synthesize/label", "/api/synthesize/result", "/api/synthesize/batch"} {
			body := `{"text":"あ"}`
			if strings.HasSuffix(path, "/batch") {
				body = `{"items":[{"request":{"reading":"あ"}}]}`
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, path, strings.NewReader(body)))
			assertAPIError(t, response, status, code)
			if len(server.synthesisSem) != 1 || len(server.batchSem) != 1 {
				t.Fatal("wait timeout changed the occupied slots")
			}
		}
		<-server.synthesisSem
		<-server.batchSem
		if !synthesisDeadline {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/analyze", strings.NewReader(`{"text":"あ"}`)))
			if response.Code != 200 || len(server.synthesisSem) != 0 {
				t.Fatalf("next analysis leaked slot: %d %s", response.Code, response.Body.String())
			}
		}
	}
}

func TestInvalidLimitConfigurationIsRejected(t *testing.T) {
	for _, limits := range []Limits{
		{MaxJSONRequestBytes: -1}, {MaxTextRunes: -1}, {MaxBatchItems: -1}, {MaxBatchWAVBytes: -1},
		{MaxManualPitchPoints: -1}, {MaxConcurrentSynthesis: -1}, {MaxConcurrentBatches: -1},
		{QueueTimeout: -time.Second}, {SynthesisTimeout: -time.Second},
	} {
		server, err := New(Config{VoiceDir: t.TempDir(), Limits: limits})
		if err == nil {
			_ = server.Close()
			t.Fatalf("invalid limits accepted: %#v", limits)
		}
	}
}
