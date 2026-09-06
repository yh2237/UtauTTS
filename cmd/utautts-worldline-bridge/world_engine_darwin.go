//go:build darwin && cgo

package main

/*
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

typedef struct { int sample_count; int sample_rate; double frame_period_ms; int frame_count; int fft_size; } WorldShape;
typedef struct { const double* samples; int sample_count; int sample_rate; double frame_period_ms; const double* input_f0; int input_f0_count; double* f0; double* spectrum; double* ap; } WorldAnalysis;
typedef struct { const double* f0; int frame_count; const double* spectrum; const double* ap; int fft_size; double frame_period_ms; int sample_rate; double* output; int output_count; } WorldSynthesis;
typedef int (*WorldShapeFn)(WorldShape*, char*, int);
typedef int (*WorldAnalyzeFn)(const WorldAnalysis*, char*, int);
typedef int (*WorldSynthesizeFn)(const WorldSynthesis*, char*, int);
typedef struct { void* handle; WorldShapeFn shape; WorldAnalyzeFn analyze; WorldSynthesizeFn synthesize; } WorldEngine;

static WorldEngine* world_open(const char* path, char* error, int capacity) {
  WorldEngine* engine = (WorldEngine*)calloc(1, sizeof(WorldEngine));
  if (!engine) return NULL;
  engine->handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
  if (!engine->handle) { snprintf(error, capacity, "%s", dlerror()); free(engine); return NULL; }
#define LOAD(field, name) do { engine->field = (void*)dlsym(engine->handle, name); if (!engine->field) { snprintf(error, capacity, "missing %s", name); dlclose(engine->handle); free(engine); return NULL; } } while (0)
  LOAD(shape, "UtauTTSWorldAnalysisShape");
  LOAD(analyze, "UtauTTSWorldAnalyze");
  LOAD(synthesize, "UtauTTSWorldSynthesize");
#undef LOAD
  return engine;
}
static void world_close(WorldEngine* engine) { if (engine) { dlclose(engine->handle); free(engine); } }
static int world_shape(WorldEngine* engine, WorldShape* request, char* error, int capacity) { return engine->shape(request, error, capacity); }
static int world_analyze(WorldEngine* engine, const WorldAnalysis* request, char* error, int capacity) { return engine->analyze(request, error, capacity); }
static int world_synthesize(WorldEngine* engine, const WorldSynthesis* request, char* error, int capacity) { return engine->synthesize(request, error, capacity); }
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type darwinWorldEngine struct{ pointer *C.WorldEngine }

func openWorldEngine(path string) (worldEngine, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	errorBuffer := make([]byte, 512)
	pointer := C.world_open(cPath, (*C.char)(unsafe.Pointer(&errorBuffer[0])), C.int(len(errorBuffer)))
	if pointer == nil {
		return nil, fmt.Errorf("load UtauTTS WORLD engine: %s", cString(errorBuffer))
	}
	return &darwinWorldEngine{pointer: pointer}, nil
}

func (engine *darwinWorldEngine) Close() error {
	C.world_close(engine.pointer)
	engine.pointer = nil
	return nil
}

func (engine *darwinWorldEngine) Analyze(samples []float64, sampleRate int, inputF0 []float64) (worldFeatures, error) {
	errorBuffer := make([]byte, 512)
	shape := C.WorldShape{sample_count: C.int(len(samples)), sample_rate: C.int(sampleRate), frame_period_ms: C.double(worldFramePeriodMS)}
	if C.world_shape(engine.pointer, &shape, (*C.char)(unsafe.Pointer(&errorBuffer[0])), C.int(len(errorBuffer))) == 0 {
		return worldFeatures{}, fmt.Errorf("WORLD analysis shape: %s", cString(errorBuffer))
	}
	frames := int(shape.frame_count)
	if len(inputF0) >= 2 {
		frames = len(inputF0)
	}
	features := worldFeatures{Frames: frames, FFTSize: int(shape.fft_size)}
	bins := features.FFTSize/2 + 1
	features.F0 = make([]float64, features.Frames)
	features.Spectrum = make([]float64, features.Frames*bins)
	features.Aperiodicity = make([]float64, features.Frames*bins)
	cSamples, cInputF0, cF0 := cBytes(samples), cBytes(inputF0), cBytes(features.F0)
	cSpectrum, cAP := cBytes(features.Spectrum), cBytes(features.Aperiodicity)
	defer freePointers([]unsafe.Pointer{cSamples, cInputF0, cF0, cSpectrum, cAP})
	request := C.WorldAnalysis{
		samples: (*C.double)(cSamples), sample_count: C.int(len(samples)), sample_rate: C.int(sampleRate), frame_period_ms: C.double(worldFramePeriodMS),
		input_f0: (*C.double)(cInputF0), input_f0_count: C.int(len(inputF0)),
		f0: (*C.double)(cF0), spectrum: (*C.double)(cSpectrum), ap: (*C.double)(cAP),
	}
	if C.world_analyze(engine.pointer, &request, (*C.char)(unsafe.Pointer(&errorBuffer[0])), C.int(len(errorBuffer))) == 0 {
		return worldFeatures{}, fmt.Errorf("WORLD analysis: %s", cString(errorBuffer))
	}
	copy(features.F0, unsafe.Slice((*float64)(cF0), len(features.F0)))
	copy(features.Spectrum, unsafe.Slice((*float64)(cSpectrum), len(features.Spectrum)))
	copy(features.Aperiodicity, unsafe.Slice((*float64)(cAP), len(features.Aperiodicity)))
	return features, nil
}

func (engine *darwinWorldEngine) Synthesize(features worldFeatures, sampleRate int) ([]float64, error) {
	if err := validateWorldFeatures(features); err != nil {
		return nil, err
	}
	output := make([]float64, worldSynthesisLength(features.Frames, sampleRate))
	errorBuffer := make([]byte, 512)
	cF0, cSpectrum := cBytes(features.F0), cBytes(features.Spectrum)
	cAP, cOutput := cBytes(features.Aperiodicity), cBytes(output)
	defer freePointers([]unsafe.Pointer{cF0, cSpectrum, cAP, cOutput})
	request := C.WorldSynthesis{
		f0: (*C.double)(cF0), frame_count: C.int(features.Frames), spectrum: (*C.double)(cSpectrum), ap: (*C.double)(cAP),
		fft_size: C.int(features.FFTSize), frame_period_ms: C.double(worldFramePeriodMS), sample_rate: C.int(sampleRate), output: (*C.double)(cOutput), output_count: C.int(len(output)),
	}
	if C.world_synthesize(engine.pointer, &request, (*C.char)(unsafe.Pointer(&errorBuffer[0])), C.int(len(errorBuffer))) == 0 {
		return nil, fmt.Errorf("WORLD synthesis: %s", cString(errorBuffer))
	}
	copy(output, unsafe.Slice((*float64)(cOutput), len(output)))
	return output, nil
}
