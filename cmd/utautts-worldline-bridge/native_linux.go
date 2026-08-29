//go:build linux && cgo

package main

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
  int sample_fs;
  int sample_length;
  double* sample;
  int frq_length;
  char* frq;
  int tone;
  double con_vel;
  double offset;
  double required_length;
  double consonant;
  double cut_off;
  double volume;
  double modulation;
  double tempo;
  int pitch_bend_length;
  int* pitch_bend;
  int flag_g;
  int flag_O;
  int flag_P;
  int flag_Mt;
  int flag_Mb;
  int flag_Mv;
} SynthRequest;

typedef struct {
  double position_ms;
  double skip_ms;
  double length_ms;
  double fade_in_ms;
  double fade_out_ms;
} PhraseTiming;

typedef int (*ResampleFn)(const SynthRequest*, float**);
typedef void (*FreeFloatFn)(float*);
typedef void* (*PhraseNewFn)(void);
typedef void (*PhraseDeleteFn)(void*);
typedef void (*PhraseAddFn)(void*, const SynthRequest*, int, const PhraseTiming*, const char*);
typedef void (*PhraseSetCurvesFn)(void*, double*, double*, double*, double*, double*, int, void*);
typedef int (*PhraseSynthFn)(void*, float**, void*);

typedef struct {
  void* handle;
  ResampleFn resample;
  FreeFloatFn free_float;
  PhraseNewFn phrase_new;
  PhraseDeleteFn phrase_delete;
  PhraseAddFn phrase_add;
  PhraseSetCurvesFn phrase_set_curves;
  PhraseSynthFn phrase_synth;
} NativeLibrary;

static NativeLibrary* native_open(const char* path, char* error, int capacity) {
  NativeLibrary* library = (NativeLibrary*)calloc(1, sizeof(NativeLibrary));
  if (!library) return NULL;
  library->handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
  if (!library->handle) {
    snprintf(error, capacity, "%s", dlerror());
    free(library);
    return NULL;
  }
#define LOAD(field, name) do { library->field = (void*)dlsym(library->handle, name); if (!library->field) { snprintf(error, capacity, "missing %s", name); dlclose(library->handle); free(library); return NULL; } } while (0)
  LOAD(resample, "Resample");
  LOAD(free_float, "UtauTTSFreeFloat");
  LOAD(phrase_new, "PhraseSynthNew");
  LOAD(phrase_delete, "PhraseSynthDelete");
  LOAD(phrase_add, "UtauTTSPhraseSynthAddRequest");
  LOAD(phrase_set_curves, "PhraseSynthSetCurves");
  LOAD(phrase_synth, "PhraseSynthSynth");
#undef LOAD
  return library;
}

static void native_close(NativeLibrary* library) { if (library) { dlclose(library->handle); free(library); } }
static int native_resample(NativeLibrary* library, const SynthRequest* request, float** output) { return library->resample(request, output); }
static void native_free_float(NativeLibrary* library, float* output) { library->free_float(output); }
static void* native_phrase_new(NativeLibrary* library) { return library->phrase_new(); }
static void native_phrase_delete(NativeLibrary* library, void* phrase) { library->phrase_delete(phrase); }
static void native_phrase_add(NativeLibrary* library, void* phrase, const SynthRequest* request, int index, const PhraseTiming* timing, const char* cache_key) { library->phrase_add(phrase, request, index, timing, cache_key); }
static void native_phrase_set_curves(NativeLibrary* library, void* phrase, double* f0, double* gender, double* tension, double* breathiness, double* voicing, int length) { library->phrase_set_curves(phrase, f0, gender, tension, breathiness, voicing, length, NULL); }
static int native_phrase_synth(NativeLibrary* library, void* phrase, float** output) { return library->phrase_synth(phrase, output, NULL); }
// Keep uintptr_t-to-pointer conversions on the C side. Go 1.27's vet
// unsafeptr check correctly rejects converting a handle back to a pointer in Go.
static void native_phrase_delete_handle(NativeLibrary* library, uintptr_t phrase) { native_phrase_delete(library, (void*)phrase); }
static void native_phrase_add_handle(NativeLibrary* library, uintptr_t phrase, const SynthRequest* request, int index, const PhraseTiming* timing, const char* cache_key) { native_phrase_add(library, (void*)phrase, request, index, timing, cache_key); }
static void native_phrase_set_curves_handle(NativeLibrary* library, uintptr_t phrase, double* f0, double* gender, double* tension, double* breathiness, double* voicing, int length) { native_phrase_set_curves(library, (void*)phrase, f0, gender, tension, breathiness, voicing, length); }
static int native_phrase_synth_handle(NativeLibrary* library, uintptr_t phrase, float** output) { return native_phrase_synth(library, (void*)phrase, output); }
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type linuxLibrary struct{ pointer *C.NativeLibrary }

func openNativeLibrary(path string) (nativeLibrary, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	errorBuffer := make([]byte, 512)
	pointer := C.native_open(cPath, (*C.char)(unsafe.Pointer(&errorBuffer[0])), C.int(len(errorBuffer)))
	if pointer == nil {
		return nil, fmt.Errorf("load worldline: %s", cString(errorBuffer))
	}
	return &linuxLibrary{pointer: pointer}, nil
}

func cString(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}
	return string(buffer)
}

func (library *linuxLibrary) Close() error {
	C.native_close(library.pointer)
	library.pointer = nil
	return nil
}

func cBytes[T any](values []T) unsafe.Pointer {
	if len(values) == 0 {
		return nil
	}
	return C.CBytes(unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*int(unsafe.Sizeof(values[0]))))
}

func linuxNativeRequest(request nativeRequest) (C.SynthRequest, []unsafe.Pointer) {
	samples := cBytes(request.Samples)
	frq := cBytes(request.FRQ)
	bends := cBytes(request.PitchBend)
	native := C.SynthRequest{
		sample_fs: C.int(request.SampleRate), sample_length: C.int(len(request.Samples)), sample: (*C.double)(samples),
		frq_length: C.int(len(request.FRQ)), frq: (*C.char)(frq), tone: C.int(request.Tone),
		con_vel: C.double(request.ConsonantVelocity), offset: C.double(request.OffsetMS), required_length: C.double(request.RequiredLengthMS),
		consonant: C.double(request.ConsonantMS), cut_off: C.double(request.CutoffMS), volume: C.double(request.Volume),
		modulation: C.double(request.Modulation), tempo: C.double(request.Tempo), pitch_bend_length: C.int(len(request.PitchBend)), pitch_bend: (*C.int)(bends),
		flag_g: C.int(request.FlagG), flag_O: C.int(request.FlagO), flag_P: C.int(request.FlagP), flag_Mt: C.int(request.FlagMt), flag_Mb: C.int(request.FlagMb), flag_Mv: C.int(request.FlagMv),
	}
	return native, []unsafe.Pointer{samples, frq, bends}
}

func freePointers(pointers []unsafe.Pointer) {
	for _, pointer := range pointers {
		if pointer != nil {
			C.free(pointer)
		}
	}
}

func (library *linuxLibrary) copyAndFree(output *C.float, length int) []float32 {
	if output == nil || length <= 0 {
		return nil
	}
	defer C.native_free_float(library.pointer, output)
	return append([]float32(nil), unsafe.Slice((*float32)(unsafe.Pointer(output)), length)...)
}

func (library *linuxLibrary) Resample(request nativeRequest) ([]float32, error) {
	native, allocations := linuxNativeRequest(request)
	defer freePointers(allocations)
	var output *C.float
	count := int(C.native_resample(library.pointer, &native, &output))
	if count <= 0 || output == nil {
		return nil, fmt.Errorf("worldline Resample returned no audio")
	}
	return library.copyAndFree(output, count), nil
}

func (library *linuxLibrary) PhraseNew() (uintptr, error) {
	pointer := C.native_phrase_new(library.pointer)
	if pointer == nil {
		return 0, fmt.Errorf("worldline PhraseSynthNew returned null")
	}
	return uintptr(pointer), nil
}
func (library *linuxLibrary) PhraseDelete(phrase uintptr) {
	C.native_phrase_delete_handle(library.pointer, C.uintptr_t(phrase))
}
func (library *linuxLibrary) PhraseAdd(phrase uintptr, index int, request nativeRequest, timing phraseTiming) error {
	native, allocations := linuxNativeRequest(request)
	defer freePointers(allocations)
	nativeTiming := C.PhraseTiming{position_ms: C.double(timing.PositionMS), skip_ms: C.double(timing.SkipMS), length_ms: C.double(timing.LengthMS), fade_in_ms: C.double(timing.FadeInMS), fade_out_ms: C.double(timing.FadeOutMS)}
	cacheKey := C.CString(request.CacheKey)
	defer C.free(unsafe.Pointer(cacheKey))
	C.native_phrase_add_handle(library.pointer, C.uintptr_t(phrase), &native, C.int(index), &nativeTiming, cacheKey)
	return nil
}
func (library *linuxLibrary) PhraseSetCurves(phrase uintptr, f0, gender, tension, breathiness, voicing []float64) error {
	arrays := [][]float64{f0, gender, tension, breathiness, voicing}
	pointers := make([]unsafe.Pointer, len(arrays))
	for index := range arrays {
		pointers[index] = cBytes(arrays[index])
	}
	defer freePointers(pointers)
	C.native_phrase_set_curves_handle(library.pointer, C.uintptr_t(phrase), (*C.double)(pointers[0]), (*C.double)(pointers[1]), (*C.double)(pointers[2]), (*C.double)(pointers[3]), (*C.double)(pointers[4]), C.int(len(f0)))
	return nil
}
func (library *linuxLibrary) PhraseSynth(phrase uintptr) ([]float32, error) {
	var output *C.float
	count := int(C.native_phrase_synth_handle(library.pointer, C.uintptr_t(phrase), &output))
	if count <= 0 || output == nil {
		return nil, fmt.Errorf("worldline PhraseSynthSynth returned no audio")
	}
	return library.copyAndFree(output, count), nil
}
