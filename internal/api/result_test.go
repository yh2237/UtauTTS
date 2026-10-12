package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONSynthesisResultMatchesWAVAndLAB(t *testing.T) {
	server := newWaveformTestServer(t)
	if err := os.WriteFile(filepath.Join(server.voicebanks["test"].Path, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\na.wav=tA,0,0,0,0,0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"reading":"あ","apply_pitch":false}`,
		`{"reading":"S T AA1","language":"en","phonemizer":"en-delta","apply_pitch":false}`,
	} {
		responses := make(map[string]*httptest.ResponseRecorder)
		for _, kind := range []string{"audio", "label", "result"} {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodPost, "/api/synthesize/"+kind, strings.NewReader(input)))
			if response.Code != 200 {
				t.Fatalf("%s: status=%d body=%s", kind, response.Code, response.Body.String())
			}
			responses[kind] = response
		}
		var result synthesisResponse
		if err := json.Unmarshal(responses["result"].Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Audio.Format != "wav_pcm_s16le" || !bytes.Equal(result.Audio.Data, responses["audio"].Body.Bytes()) || result.Result.PhonemeLabels != responses["label"].Body.String() || result.Result.RendererID != "waveform" || result.Result.UnitCount == 0 {
			t.Fatalf("JSON result differs from binary endpoints: %#v", result.Result)
		}
		if result.Result.Diagnostics == nil {
			t.Fatal("diagnostics must be an array")
		}
		if strings.Contains(input, "S T AA1") {
			if len(result.Result.Diagnostics) == 0 || responses["audio"].Header().Get("X-UtauTTS-Diagnostic-Count") == "0" {
				t.Fatal("missing phones were not reported")
			}
		} else if len(result.Result.Diagnostics) != 0 || responses["audio"].Header().Get("X-UtauTTS-Diagnostic-Count") != "0" {
			t.Fatalf("unexpected diagnostics: %#v", result.Result.Diagnostics)
		}
	}
}

func TestReadinessIsSeparateFromHealth(t *testing.T) {
	server := newWaveformTestServer(t)
	for _, expected := range []int{200, 503} {
		if expected == 503 {
			server.voicebanks = map[string]Voicebank{}
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/readiness", nil))
		if response.Code != expected {
			t.Fatalf("readiness = %d %s", response.Code, response.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["renderer_id"] != "waveform" || payload["renderer"].(map[string]any)["available"] != true {
			t.Fatalf("readiness = %#v", payload)
		}
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/health", nil))
		if response.Code != 200 {
			t.Fatalf("health = %d", response.Code)
		}
	}
	server = newWaveformTestServer(t)
	server.renderer = "missing"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/readiness", nil))
	if response.Code != 503 || !strings.Contains(response.Body.String(), `"available":false`) {
		t.Fatalf("missing renderer readiness = %d %s", response.Code, response.Body.String())
	}
}
