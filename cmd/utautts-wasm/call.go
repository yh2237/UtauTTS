//go:build js && wasm

package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sync"
	"syscall/js"

	"github.com/yh2237/UtauTTS/internal/native"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

// inferenceDirは、JSへ渡す推論の入力（16 kHzの音声）を置く仮想FSのフォルダ。
const inferenceDir = "/tmp/utautts-inference"

// needJSONは、ONNX Runtime Webで求める推論を返す。JSは結果をprovideInferenceで渡して同じ呼び出しをやり直す。
// featuresの音声はfloat32のファイルにして、samples_pathで渡す。
func needJSON(need *refspeech.NeedInference) string {
	type job struct {
		refspeech.InferenceJob
		SamplesPath string `json:"samples_path,omitempty"`
	}
	jobs := make([]job, len(need.Jobs))
	for i, item := range need.Jobs {
		jobs[i].InferenceJob = item
		if item.Samples == nil {
			continue
		}
		if err := os.MkdirAll(inferenceDir, 0o755); err != nil {
			return errorJSON(err)
		}
		path := inferenceDir + "/" + item.Key + ".f32"
		data := make([]byte, len(item.Samples)*4)
		for k, v := range item.Samples {
			binary.LittleEndian.PutUint32(data[k*4:], math.Float32bits(v))
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return errorJSON(err)
		}
		jobs[i].SamplesPath = path
	}
	data, err := json.Marshal(struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Need  []job  `json:"need"`
	}{Error: need.Error(), Need: jobs})
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

var (
	engineOnce sync.Once
	engine     *native.Engine
	engineErr  error
)

func sharedEngine() (*native.Engine, error) {
	engineOnce.Do(func() {
		engine, engineErr = native.New(native.Config{
			VoiceDir:         "/voice",
			ModelDirectories: []string{"/models"},
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
	var result []byte
	if method == "prefetchSources" {
		paths, prepareErr := instance.SynthesisSourcePaths([]byte(request))
		err = prepareErr
		if err == nil {
			result, err = json.Marshal(paths)
		}
	} else {
		result, err = instance.Call(method, []byte(request))
	}
	var need *refspeech.NeedInference
	if errors.As(err, &need) {
		return needJSON(need)
	}
	if err != nil {
		return errorJSON(err)
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
