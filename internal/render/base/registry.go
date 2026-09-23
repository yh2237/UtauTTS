package base

import (
	"utautts/internal/audio"
	"utautts/internal/plan"
)

// RenderFuncはrenderer backendの実装シグネチャ。
type RenderFunc func(*plan.Plan, Config) (*audio.PCM, error)

var implementations = map[string]RenderFunc{}

// RegisterRendererはbackend実装を登録する。renderer固有パッケージのinitから呼ぶ。
func RegisterRenderer(id string, fn RenderFunc) {
	implementations[id] = fn
}

// RendererImplementationは登録済みbackendを返す。
func RendererImplementation(id string) (RenderFunc, bool) {
	fn, ok := implementations[id]
	return fn, ok
}

// KnownRendererはbackendが登録済みかを返す。
func KnownRenderer(id string) bool {
	_, ok := implementations[id]
	return ok
}
