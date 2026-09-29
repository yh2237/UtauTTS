//go:build js

package worldrender

import (
	"errors"
	"syscall/js"
	"unsafe"
)

// wasmWorldEngineはブラウザ/Node上のWORLD wasmをJSブリッジ経由で呼ぶ。
type wasmWorldEngine struct{}

func openWorldEngine(path string) (worldEngine, error) {
	_ = path
	if js.Global().Get("utauttsWorld").Type() != js.TypeObject {
		return nil, errors.New("utauttsWorld is not initialized; load the WORLD wasm module first")
	}
	return &wasmWorldEngine{}, nil
}

func (engine *wasmWorldEngine) Close() error { return nil }

func (engine *wasmWorldEngine) Analyze(samples []float64, sampleRate int, inputF0 []float64) (worldFeatures, error) {
	api := js.Global().Get("utauttsWorld")
	result, ok := callWorld(api, "analyze", float64SliceToJS(samples), sampleRate, float64SliceToJS(inputF0))
	if !ok {
		return worldFeatures{}, errors.New("WORLD analyze call failed")
	}
	if !result.Get("ok").Bool() {
		return worldFeatures{}, errors.New(worldErrorMessage(result))
	}
	frames := result.Get("frames").Int()
	fftSize := result.Get("fftSize").Int()
	bins := fftSize/2 + 1
	return worldFeatures{
		Frames:       frames,
		FFTSize:      fftSize,
		F0:           float64SliceFromJS(result.Get("f0"), frames),
		Spectrum:     float64SliceFromJS(result.Get("spectrum"), frames*bins),
		Aperiodicity: float64SliceFromJS(result.Get("aperiodicity"), frames*bins),
	}, nil
}

func (engine *wasmWorldEngine) Synthesize(features worldFeatures, sampleRate int) ([]float64, error) {
	api := js.Global().Get("utauttsWorld")
	result, ok := callWorld(api, "synthesize",
		float64SliceToJS(features.F0), float64SliceToJS(features.Spectrum),
		float64SliceToJS(features.Aperiodicity), features.Frames, features.FFTSize, sampleRate)
	if !ok {
		return nil, errors.New("WORLD synthesize call failed")
	}
	if !result.Get("ok").Bool() {
		return nil, errors.New(worldErrorMessage(result))
	}
	samples := result.Get("samples")
	return float64SliceFromJS(samples, samples.Get("length").Int()), nil
}

func callWorld(api js.Value, method string, args ...any) (result js.Value, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	jsArgs := make([]any, len(args))
	for index, arg := range args {
		jsArgs[index] = js.ValueOf(arg)
	}
	return api.Call(method, jsArgs...), true
}

func worldErrorMessage(result js.Value) string {
	message := result.Get("error").String()
	if message == "" {
		message = "WORLD failed"
	}
	return message
}

func float64SliceToJS(values []float64) js.Value {
	if len(values) == 0 {
		return js.Null()
	}
	array := js.Global().Get("Float64Array").New(len(values))
	bytes := js.Global().Get("Uint8Array").New(array.Get("buffer"), array.Get("byteOffset").Int(), len(values)*8)
	js.CopyBytesToJS(bytes, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*8))
	return array
}

func float64SliceFromJS(value js.Value, length int) []float64 {
	result := make([]float64, length)
	if length == 0 || value.Type() != js.TypeObject {
		return result
	}
	bytes := js.Global().Get("Uint8Array").New(value.Get("buffer"), value.Get("byteOffset").Int(), length*8)
	js.CopyBytesToGo(unsafe.Slice((*byte)(unsafe.Pointer(&result[0])), length*8), bytes)
	return result
}
