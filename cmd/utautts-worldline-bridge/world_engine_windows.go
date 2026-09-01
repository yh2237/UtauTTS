//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type windowsWorldEngine struct {
	dll                        *syscall.DLL
	shape, analyze, synthesize *syscall.Proc
}

type windowsWorldShape struct {
	SampleCount, SampleRate int32
	FramePeriodMS           float64
	FrameCount, FFTSize     int32
}

type windowsWorldAnalysis struct {
	Samples                 uintptr
	SampleCount, SampleRate int32
	FramePeriodMS           float64
	InputF0                 uintptr
	InputF0Count            int32
	F0, Spectrum, AP        uintptr
}

type windowsWorldSynthesis struct {
	F0         uintptr
	FrameCount int32
	Spectrum   uintptr
	AP         uintptr
	FFTSize    int32
	FrameMS    float64
	SampleRate int32
	Output     uintptr
	OutputSize int32
}

func openWorldEngine(path string) (worldEngine, error) {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("load UtauTTS WORLD engine: %w", err)
	}
	find := func(name string) (*syscall.Proc, error) {
		proc, findErr := dll.FindProc(name)
		if findErr != nil {
			return nil, fmt.Errorf("WORLD engine export %s: %w", name, findErr)
		}
		return proc, nil
	}
	shape, err := find("UtauTTSWorldAnalysisShape")
	if err != nil {
		_ = dll.Release()
		return nil, err
	}
	analyze, err := find("UtauTTSWorldAnalyze")
	if err != nil {
		_ = dll.Release()
		return nil, err
	}
	synthesize, err := find("UtauTTSWorldSynthesize")
	if err != nil {
		_ = dll.Release()
		return nil, err
	}
	return &windowsWorldEngine{dll: dll, shape: shape, analyze: analyze, synthesize: synthesize}, nil
}

func (engine *windowsWorldEngine) Close() error { return engine.dll.Release() }

func worldError(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}
	return string(buffer)
}

func (engine *windowsWorldEngine) Analyze(samples []float64, sampleRate int, inputF0 []float64) (worldFeatures, error) {
	errorBuffer := make([]byte, 512)
	shape := windowsWorldShape{SampleCount: int32(len(samples)), SampleRate: int32(sampleRate), FramePeriodMS: worldFramePeriodMS}
	ok, _, _ := engine.shape.Call(uintptr(unsafe.Pointer(&shape)), windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)))
	if ok == 0 {
		return worldFeatures{}, fmt.Errorf("WORLD analysis shape: %s", worldError(errorBuffer))
	}
	frames := int(shape.FrameCount)
	if len(inputF0) >= 2 {
		frames = len(inputF0)
	}
	features := worldFeatures{Frames: frames, FFTSize: int(shape.FFTSize)}
	bins := features.FFTSize/2 + 1
	features.F0 = make([]float64, features.Frames)
	features.Spectrum = make([]float64, features.Frames*bins)
	features.Aperiodicity = make([]float64, features.Frames*bins)
	request := windowsWorldAnalysis{
		Samples: windowsSlicePointer(samples), SampleCount: int32(len(samples)), SampleRate: int32(sampleRate), FramePeriodMS: worldFramePeriodMS,
		InputF0: windowsSlicePointer(inputF0), InputF0Count: int32(len(inputF0)),
		F0: windowsSlicePointer(features.F0), Spectrum: windowsSlicePointer(features.Spectrum), AP: windowsSlicePointer(features.Aperiodicity),
	}
	ok, _, _ = engine.analyze.Call(uintptr(unsafe.Pointer(&request)), windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)))
	runtime.KeepAlive(samples)
	runtime.KeepAlive(inputF0)
	runtime.KeepAlive(features)
	if ok == 0 {
		return worldFeatures{}, fmt.Errorf("WORLD analysis: %s", worldError(errorBuffer))
	}
	return features, nil
}

func (engine *windowsWorldEngine) Synthesize(features worldFeatures, sampleRate int) ([]float64, error) {
	if err := validateWorldFeatures(features); err != nil {
		return nil, err
	}
	output := make([]float64, worldSynthesisLength(features.Frames, sampleRate))
	errorBuffer := make([]byte, 512)
	request := windowsWorldSynthesis{
		F0: windowsSlicePointer(features.F0), FrameCount: int32(features.Frames), Spectrum: windowsSlicePointer(features.Spectrum), AP: windowsSlicePointer(features.Aperiodicity),
		FFTSize: int32(features.FFTSize), FrameMS: worldFramePeriodMS, SampleRate: int32(sampleRate), Output: windowsSlicePointer(output), OutputSize: int32(len(output)),
	}
	ok, _, _ := engine.synthesize.Call(uintptr(unsafe.Pointer(&request)), windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)))
	runtime.KeepAlive(features)
	runtime.KeepAlive(output)
	if ok == 0 {
		return nil, fmt.Errorf("WORLD synthesis: %s", worldError(errorBuffer))
	}
	return output, nil
}
