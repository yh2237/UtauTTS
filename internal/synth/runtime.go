package synth

import (
	"fmt"

	"utautts/internal/plugin"
	"utautts/internal/render"
)

// RuntimeConfig contains the process-level dependencies shared by every
// synthesis entry point.
type RuntimeConfig struct {
	Renderer                              string
	WorldlineBridgePath                   string
	OpenJTalkPath                         string
	OpenJTalkDictionary                   string
	RendererDirectories, ModelDirectories []string
}

// Runtime owns the discovered plugin catalog and the service built from it.
type Runtime struct {
	Catalog  *plugin.Catalog
	Renderer string
	Service  *Service
}

// NewRuntime centralizes plugin discovery and default-renderer normalization
// for GUI, HTTP, and command-line hosts.
func NewRuntime(config RuntimeConfig, voicebanks VoicebankResolver) (*Runtime, error) {
	catalog, err := plugin.DiscoverWithDefaults(config.RendererDirectories, config.ModelDirectories, render.IsKnownRenderer)
	if err != nil {
		return nil, fmt.Errorf("discover renderers: %w", err)
	}
	if renderer, ok := catalog.Renderer(config.Renderer); ok {
		config.Renderer = renderer.ID
	}
	return &Runtime{
		Catalog: catalog, Renderer: config.Renderer,
		Service: NewService(catalog, config.Renderer, config.WorldlineBridgePath,
			config.OpenJTalkPath, config.OpenJTalkDictionary, voicebanks),
	}, nil
}
