package synth

import (
	"context"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/plugin"
)

type RuntimeConfig struct {
	OpenJTalkPath       string
	OpenJTalkDictionary string
	ModelDirectories    []string
}

type Runtime struct {
	Catalog *plugin.Catalog
	Service *Service
	scope   *lifecycle.Scope
}

func NewRuntime(config RuntimeConfig, voicebanks VoicebankResolver) (*Runtime, error) {
	catalog := plugin.DiscoverWithDefaults(config.ModelDirectories)
	scope := lifecycle.New()
	service := NewService(catalog, config.OpenJTalkPath, config.OpenJTalkDictionary, voicebanks)
	service.scope = scope
	return &Runtime{Catalog: catalog, Service: service, scope: scope}, nil
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
