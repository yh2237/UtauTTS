//go:build js

package openjtalk

import (
	"context"
	"errors"
	"syscall/js"
)

// JS側にglobalThis.utauttsOpenJTalk = {ready, run(text) -> {ok, tsv, error}}を用意する。
func runFrontend(ctx context.Context, text string, cfg Config) (*Analysis, error) {
	_ = ctx
	_ = cfg
	api := js.Global().Get("utauttsOpenJTalk")
	if api.Type() != js.TypeObject {
		return nil, errors.New("utauttsOpenJTalk is not initialized; load the Open JTalk wasm module first")
	}
	if ready := api.Get("ready"); ready.Type() == js.TypeBoolean && !ready.Bool() {
		return nil, errors.New("utauttsOpenJTalk is not ready")
	}
	result, ok := callOpenJTalk(api, text)
	if !ok {
		return nil, errors.New("Open JTalk wasm call failed")
	}
	if !result.Get("ok").Bool() {
		message := result.Get("error").String()
		if message == "" {
			message = "Open JTalk wasm returned an error"
		}
		return nil, errors.New(message)
	}
	nodes, err := parseNJD(result.Get("tsv").String())
	if err != nil {
		return nil, err
	}
	return buildAnalysis(nodes), nil
}

func callOpenJTalk(api js.Value, text string) (result js.Value, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return api.Call("run", text), true
}
