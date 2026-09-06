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
		`{"manifest_version":2,"kind":"synthesis-engine","id":"example.wave","display_name":"Example","contract":"unit-renderer","provider":"waveform","provider_version":"1","resources":{"engine":{"required":true}}}`,
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

func TestRendererManifestV2UsesExplicitProviderAndTypedResources(t *testing.T) {
	directory := t.TempDir()
	document := `{"manifest_version":2,"kind":"synthesis-engine","id":"example.wave","display_name":"Example","contract":"unit-renderer","provider":"waveform","provider_version":"1","resources":{"engine":{"path":"runtime/engine.dll","required":true}},"platform_resources":{"any":{"engine":{"path":"runtime/engine-any.bin","required":true,"executable":true}}}}`
	if err := os.WriteFile(filepath.Join(directory, "renderer.json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := DiscoverRenderers([]string{directory}, func(provider string) bool { return provider == "waveform" })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("v2 renderers = %#v", items)
	}
	item := items[0]
	if item.ManifestVersion != 2 || item.Kind != "synthesis-engine" || item.Provider != "waveform" || item.Contract != "unit-renderer" || item.ProviderVersion != "1" || item.Backend != "" {
		t.Fatalf("v2 metadata = %#v", item)
	}
	resource := item.Resource("engine")
	if resource.Path != filepath.Join(directory, "runtime", "engine-any.bin") || !resource.Required || !resource.Executable {
		t.Fatalf("v2 resource = %#v", resource)
	}
}

func TestRendererManifestV2AcceptsExternalProviderSession(t *testing.T) {
	directory := t.TempDir()
	document := `{"manifest_version":2,"kind":"synthesis-engine","id":"example.external","display_name":"External","contract":"unit-renderer","contract_version":1,"provider":"example-provider","provider_version":"1","protocol":"utautts-provider","protocol_version":1,"provider_args":["--provider"],"resources":{"provider_executable":{"path":"bin/provider.exe","required":true,"executable":true}}}`
	if err := os.MkdirAll(filepath.Join(directory, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "renderer.json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := DiscoverRenderers([]string{directory}, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("external renderers = %#v", items)
	}
	item := items[0]
	if item.Protocol != "utautts-provider" || item.ProtocolVersion != 1 || item.ContractVersion != 1 || len(item.ProviderArgs) != 1 {
		t.Fatalf("external metadata = %#v", item)
	}
	resource := item.Resource("provider_executable")
	if resource.Path != filepath.Join(directory, "bin", "provider.exe") || !resource.Executable {
		t.Fatalf("external executable = %#v", resource)
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
