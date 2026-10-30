package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/yh2237/UtauTTS/internal/native"
)

type response struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Create失敗時はハンドルがないため、直近のエラーを別途保持する。
// 成功時は、別スレッドが読む失敗を消さないよう保持したままにする。
var lastCreateError struct {
	sync.Mutex
	message string
}

func setLastCreateError(message string) {
	lastCreateError.Lock()
	lastCreateError.message = message
	lastCreateError.Unlock()
}

//export UtauTTSCreate
func UtauTTSCreate(configJSON *C.char) (handle C.uintptr_t) {
	defer func() {
		if recovered := recover(); recovered != nil {
			setLastCreateError(fmt.Sprintf("エンジンを初期化できませんでした: %v", recovered))
			handle = 0
		}
	}()
	var data []byte
	if configJSON != nil {
		data = []byte(C.GoString(configJSON))
	}
	engine, err := native.NewJSON(data)
	if err != nil {
		setLastCreateError(err.Error())
		return 0
	}
	return C.uintptr_t(cgo.NewHandle(engine))
}

//export UtauTTSLastError
func UtauTTSLastError() *C.char {
	lastCreateError.Lock()
	defer lastCreateError.Unlock()
	return C.CString(lastCreateError.message)
}

//export UtauTTSCall
func UtauTTSCall(rawHandle C.uintptr_t, method, requestJSON *C.char) *C.char {
	answer := response{}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				answer.Error = "invalid native engine handle"
			}
		}()
		engine, ok := cgo.Handle(rawHandle).Value().(*native.Engine)
		if !ok {
			answer.Error = "invalid native engine"
			return
		}
		var request []byte
		if requestJSON != nil {
			request = []byte(C.GoString(requestJSON))
		}
		result, err := engine.Call(C.GoString(method), request)
		if err != nil {
			answer.Error = err.Error()
			return
		}
		answer.OK = true
		answer.Result = result
	}()
	data, _ := json.Marshal(answer)
	return C.CString(string(data))
}

//export UtauTTSDestroy
func UtauTTSDestroy(rawHandle C.uintptr_t) {
	defer func() { _ = recover() }()
	cgo.Handle(rawHandle).Delete()
}

//export UtauTTSFree
func UtauTTSFree(value *C.char) { C.free(unsafe.Pointer(value)) }

func main() {}
