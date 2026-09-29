//go:build js && wasm

package main

import (
	"encoding/json"
	"sync"
	"syscall/js"

	"utautts/internal/native"
)

// nativeEngine はデスクトップと共通のエンジン本体。wasm では call 経由で使う。
var (
	engineOnce sync.Once
	engine     *native.Engine
	engineErr  error
)

func sharedEngine() (*native.Engine, error) {
	engineOnce.Do(func() {
		engine, engineErr = native.New(native.Config{
			VoiceDir:            "/voice",
			Renderer:            "utautts-world-phrase",
			ModelDirectories:    []string{"/models"},
			RendererDirectories: []string{"/renderer"},
		})
	})
	return engine, engineErr
}

// engineCall(method, requestJSON) は native.Engine.Call の結果を {ok, result} JSON で返す。
func engineCall(this js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return errorJSON("call requires a method name")
	}
	method := args[0].String()
	request := "{}"
	if len(args) >= 2 && args[1].Type() == js.TypeString && args[1].String() != "" {
		request = args[1].String()
	}
	instance, err := sharedEngine()
	if err != nil {
		return errorJSON("engine init: " + err.Error())
	}
	result, err := instance.Call(method, []byte(request))
	if err != nil {
		data, _ := json.Marshal(errorResponse{Error: err.Error()})
		return string(data)
	}
	response, err := json.Marshal(struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}{OK: true, Result: json.RawMessage(result)})
	if err != nil {
		return errorJSON("encode result: " + err.Error())
	}
	return string(response)
}
