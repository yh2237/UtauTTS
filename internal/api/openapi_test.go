package api

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	utautts "github.com/yh2237/UtauTTS"
	"github.com/yh2237/UtauTTS/internal/engine"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"gopkg.in/yaml.v3"
)

type specSchema struct {
	Ref        string                `yaml:"$ref"`
	Type       string                `yaml:"type"`
	Default    any                   `yaml:"default"`
	Properties map[string]specSchema `yaml:"properties"`
	Required   []string              `yaml:"required"`
	Enum       []string              `yaml:"enum"`
	MaxLength  int                   `yaml:"x-default-maxLength"`
	MaxItems   int                   `yaml:"x-default-maxItems"`
}

type specDocument struct {
	OpenAPI string `yaml:"openapi"`
	Info    struct {
		MaxJSONBytes int64 `yaml:"x-default-max-json-bytes"`
	} `yaml:"info"`
	Paths map[string]map[string]struct {
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

func TestOpenAPIEndpointIsAvailableLocally(t *testing.T) {
	server := newWaveformTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/openapi.yaml", nil))
	if response.Code != 200 || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/yaml") || !bytes.Equal(response.Body.Bytes(), openAPISpec) {
		t.Fatalf("OpenAPI response: %d %v", response.Code, response.Header())
	}
}

func TestOpenAPIRoutesMatchRegisteredAPI(t *testing.T) {
	doc := readSpec(t)
	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	registered := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "HandleFunc" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		pattern, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		method, path, ok := strings.Cut(pattern, " ")
		if ok && strings.HasPrefix(path, "/api/") {
			registered[strings.ToLower(method)+" "+path] = true
		}
		return true
	})
	operationIDs := make(map[string]bool)
	for path, methods := range doc.Paths {
		for method, operation := range methods {
			key := method + " " + path
			if !registered[key] {
				t.Errorf("OpenAPI route not registered: %s", key)
			}
			delete(registered, key)
			if operation.OperationID == "" || operationIDs[operation.OperationID] {
				t.Errorf("missing/duplicate operationId for %s", key)
			}
			operationIDs[operation.OperationID] = true
		}
	}
	for route := range registered {
		t.Errorf("registered API not documented: %s", route)
	}
}

// Compare the documented fields with actual wire types, not a second copy of
// request decoding or synthesis. Anonymous compatibility fields are included.
func TestOpenAPISchemaFieldsMatchWireTypes(t *testing.T) {
	doc := readSpec(t)
	for _, test := range []struct {
		name     string
		value    any
		response bool
	}{
		{"SynthesisRequest", SynthesisRequest{}, false},
		{"DictionaryEntry", utautts.DictionaryEntry{}, false},
		{"ManualPitch", prosody.ManualPitchFile{}, false},
		{"ManualPitchPoint", prosody.ManualPitchPoint{}, false},
		{"PitchCurve", render.PitchCurve{}, false},
		{"UnitOverride", plan.UnitOverride{}, false},
		{"ResamplerExpression", render.ResamplerExpression{}, false},
		{"Worldline", render.WorldlineProviderOptions{}, false},
		{"SynthesisResponse", synthesisResponse{}, true},
		{"SynthesisReport", synth.Report{}, true},
		{"Diagnostic", render.RenderDiagnostic{}, true},
		{"Error", errorResponse{}, true},
		{"Voicebank", Voicebank{}, true},
		{"SubbankOption", voicebank.SubbankOption{}, true},
		{"Model", plugin.Model{}, true},
		{"Renderer", plugin.Renderer{}, true},
		{"RendererSetting", plugin.RendererSetting{}, true},
		{"RendererSettingValue", plugin.RendererSettingValue{}, true},
		{"RendererResource", plugin.RendererResource{}, true},
		{"Capabilities", plugin.Capabilities{}, true},
		{"ClassicTool", plugin.ClassicTool{}, true},
		{"Availability", engine.Availability{}, true},
		{"AvailabilityIssue", engine.AvailabilityIssue{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema, ok := doc.Components.Schemas[test.name]
			if !ok {
				t.Fatalf("schema %s missing", test.name)
			}
			fields := wireFields(reflect.TypeOf(test.value))
			if len(fields) != len(schema.Properties) {
				t.Errorf("%s fields: wire=%d schema=%d", test.name, len(fields), len(schema.Properties))
			}
			var required []string
			for name, field := range fields {
				property, ok := schema.Properties[name]
				if !ok {
					t.Errorf("wire field not documented: %s", name)
					continue
				}
				if property.Ref != "" {
					property = doc.Components.Schemas[strings.TrimPrefix(property.Ref, "#/components/schemas/")]
				}
				if kind := schemaKind(field.Type); kind != "" && property.Type != kind {
					t.Errorf("%s type: schema=%s wire=%s", name, property.Type, kind)
				}
				if !strings.Contains(field.Tag.Get("json"), ",omitempty") {
					required = append(required, name)
				}
			}
			for name := range schema.Properties {
				if _, ok := fields[name]; !ok {
					t.Errorf("documented field not on wire: %s", name)
				}
			}
			if test.response {
				slices.Sort(required)
				slices.Sort(schema.Required)
				if !slices.Equal(required, schema.Required) {
					t.Errorf("required fields: wire=%v schema=%v", required, schema.Required)
				}
			}
		})
	}
}

func wireFields(typ reflect.Type) map[string]reflect.StructField {
	fields := make(map[string]reflect.StructField)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			for name, child := range wireFields(field.Type) {
				fields[name] = child
			}
			continue
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field
	}
	return fields
}

func schemaKind(typ reflect.Type) string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int64:
		return "integer"
	case reflect.Float64:
		return "number"
	case reflect.Slice:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	default:
		return ""
	}
}

func TestOpenAPIDefaultsMatchDecodedRequestsAndLimits(t *testing.T) {
	doc := readSpec(t)
	var request SynthesisRequest
	if err := json.Unmarshal([]byte(`{}`), &request); err != nil {
		t.Fatal(err)
	}
	assertDefaults := func(name string, value any) {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]any
		if err := json.Unmarshal(encoded, &values); err != nil {
			t.Fatal(err)
		}
		for field, property := range doc.Components.Schemas[name].Properties {
			if property.Default == nil {
				if name == "SynthesisRequest" && (property.Type == "string" || property.Type == "number" || property.Type == "integer" || property.Type == "boolean") {
					t.Errorf("missing scalar default: %s.%s", name, field)
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
				t.Errorf("%s.%s: decoded=%#v schema default=%#v", name, field, values[field], expected)
			}
		}
	}
	assertDefaults("SynthesisRequest", request)
	limits := DefaultLimits()
	assertDefaults("Limits", struct {
		Limits
		QueueTimeout     string `json:"queue_timeout"`
		SynthesisTimeout string `json:"synthesis_timeout"`
	}{limits, limits.QueueTimeout.String(), limits.SynthesisTimeout.String()})
	if doc.Info.MaxJSONBytes != limits.MaxJSONRequestBytes {
		t.Errorf("default request bytes: spec=%d runtime=%d", doc.Info.MaxJSONBytes, limits.MaxJSONRequestBytes)
	}
	for _, field := range []string{"text", "reading", "kana"} {
		if doc.Components.Schemas["SynthesisRequest"].Properties[field].MaxLength != limits.MaxTextRunes {
			t.Errorf("default max length differs: %s", field)
		}
	}
	for _, check := range []struct {
		schema, field string
		limit         int
	}{
		{"SynthesisRequest", "mora_durations_ms", limits.MaxTextRunes},
		{"SynthesisRequest", "resampler_expressions", limits.MaxTextRunes},
		{"ManualPitch", "points", limits.MaxManualPitchPoints},
		{"BatchRequest", "items", limits.MaxBatchItems},
	} {
		if doc.Components.Schemas[check.schema].Properties[check.field].MaxItems != check.limit {
			t.Errorf("default max items differs: %s.%s", check.schema, check.field)
		}
	}
}

func TestOpenAPIErrorCodesMatchHandler(t *testing.T) {
	doc := readSpec(t)
	file, err := parser.ParseFile(token.NewFileSet(), "response.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
			return true
		}
		for i, lhs := range assignment.Lhs {
			identifier, ok := lhs.(*ast.Ident)
			if !ok || identifier.Name != "code" {
				continue
			}
			literal, ok := assignment.Rhs[i].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			code, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			codes[code] = true
		}
		return true
	})
	for _, code := range doc.Components.Schemas["Error"].Properties["code"].Enum {
		if !codes[code] {
			t.Errorf("documented code not returned by handler: %s", code)
		}
		delete(codes, code)
	}
	for code := range codes {
		t.Errorf("handler code not documented: %s", code)
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
