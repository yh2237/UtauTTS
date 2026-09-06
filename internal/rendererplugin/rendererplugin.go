// rendererpluginパッケージは診断・評価ツール向けにレンダラープラグインを解決する。
package rendererplugin

import (
	"fmt"

	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/tts"
)

func Discover(extraDirectories []string) ([]plugin.Renderer, error) {
	catalog, err := plugin.DiscoverWithDefaults(extraDirectories, nil, render.IsKnownRenderer)
	if err != nil {
		return nil, err
	}
	return catalog.Renderers, nil
}

func Resolve(renderers []plugin.Renderer, id string) (plugin.Renderer, error) {
	if id == "" && len(renderers) > 0 {
		return renderers[0], nil
	}
	for _, renderer := range renderers {
		if renderer.ID == id {
			return renderer, nil
		}
	}
	return plugin.Renderer{}, fmt.Errorf("renderer plugin %q is not installed", id)
}

func Apply(renderer plugin.Renderer, config *tts.Config) {
	tts.ApplyResolvedRenderer(config, renderer)
}
