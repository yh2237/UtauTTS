package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type specSchema struct {
	Type       string                `yaml:"type"`
	Default    any                   `yaml:"default"`
	Properties map[string]specSchema `yaml:"properties"`
	Enum       []string              `yaml:"enum"`
	MaxLength  int                   `yaml:"x-default-maxLength"`
	MaxItems   int                   `yaml:"x-default-maxItems"`
}

type specDocument struct {
	OpenAPI string `yaml:"openapi"`
	Paths   map[string]map[string]struct {
		OperationID string `yaml:"operationId"`
	} `yaml:"paths"`
	Components struct {
		Schemas map[string]specSchema `yaml:"schemas"`
	} `yaml:"components"`
}

func readSpec(t *testing.T) specDocument {
	t.Helper()
	var doc specDocument
	if err := yaml.Unmarshal(openAPISpec, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version=%q", doc.OpenAPI)
	}
	return doc
}

func TestOpenAPIEndpointIsServedLocally(t *testing.T) {
	server := newWaveformTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/openapi.yaml", nil))
	if response.Code != 200 || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/yaml") || !bytes.Equal(response.Body.Bytes(), openAPISpec) {
		t.Fatalf("OpenAPI response: %d %v", response.Code, response.Header())
	}
}

// Keep this list in sync when the HTTP surface changes. It is intentionally a
// plain list instead of parsing the handler source, so the test stays readable.
func TestOpenAPIPaths(t *testing.T) {
	doc := readSpec(t)
	expected := map[string][]string{
		"/api/openapi.yaml":      {http.MethodGet},
		"/api/health":            {http.MethodGet},
		"/api/voicebanks":        {http.MethodGet, http.MethodPost},
		"/api/voicebanks/reload": {http.MethodPost},
		"/api/models":            {http.MethodGet},
		"/api/renderers":         {http.MethodGet},
		"/api/analyze":           {http.MethodPost},
		"/api/synthesize/audio":  {http.MethodPost},
		"/api/synthesize/label":  {http.MethodPost},
		"/api/synthesize/batch":  {http.MethodPost},
	}
	if len(doc.Paths) != len(expected) {
		t.Errorf("documented paths=%d expected=%d", len(doc.Paths), len(expected))
	}
	for path, methods := range expected {
		documented, ok := doc.Paths[path]
		if !ok {
			t.Errorf("path not documented: %s", path)
			continue
		}
		got := make([]string, 0, len(documented))
		for method := range documented {
			got = append(got, strings.ToLower(method))
		}
		for index := range methods {
			methods[index] = strings.ToLower(methods[index])
		}
		sort.Strings(got)
		sort.Strings(methods)
		if !reflect.DeepEqual(got, methods) {
			t.Errorf("%s methods=%v expected=%v", path, got, methods)
		}
	}
}

// Decoded defaults and the fixed limits are the values a client actually sees.
func TestOpenAPIDefaultsMatchDecodedRequests(t *testing.T) {
	doc := readSpec(t)
	var request SynthesisRequest
	if err := json.Unmarshal([]byte(`{}`), &request); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatal(err)
	}
	for field, property := range doc.Components.Schemas["SynthesisRequest"].Properties {
		if property.Default == nil {
			if property.Type == "string" || property.Type == "number" || property.Type == "integer" || property.Type == "boolean" {
				t.Errorf("missing scalar default: %s", field)
			}
			continue
		}
		defaultJSON, err := json.Marshal(property.Default)
		if err != nil {
			t.Fatal(err)
		}
		var expected any
		if err := json.Unmarshal(defaultJSON, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(values[field], expected) {
			t.Errorf("%s: decoded=%#v schema default=%#v", field, values[field], expected)
		}
	}
	for _, field := range []string{"text", "reading", "kana"} {
		if doc.Components.Schemas["SynthesisRequest"].Properties[field].MaxLength != maxTextRunes {
			t.Errorf("default max length differs: %s", field)
		}
	}
	for _, check := range []struct {
		schema, field string
		limit         int
	}{
		{"SynthesisRequest", "mora_durations_ms", maxTextRunes},
		{"SynthesisRequest", "resampler_expressions", maxTextRunes},
		{"ManualPitch", "points", maxManualPitchPoints},
		{"BatchRequest", "items", maxBatchItems},
	} {
		if doc.Components.Schemas[check.schema].Properties[check.field].MaxItems != check.limit {
			t.Errorf("default max items differs: %s.%s", check.schema, check.field)
		}
	}
}

// Keep this list in sync with the codes writeAPIError can return.
func TestOpenAPIErrorCodes(t *testing.T) {
	doc := readSpec(t)
	expected := []string{"invalid_request", "limit_exceeded", "unavailable", "synthesis_error", "canceled", "deadline_exceeded", "forbidden", "internal_error"}
	got := append([]string(nil), doc.Components.Schemas["Error"].Properties["code"].Enum...)
	sort.Strings(expected)
	sort.Strings(got)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("error codes=%v expected=%v", got, expected)
	}
}

// Every local $ref must point to a node in this document.
func TestOpenAPILocalReferencesResolve(t *testing.T) {
	var document map[string]any
	if err := yaml.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if ref, ok := value["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/") {
					t.Errorf("unexpected external ref: %s", ref)
				} else {
					var target any = document
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
						object, ok := target.(map[string]any)
						if !ok {
							target = nil
							break
						}
						target = object[part]
					}
					if target == nil {
						t.Errorf("unresolved ref: %s", ref)
					}
				}
			}
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(document)
}
