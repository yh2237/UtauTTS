//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
	"utautts/internal/audio"
)

type worldShape struct {
	Count, Rate int32
	Step        float64
	Frames, FFT int32
}
type worldRequest struct {
	Samples          uintptr
	Count, Rate      int32
	Step             float64
	InputF0          uintptr
	InputCount       int32
	F0, Spectrum, AP uintptr
}

func worldPtr[T any](x []T) uintptr {
	if len(x) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&x[0]))
}
func worldF0(r record, step float64, path string) ([]float64, error) {
	if path == "" {
		path = "runtime/utautts-world-engine.dll"
	}
	dll, e := syscall.LoadDLL(path)
	if e != nil {
		return nil, e
	}
	defer dll.Release()
	shapeProc, e := dll.FindProc("UtauTTSWorldAnalysisShape")
	if e != nil {
		return nil, e
	}
	analyze, e := dll.FindProc("UtauTTSWorldAnalyze")
	if e != nil {
		return nil, e
	}
	pcm, e := audio.ReadWav(strings.ReplaceAll(r.AudioPath, "\\", "/"))
	if e != nil {
		return nil, e
	}
	samples := make([]float64, len(pcm.Data)/pcm.Channels)
	for i := range samples {
		for c := 0; c < pcm.Channels; c++ {
			samples[i] += float64(pcm.Data[i*pcm.Channels+c]) / 32768
		}
		samples[i] /= float64(pcm.Channels)
	}
	buf := make([]byte, 1024)
	shape := worldShape{Count: int32(len(samples)), Rate: int32(pcm.SampleRate), Step: step}
	ok, _, _ := shapeProc.Call(uintptr(unsafe.Pointer(&shape)), worldPtr(buf), uintptr(len(buf)))
	if ok == 0 {
		return nil, fmt.Errorf("WORLD shape: %s", string(buf))
	}
	f0 := make([]float64, shape.Frames)
	bins := int(shape.FFT)/2 + 1
	sp := make([]float64, int(shape.Frames)*bins)
	ap := make([]float64, len(sp))
	req := worldRequest{Samples: worldPtr(samples), Count: int32(len(samples)), Rate: int32(pcm.SampleRate), Step: step, F0: worldPtr(f0), Spectrum: worldPtr(sp), AP: worldPtr(ap)}
	ok, _, _ = analyze.Call(uintptr(unsafe.Pointer(&req)), worldPtr(buf), uintptr(len(buf)))
	runtime.KeepAlive(samples)
	runtime.KeepAlive(f0)
	runtime.KeepAlive(sp)
	runtime.KeepAlive(ap)
	if ok == 0 {
		return nil, fmt.Errorf("WORLD Analyze: %s", string(buf))
	}
	return f0, nil
}
