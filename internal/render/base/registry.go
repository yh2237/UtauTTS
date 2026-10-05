package base

import (
	"errors"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/plan"
)

type RenderFunc func(*plan.Plan, Config) (*audio.PCM, error)

var implementations = map[string]RenderFunc{}

// レンダラー固有パッケージのinitから登録する。
func RegisterRenderer(id string, fn RenderFunc) {
	implementations[id] = fn
}

func RendererImplementation(id string) (RenderFunc, bool) {
	fn, ok := implementations[id]
	return fn, ok
}

var (
	closerMu sync.Mutex
	closers  []func() error
)

func RegisterCloser(fn func() error) {
	if fn == nil {
		return
	}
	closerMu.Lock()
	closers = append(closers, fn)
	closerMu.Unlock()
}

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
