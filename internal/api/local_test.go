package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalHandlerRejectsNonLocalHosts(t *testing.T) {
	handler := newWaveformTestServer(t).Handler()
	for _, host := range []string{"remote.example:8080", "127.0.0.1.remote.example", "192.168.1.10:8080", "0.0.0.0:8080", "[::]:8080", ""} {
		for _, path := range []string{"/", "/api/health", "/api/openapi.yaml"} {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Host = host
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assertAPIError(t, response, 403, "forbidden")
		}
	}
	for _, host := range []string{"127.0.0.1:8080", "127.0.0.2", "localhost:8080", "LOCALHOST", "[::1]:8080", "[::1]", "[::ffff:127.0.0.1]:8080"} {
		request := localRequest(http.MethodGet, "/api/health", nil)
		request.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("local Host %q rejected: %d %s", host, response.Code, response.Body.String())
		}
	}
}

func TestLocalAPIRejectsCrossSiteOperationsWithoutToken(t *testing.T) {
	handler := newWaveformTestServer(t).Handler()
	for _, header := range []struct{ name, value string }{
		{"Origin", "https://other.example"},
		{"Origin", "null"},
		{"Origin", "http://127.0.0.1:9999"},
		{"Sec-Fetch-Site", "cross-site"},
	} {
		for _, path := range []string{"/api/analyze", "/api/synthesize/audio", "/api/voicebanks/reload"} {
			request := localRequest(http.MethodPost, path, strings.NewReader(`{"text":"あ"}`))
			request.Header.Set(header.name, header.value)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assertAPIError(t, response, 403, "forbidden")
		}
	}
	for _, browser := range []bool{false, true} {
		request := localRequest(http.MethodPost, "/api/analyze", strings.NewReader(`{"text":"あ"}`))
		if browser {
			request.Header.Set("Origin", "http://127.0.0.1")
			request.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("local operation rejected: %d %s", response.Code, response.Body.String())
		}
	}
}
