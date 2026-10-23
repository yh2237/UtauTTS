package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("error is not JSON: %v, body=%q", err, response.Body.String())
	}
	if response.Code != status || body.Code != code || body.Error == "" || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("error response = %d %#v, want %d %s", response.Code, body, status, code)
	}
	if !slices.Contains(readSpec(t).Components.Schemas["Error"].Properties["code"].Enum, body.Code) {
		t.Errorf("response code %q is not documented", body.Code)
	}
}

func TestAPIErrorCodesPreserveLegacyMessageAndStatus(t *testing.T) {
	server := newWaveformTestServer(t)
	for _, test := range []struct {
		path, body, code string
		status           int
	}{
		{"/api/analyze", `{"unknown":true}`, "invalid_request", 400},
		{"/api/synthesize/audio", `{}`, "invalid_request", 400},
		{"/api/synthesize/audio", `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test","reading":"あ","mora_duration_ms":1001}`, "invalid_request", 400},
		{"/api/synthesize/audio", `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test","reading":"あ","alias_policy":"missing"}`, "synthesis_error", 422},
		{"/api/synthesize/audio", `{"method":"waveform-synthesis","language":"ja","voicebank_id":"test","reading":"` + strings.Repeat("あ", maxTextRunes+1) + `"}`, "limit_exceeded", 413},
	} {
		t.Run(test.path+"/"+test.code+"/"+test.body[:min(len(test.body), 30)], func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, test.path, strings.NewReader(test.body)))
			assertAPIError(t, response, test.status, test.code)
		})
	}
}

func TestAPIClassifiesCanceledAndExpiredRequests(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		var ctx context.Context
		var cancel context.CancelFunc
		code := "canceled"
		if deadline {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			code = "deadline_exceeded"
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		cancel()
		server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
		for _, path := range []string{"/api/analyze", "/api/synthesize/audio", "/api/synthesize/label", "/api/synthesize/batch"} {
			body := `{"text":"あ"}`
			if strings.HasSuffix(path, "/batch") {
				body = `{"items":[{"request":{"method":"waveform-synthesis","language":"ja","reading":"あ"}}]}`
			}
			response := httptest.NewRecorder()
			request := localRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
			server.Handler().ServeHTTP(response, request)
			assertAPIError(t, response, 408, code)
		}
	}
}

func TestClosedServerRejectsSynthesis(t *testing.T) {
	server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/audio", strings.NewReader(`{"method":"waveform-synthesis","language":"ja","reading":"あ"}`)))
	assertAPIError(t, response, 503, "unavailable")
}
