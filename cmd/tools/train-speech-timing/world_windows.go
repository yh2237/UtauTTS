//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type worldEngine struct {
	dll                *syscall.DLL
	shape, analyzeProc *syscall.Proc
}
type worldShape struct {
	SampleCount, SampleRate int32
	FramePeriodMS           float64
	FrameCount, FFTSize     int32
}
type worldRequest struct {
	Samples                 uintptr
	SampleCount, SampleRate int32
	FramePeriodMS           float64
	InputF0                 uintptr
	InputF0Count            int32
	F0, Spectrum, AP        uintptr
}

func ptr[T any](s []T) uintptr {
	if len(s) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&s[0]))
}
func openWorld(path string) (*worldEngine, error) {
	dll, e := syscall.LoadDLL(path)
	if e != nil {
		return nil, e
	}
	shape, e := dll.FindProc("UtauTTSWorldAnalysisShape")
	if e != nil {
		dll.Release()
		return nil, e
	}
	analyze, e := dll.FindProc("UtauTTSWorldAnalyze")
	if e != nil {
		dll.Release()
		return nil, e
	}
	return &worldEngine{dll, shape, analyze}, nil
}
func (w *worldEngine) close() { _ = w.dll.Release() }
func (w *worldEngine) analyze(x []float64, rate int) ([]float64, []float64, int, error) {
	errBuf := make([]byte, 512)
	shape := worldShape{SampleCount: int32(len(x)), SampleRate: int32(rate), FramePeriodMS: 10}
	ok, _, _ := w.shape.Call(uintptr(unsafe.Pointer(&shape)), ptr(errBuf), 512)
	if ok == 0 {
		return nil, nil, 0, fmt.Errorf("WORLD shape: %s", worldError(errBuf))
	}
	frames, fft := int(shape.FrameCount), int(shape.FFTSize)
	f0 := make([]float64, frames)
	sp := make([]float64, frames*(fft/2+1))
	ap := make([]float64, len(sp))
	req := worldRequest{Samples: ptr(x), SampleCount: int32(len(x)), SampleRate: int32(rate), FramePeriodMS: 10, F0: ptr(f0), Spectrum: ptr(sp), AP: ptr(ap)}
	ok, _, _ = w.analyzeProc.Call(uintptr(unsafe.Pointer(&req)), ptr(errBuf), 512)
	runtime.KeepAlive(x)
	runtime.KeepAlive(f0)
	runtime.KeepAlive(sp)
	runtime.KeepAlive(ap)
	if ok == 0 {
		return nil, nil, 0, fmt.Errorf("WORLD analysis: %s", worldError(errBuf))
	}
	return f0, sp, fft, nil
}
func worldError(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
