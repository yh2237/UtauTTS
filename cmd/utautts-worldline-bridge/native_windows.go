//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type windowsRequest struct {
	SampleFS, SampleLength int32
	Sample                 uintptr
	FRQLength              int32
	FRQ                    uintptr
	Tone                   int32
	ConVel                 float64
	Offset                 float64
	RequiredLength         float64
	Consonant              float64
	Cutoff                 float64
	Volume                 float64
	Modulation             float64
	Tempo                  float64
	PitchBendLength        int32
	PitchBend              uintptr
	FlagG, FlagO, FlagP    int32
	FlagMt, FlagMb, FlagMv int32
}

type windowsTiming struct {
	PositionMS, SkipMS, LengthMS, FadeInMS, FadeOutMS float64
}

type windowsLibrary struct {
	dll                        *syscall.DLL
	resample, freeFloat        *syscall.Proc
	copyFloat                  *syscall.Proc
	phraseNew, phraseDelete    *syscall.Proc
	phraseAdd, phraseSetCurves *syscall.Proc
	phraseSynth                *syscall.Proc
}

func openNativeLibrary(path string) (nativeLibrary, error) {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("load worldline: %w", err)
	}
	load := func(name string) (*syscall.Proc, error) {
		proc, err := dll.FindProc(name)
		if err != nil {
			return nil, fmt.Errorf("worldline export %s: %w", name, err)
		}
		return proc, nil
	}
	names := []string{"Resample", "UtauTTSFreeFloat", "UtauTTSCopyFloat", "PhraseSynthNew", "PhraseSynthDelete", "UtauTTSPhraseSynthAddRequest", "PhraseSynthSetCurves", "PhraseSynthSynth"}
	procs := make([]*syscall.Proc, len(names))
	for index, name := range names {
		procs[index], err = load(name)
		if err != nil {
			_ = dll.Release()
			return nil, err
		}
	}
	return &windowsLibrary{dll: dll, resample: procs[0], freeFloat: procs[1], copyFloat: procs[2], phraseNew: procs[3], phraseDelete: procs[4], phraseAdd: procs[5], phraseSetCurves: procs[6], phraseSynth: procs[7]}, nil
}

func (library *windowsLibrary) Close() error { return library.dll.Release() }

func windowsSlicePointer[T any](values []T) uintptr {
	if len(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&values[0]))
}

func windowsNativeRequest(request nativeRequest) windowsRequest {
	return windowsRequest{
		SampleFS: int32(request.SampleRate), SampleLength: int32(len(request.Samples)), Sample: windowsSlicePointer(request.Samples),
		FRQLength: int32(len(request.FRQ)), FRQ: windowsSlicePointer(request.FRQ), Tone: int32(request.Tone),
		ConVel: request.ConsonantVelocity, Offset: request.OffsetMS, RequiredLength: request.RequiredLengthMS,
		Consonant: request.ConsonantMS, Cutoff: request.CutoffMS, Volume: request.Volume,
		Modulation: request.Modulation, Tempo: request.Tempo,
		PitchBendLength: int32(len(request.PitchBend)), PitchBend: windowsSlicePointer(request.PitchBend),
		FlagG: int32(request.FlagG), FlagO: int32(request.FlagO), FlagP: int32(request.FlagP),
		FlagMt: int32(request.FlagMt), FlagMb: int32(request.FlagMb), FlagMv: int32(request.FlagMv),
	}
}

func (library *windowsLibrary) copyAndFree(pointer uintptr, length int) []float32 {
	if pointer == 0 || length <= 0 {
		return nil
	}
	defer library.freeFloat.Call(pointer)
	result := make([]float32, length)
	library.copyFloat.Call(pointer, windowsSlicePointer(result), uintptr(length))
	return result
}

func (library *windowsLibrary) Resample(request nativeRequest) ([]float32, error) {
	native := windowsNativeRequest(request)
	var output uintptr
	count, _, _ := library.resample.Call(uintptr(unsafe.Pointer(&native)), uintptr(unsafe.Pointer(&output)))
	runtime.KeepAlive(request)
	if int32(count) <= 0 || output == 0 {
		return nil, fmt.Errorf("worldline Resample returned no audio")
	}
	return library.copyAndFree(output, int(int32(count))), nil
}

func (library *windowsLibrary) PhraseNew() (uintptr, error) {
	pointer, _, _ := library.phraseNew.Call()
	if pointer == 0 {
		return 0, fmt.Errorf("worldline PhraseSynthNew returned null")
	}
	return pointer, nil
}

func (library *windowsLibrary) PhraseDelete(pointer uintptr) { library.phraseDelete.Call(pointer) }

func (library *windowsLibrary) PhraseAdd(phrase uintptr, index int, request nativeRequest, timing phraseTiming) error {
	native := windowsNativeRequest(request)
	nativeTiming := windowsTiming{timing.PositionMS, timing.SkipMS, timing.LengthMS, timing.FadeInMS, timing.FadeOutMS}
	cacheKey := append([]byte(request.CacheKey), 0)
	library.phraseAdd.Call(phrase, uintptr(unsafe.Pointer(&native)), uintptr(index), uintptr(unsafe.Pointer(&nativeTiming)), windowsSlicePointer(cacheKey))
	runtime.KeepAlive(request)
	runtime.KeepAlive(cacheKey)
	return nil
}

func (library *windowsLibrary) PhraseSetCurves(phrase uintptr, f0, gender, tension, breathiness, voicing []float64) error {
	library.phraseSetCurves.Call(phrase, windowsSlicePointer(f0), windowsSlicePointer(gender), windowsSlicePointer(tension), windowsSlicePointer(breathiness), windowsSlicePointer(voicing), uintptr(len(f0)), 0)
	runtime.KeepAlive(f0)
	runtime.KeepAlive(gender)
	runtime.KeepAlive(tension)
	runtime.KeepAlive(breathiness)
	runtime.KeepAlive(voicing)
	return nil
}

func (library *windowsLibrary) PhraseSynth(phrase uintptr) ([]float32, error) {
	var output uintptr
	count, _, _ := library.phraseSynth.Call(phrase, uintptr(unsafe.Pointer(&output)), 0)
	if int32(count) <= 0 || output == 0 {
		return nil, fmt.Errorf("worldline PhraseSynthSynth returned no audio")
	}
	return library.copyAndFree(output, int(int32(count))), nil
}
