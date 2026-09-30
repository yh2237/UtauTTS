package synth

import (
	"fmt"

	"utautts/internal/plugin"
	"utautts/internal/render"
)

type RuntimeConfig struct {
	Renderer                              string
	WorldlineBridgePath                   string
	OpenJTalkPath                         string
	OpenJTalkDictionary                   string
	RendererDirectories, ModelDirectories []string
}

type Runtime struct {
	Catalog  *plugin.Catalog
	Renderer string
	Service  *Service
}

// NewRuntimeはGUI・HTTP・CLIで共有するプラグイン検出と既定Renderer補正を行う。
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
