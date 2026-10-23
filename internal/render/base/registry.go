package base

import (
	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
)

type RenderFunc func(*plan.Plan, Config) (*audio.PCM, error)

var implementations = map[string]RenderFunc{}

// DefaultBackendは、Backendが空のときに使う描画。テストではWORLDのいらない描画に差し替える。
var DefaultBackend = WorldPhraseBackend

// 描画を実装するパッケージのinitから登録する。
func RegisterRenderer(id string, fn RenderFunc) {
	implementations[id] = fn
}

func RendererImplementation(id string) (RenderFunc, bool) {
	fn, ok := implementations[id]
	return fn, ok
}
