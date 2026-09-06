package engine

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"utautts/internal/plugin"
)

func TestDefinitionFromV1SeparatesPublicIDProviderAndResources(t *testing.T) {
	directory := t.TempDir()
	renderer := plugin.Renderer{
		ManifestVersion: 1,
		ID:              "world-phrase-fast",
		DisplayName:     "World phrase fast",
		Backend:         "utautts-world-phrase",
		Version:         "renderer-definition-2",
		Capabilities:    plugin.Capabilities{FramePitch: true},
		Directory:       directory,
		PlatformAssets: map[string]map[string]string{
			runtime.GOOS + "-" + runtime.GOARCH: {
				"world_engine": "runtime/world-engine",
			},
		},
	}

	definition := DefinitionFromV1(renderer)
	if definition.ID != "world-phrase-fast" {
		t.Fatalf("public id = %q", definition.ID)
	}
	if definition.Provider != "utautts-world-phrase" {
		t.Fatalf("provider = %q", definition.Provider)
	}
	if definition.ProviderVersion != "" {
		t.Fatalf("v1 renderer version was treated as provider version: %q", definition.ProviderVersion)
	}
	if definition.Contract != ContractUnitRenderer {
		t.Fatalf("contract = %q", definition.Contract)
	}
	if got, want := definition.Resource(ResourceWorldEngine), filepath.Join(directory, "runtime", "world-engine"); got != want {
		t.Fatalf("world engine resource = %q, want %q", got, want)
	}
	if !definition.Capabilities.FramePitch {
		t.Fatal("frame pitch capability was lost")
	}
}

func TestDefinitionFromV2UsesExplicitContractProviderAndResources(t *testing.T) {
	directory := t.TempDir()
	renderer := plugin.Renderer{
		ManifestVersion: 2,
		Kind:            "synthesis-engine",
		ID:              "friendly-world",
		DisplayName:     "Friendly WORLD",
		Contract:        string(ContractUnitRenderer),
		Provider:        "utautts-world-phrase",
		ProviderVersion: "1",
		Directory:       directory,
		Resources:       map[string]plugin.RendererResource{"world_engine": {Path: "runtime/world-engine"}},
	}

	definition := DefinitionFromV2(renderer)
	if definition.ID != "friendly-world" || definition.Provider != "utautts-world-phrase" || definition.Contract != ContractUnitRenderer || definition.ProviderVersion != "1" {
		t.Fatalf("v2 definition = %#v", definition)
	}
	if got, want := definition.Resource(ResourceWorldEngine), filepath.Join(directory, "runtime", "world-engine"); got != want {
		t.Fatalf("v2 world engine resource = %q, want %q", got, want)
	}
}

func TestResolverAcceptsManifestDeclaredExternalProvider(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "provider")
	if err := os.WriteFile(executable, []byte("provider"), 0o700); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{
		ID: "external-world", Contract: ContractUnitRenderer, ContractVersion: 1,
		Provider: "example.world", ProviderVersion: "7", Protocol: "utautts-provider", ProtocolVersion: 1,
		Capabilities: Capabilities{FramePitch: true},
		Resources:    map[ResourceKey]string{ResourceProviderExecutable: executable},
	}
	resolved, err := NewResolver(registry).Resolve([]Definition{definition}, "external-world")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Provider.ID != "example.world" || resolved.Provider.Version != "7" || !resolved.Availability.Available {
		t.Fatalf("external provider resolution = %#v", resolved)
	}
}

func TestResolverUsesDefaultOnlyForEmptyPublicID(t *testing.T) {
	registry, err := NewRegistry(
		Provider{ID: "builtin.first", Contract: ContractUnitRenderer, Version: "1"},
		Provider{ID: "builtin.second", Contract: ContractNeuralSynthesizer, Version: "1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(registry)
	definitions := []Definition{
		{ID: "default", Contract: ContractUnitRenderer, Provider: "builtin.first"},
		{ID: "neural", Contract: ContractNeuralSynthesizer, Provider: "builtin.second"},
	}

	resolved, err := resolver.Resolve(definitions, "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.PublicID() != "default" || resolved.Provider.ID != "builtin.first" {
		t.Fatalf("default resolution = %#v", resolved)
	}

	resolved, err = resolver.Resolve(definitions, "neural")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.PublicID() != "neural" || resolved.Provider.Contract != ContractNeuralSynthesizer {
		t.Fatalf("explicit resolution = %#v", resolved)
	}

	if _, err := resolver.Resolve(definitions, "removed"); !errors.Is(err, ErrDefinitionNotFound) {
		t.Fatalf("missing explicit renderer error = %v", err)
	}
}

func TestResolverRejectsProviderWithWrongContract(t *testing.T) {
	registry, err := NewRegistry(Provider{ID: "builtin.neural", Contract: ContractNeuralSynthesizer, Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewResolver(registry).Resolve([]Definition{{
		ID: "incorrect", Contract: ContractUnitRenderer, Provider: "builtin.neural",
	}}, "incorrect")
	if !errors.Is(err, ErrContractMismatch) {
		t.Fatalf("contract mismatch error = %v", err)
	}
}

func TestResolverRejectsProviderVersionMismatch(t *testing.T) {
	registry, err := NewRegistry(Provider{ID: "builtin.waveform", Contract: ContractUnitRenderer, Version: "2"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewResolver(registry).Resolve([]Definition{{
		ID: "old-definition", Contract: ContractUnitRenderer, Provider: "builtin.waveform", ProviderVersion: "1",
	}}, "old-definition")
	if !errors.Is(err, ErrProviderVersionMismatch) {
		t.Fatalf("provider version mismatch error = %v", err)
	}
}

func TestResolverRejectsDefinitionCapabilityUnsupportedByProvider(t *testing.T) {
	registry, err := NewRegistry(Provider{
		ID: "builtin.basic", Contract: ContractUnitRenderer, Version: "1",
		Capabilities: Capabilities{FramePitch: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewResolver(registry).Resolve([]Definition{{
		ID: "incorrect", Contract: ContractUnitRenderer, Provider: "builtin.basic",
		Capabilities: Capabilities{FramePitch: true, BoundaryBridge: true},
	}}, "incorrect")
	if !errors.Is(err, ErrCapabilityMismatch) {
		t.Fatalf("capability mismatch error = %v", err)
	}
}

func TestResolverReportsMissingRequiredResourceAndHonorsOverride(t *testing.T) {
	registry, err := NewRegistry(Provider{
		ID: "builtin.requires-bridge", Contract: ContractUnitRenderer, Version: "1",
		Requirements: []ResourceRequirement{{Key: ResourceWorldlineBridge, Required: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(registry)
	definitions := []Definition{{
		ID: "requires-bridge", Contract: ContractUnitRenderer, Provider: "builtin.requires-bridge",
	}}

	resolved, err := resolver.Resolve(definitions, "requires-bridge")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Availability.Available {
		t.Fatalf("missing resource was reported as available: %#v", resolved.Availability)
	}
	if err := resolved.RequireAvailable(); !errors.Is(err, ErrResourcesUnavailable) {
		t.Fatalf("required resource error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "bridge")
	if err := os.WriteFile(path, []byte("bridge"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err = resolver.ResolveWithOptions(definitions, "requires-bridge", ResolveOptions{
		ResourceOverrides: map[ResourceKey]string{ResourceWorldlineBridge: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Availability.Available {
		t.Fatalf("override availability = %#v", resolved.Availability)
	}
}

func TestBundledManifestDefinitionsResolveAgainstBuiltinRegistry(t *testing.T) {
	directories, _ := plugin.DefaultDirectories()
	renderers, err := plugin.DiscoverRenderers(directories, IsBuiltinProvider)
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(BuiltinRegistry())
	for _, definition := range DefinitionsFromCatalog(&plugin.Catalog{Renderers: renderers}) {
		resolved, resolveErr := resolver.Resolve([]Definition{definition}, string(definition.ID))
		if resolveErr != nil {
			t.Fatalf("resolve bundled renderer %q: %v", definition.ID, resolveErr)
		}
		if resolved.Provider.ID != definition.Provider {
			t.Fatalf("renderer %q provider = %q, want %q", definition.ID, resolved.Provider.ID, definition.Provider)
		}
		if !resolved.Availability.Available {
			t.Fatalf("bundled renderer %q is missing runtime resources: %s", definition.ID, resolved.Availability.Error())
		}
	}
}
