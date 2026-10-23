package synth

import (
	"context"
	"fmt"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/render"
)

type RuntimeConfig struct {
	Renderer                              string
	OpenJTalkPath                         string
	OpenJTalkDictionary                   string
	RendererDirectories, ModelDirectories []string
}

type Runtime struct {
	Catalog  *plugin.Catalog
	Renderer string
	Service  *Service
	scope    *lifecycle.Scope
}

func NewRuntime(config RuntimeConfig, voicebanks VoicebankResolver) (*Runtime, error) {
	catalog, err := plugin.DiscoverWithDefaults(config.RendererDirectories, config.ModelDirectories, render.IsKnownRenderer)
	if err != nil {
		return nil, fmt.Errorf("discover renderers: %w", err)
	}
	if renderer, ok := catalog.Renderer(config.Renderer); ok {
		config.Renderer = renderer.ID
	}
	scope := lifecycle.New()
	service := NewService(catalog, config.Renderer,
		config.OpenJTalkPath, config.OpenJTalkDictionary, voicebanks)
	service.scope = scope
	return &Runtime{
		Catalog: catalog, Renderer: config.Renderer,
		Service: service, scope: scope,
	}, nil
}

func (r *Runtime) Begin(ctx context.Context) (context.Context, func(), error) {
	return r.scope.Begin(ctx)
}

func (r *Runtime) Reload(reload func() error) error {
	return r.scope.Reset(reload)
}

func (r *Runtime) Close() error {
	return r.scope.Close()
}
