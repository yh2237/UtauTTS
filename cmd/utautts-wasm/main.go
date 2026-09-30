//go:build js && wasm

// Goエンジンをwasmへ公開する。callはデスクトップと共通のnative.Engineを使う。
package main

import (
	"encoding/json"
	"os"
	"syscall/js"

	"utautts/internal/render/worldline"
	"utautts/internal/tts"
)

// versionはJS側との互換判定に使う公開APIのバージョン。
const version = 1

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	api := map[string]any{
		"version": version,
		"call":    js.FuncOf(engineCall),
	}
	js.Global().Set("utauttsWasm", js.ValueOf(api))
	if os.Getenv("UTAUTTS_TTS_PROFILE") != "" {
		log := func(message string) {
			js.Global().Get("console").Call("log", "utautts "+message)
		}
		tts.Trace = log
		worldline.Trace = log
	}
	// wasmのランタイムを生かしたままJSからの呼び出しを待つ。
	keepAlive := make(chan struct{})
	<-keepAlive
}

func errorJSON(value any) string {
	data, err := json.Marshal(errorResponse{Error: toString(value)})
	if err != nil {
		return `{"error":"unknown"}`
	}
	return string(data)
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case error:
		return typed.Error()
	default:
		return "unknown error"
	}
}
