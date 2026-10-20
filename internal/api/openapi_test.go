package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	Required   []string              `yaml:"required"`
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

func TestOpenAPIPaths(t *testing.T) {
	doc := readSpec(t)
	handler := newWaveformTestServer(t).Handler()
	for path, operations := range doc.Paths {
		for method := range operations {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, localRequest(strings.ToUpper(method), path, strings.NewReader(`{}`)))
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed || response.Code >= 500 {
				t.Errorf("documented route %s %s returned %d: %s", method, path, response.Code, response.Body.String())
			}
		}
	}
}

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
	required := map[string]bool{}
	for _, field := range doc.Components.Schemas["SynthesisRequest"].Required {
		required[field] = true
	}
	for field, property := range doc.Components.Schemas["SynthesisRequest"].Properties {
		if property.Default == nil {
			if !required[field] && (property.Type == "string" || property.Type == "number" || property.Type == "integer" || property.Type == "boolean") {
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
