package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRendererManifestUsesDeclaredMetadata(t *testing.T) {
	const document = `{"manifest_version":1,"kind":"renderer","id":"example.wave","display_name":"Example","version":"1.0","backend":"waveform","acceleration":"cpu","capabilities":{"frame_pitch":true},"assets":{"engine":"../../runtime/engine"}}`
	item, err := decodeRenderer([]byte(document), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "example.wave" || item.Backend != "waveform" || item.Acceleration != "cpu" || !item.Capabilities.FramePitch {
		t.Fatalf("manifest metadata = %#v", item)
	}
	if item.Assets["engine"] != "../../runtime/engine" {
		t.Fatalf("manifest assets = %#v", item.Assets)
	}
}

func TestRendererManifestRejectsLegacyAndUnknownFields(t *testing.T) {
	for _, document := range []string{
		`{"manifest_version":2,"kind":"renderer","id":"example.wave","display_name":"Example","backend":"waveform"}`,
		`{"manifest_version":1,"kind":"renderer","id":"example.wave","display_name":"Example","backend":"waveform","protocol_version":1}`,
	} {
		if _, err := decodeRenderer([]byte(document), t.TempDir()); err == nil {
			t.Fatalf("legacy or unknown field was accepted: %s", document)
		}
	}
}

func TestRendererManifestV1PlatformAssetsSelectCurrentPlatform(t *testing.T) {
	directory := t.TempDir()
	document := `{"manifest_version":1,"kind":"renderer","id":"example.wave","display_name":"Example","version":"1","backend":"waveform","platform_assets":{"any":{"engine":"any-engine"}}}`
	item, err := decodeRenderer([]byte(document), directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(item.Asset("engine")); got != "any-engine" {
		t.Fatalf("asset = %q", got)
	}
}

func TestRendererManifestUnsupportedPlatformIsSkipped(t *testing.T) {
	directory := t.TempDir()
	document := `{"manifest_version":1,"kind":"renderer","id":"example.future","display_name":"Future","version":"1","backend":"waveform","platforms":["plan9-amd64"]}`
	item, err := decodeRenderer([]byte(document), directory)
	if err != nil {
		t.Fatal(err)
	}
	if rendererSupportedOnCurrentPlatform(item) {
		t.Fatalf("unsupported platform was accepted: %#v", item.Platforms)
	}
	if err := os.WriteFile(filepath.Join(directory, "renderer.json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := DiscoverRenderers([]string{directory}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("unsupported platform was catalogued: %#v", items)
	}
}
