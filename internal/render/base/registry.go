package base

import (
	"errors"
	"sync"

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

var (
	closerMu sync.Mutex
	closers  []func() error
)

// RegisterCloserは常駐する外部リソースの解放関数を登録する。renderer固有パッケージのinitから呼ぶ。
func RegisterCloser(fn func() error) {
	if fn == nil {
		return
	}
	closerMu.Lock()
	closers = append(closers, fn)
	closerMu.Unlock()
}

// CloseRegisteredは登録済みの解放関数を登録と逆順に呼び、返されたエラーをまとめる。
func CloseRegistered() error {
	closerMu.Lock()
	registered := append([]func() error(nil), closers...)
	closerMu.Unlock()
	var err error
	for index := len(registered) - 1; index >= 0; index-- {
		err = errors.Join(err, registered[index]())
	}
	return err
}
