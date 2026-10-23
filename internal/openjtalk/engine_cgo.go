//go:build (linux || darwin) && cgo

package openjtalk

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>

typedef int (*OpenJTalkInitFn)(const char*);
typedef int (*OpenJTalkRunFn)(const char*, char*, int);
typedef const char* (*OpenJTalkErrorFn)(void);
typedef struct { void* handle; OpenJTalkInitFn initialize; OpenJTalkRunFn run; OpenJTalkErrorFn error; } OpenJTalkEngine;

static OpenJTalkEngine* openjtalk_open(const char* path, const char** message) {
  OpenJTalkEngine* engine = (OpenJTalkEngine*)calloc(1, sizeof(OpenJTalkEngine));
  if (!engine) { *message = "out of memory"; return NULL; }
  engine->handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
  if (!engine->handle) { *message = dlerror(); free(engine); return NULL; }
  engine->initialize = (OpenJTalkInitFn)dlsym(engine->handle, "UtauTTSOpenJTalkInit");
  engine->run = (OpenJTalkRunFn)dlsym(engine->handle, "UtauTTSOpenJTalkRun");
  engine->error = (OpenJTalkErrorFn)dlsym(engine->handle, "UtauTTSOpenJTalkError");
  if (!engine->initialize || !engine->run || !engine->error) {
    *message = "missing UtauTTSOpenJTalk exports";
    dlclose(engine->handle);
    free(engine);
    return NULL;
  }
  return engine;
}
static int openjtalk_init(OpenJTalkEngine* engine, const char* dictionary) { return engine->initialize(dictionary); }
static int openjtalk_run(OpenJTalkEngine* engine, const char* text, char* output, int capacity) { return engine->run(text, output, capacity); }
static const char* openjtalk_error(OpenJTalkEngine* engine) { return engine->error(); }
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type cgoEngine struct{ pointer *C.OpenJTalkEngine }

func openNativeEngine(path string) (nativeEngine, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var message *C.char
	pointer := C.openjtalk_open(cPath, &message)
	if pointer == nil {
		return nil, errors.New(C.GoString(message))
	}
	return &cgoEngine{pointer: pointer}, nil
}

func (engine *cgoEngine) load(dictionary string) error {
	cDictionary := C.CString(dictionary)
	defer C.free(unsafe.Pointer(cDictionary))
	if C.openjtalk_init(engine.pointer, cDictionary) == 0 {
		return engine.err()
	}
	return nil
}

func (engine *cgoEngine) run(text string) (string, error) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	capacity := len(text)*64 + 4096
	for range 2 {
		output := C.malloc(C.size_t(capacity))
		written := int(C.openjtalk_run(engine.pointer, cText, (*C.char)(output), C.int(capacity)))
		if written > 0 {
			result := C.GoStringN((*C.char)(output), C.int(written))
			C.free(output)
			return result, nil
		}
		C.free(output)
		if written == 0 {
			return "", engine.err()
		}
		capacity = -written
	}
	return "", errors.New("Open JTalk output buffer is too small")
}

func (engine *cgoEngine) err() error {
	return fmt.Errorf("Open JTalk: %s", C.GoString(C.openjtalk_error(engine.pointer)))
}
