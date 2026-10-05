package synth

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"utautts/internal/render"
	"utautts/internal/tts"
)

// manifestの既定値とrendererSettingSpecsの既定値がずれていないことを確認する。
func TestManifestSettingDefaultsMatchSpecs(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "renderer", "*", "renderer.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no bundled renderer manifests found")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Settings []struct {
				ID      string `json:"id"`
				Default any    `json:"default"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, setting := range manifest.Settings {
			spec, found := rendererSettingSpecFor(rendererSettingSpecs, setting.ID)
			if !found {
				t.Errorf("%s: setting %q is not in rendererSettingSpecs", path, setting.ID)
				continue
			}
			if !equalSettingDefault(spec.defaultValue, setting.Default) {
				t.Errorf("%s: setting %q default = %v, want %v", path, setting.ID, setting.Default, spec.defaultValue)
			}
		}
	}
}

// DefaultRequestの値が全specの既定値と同じ解決結果になることを確認する。
func TestDefaultRequestCoversSettingDefaults(t *testing.T) {
	request := DefaultRequest()
	for _, spec := range rendererSettingSpecs {
		if spec.typed == nil {
			continue
		}
		var defaultConfig, requestConfig tts.Config
		var defaultOptions, requestOptions render.ProviderOptions
		var defaultResolution, requestResolution rendererSettingsResolution
		spec.apply(spec.defaultValue, &defaultConfig, &defaultOptions, &defaultResolution)
		spec.apply(spec.typed(request), &requestConfig, &requestOptions, &requestResolution)
		if !reflect.DeepEqual(defaultConfig, requestConfig) ||
			!reflect.DeepEqual(defaultOptions, requestOptions) ||
			defaultResolution != requestResolution {
			t.Errorf("setting %q: DefaultRequest does not match the spec default", spec.id)
		}
	}
}

func equalSettingDefault(specDefault, manifestDefault any) bool {
	specNumber, specOK := floatSettingValue(specDefault)
	manifestNumber, manifestOK := floatSettingValue(manifestDefault)
	if specOK && manifestOK {
		return math.Abs(specNumber-manifestNumber) < 1e-9
	}
	return reflect.DeepEqual(specDefault, manifestDefault)
}

func floatSettingValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
