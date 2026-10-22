//go:build windows

package worldrender

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

type windowsWorldProcs struct {
	once                             sync.Once
	f0, open, add, synthesize, close *syscall.Proc
}

var windowsWorldExtras = map[*syscall.DLL]*windowsWorldProcs{}
var windowsWorldExtrasMu sync.Mutex

func (engine *windowsWorldEngine) extras() *windowsWorldProcs {
	windowsWorldExtrasMu.Lock()
	procs := windowsWorldExtras[engine.dll]
	if procs == nil {
		procs = &windowsWorldProcs{}
		windowsWorldExtras[engine.dll] = procs
	}
	windowsWorldExtrasMu.Unlock()
	procs.once.Do(func() {
		procs.f0, _ = engine.dll.FindProc("UtauTTSWorldF0")
		procs.open, _ = engine.dll.FindProc("UtauTTSWorldStreamOpen")
		procs.add, _ = engine.dll.FindProc("UtauTTSWorldStreamAdd")
		procs.synthesize, _ = engine.dll.FindProc("UtauTTSWorldStreamSynthesize")
		procs.close, _ = engine.dll.FindProc("UtauTTSWorldStreamClose")
	})
	return procs
}

func (engine *windowsWorldEngine) f0(samples []float64, sampleRate int, frameMS float64) ([]float64, error) {
	procs := engine.extras()
	if procs.f0 == nil {
		return nil, errors.New("WORLD engine export UtauTTSWorldF0 is missing")
	}
	errorBuffer := make([]byte, 512)
	output := make([]float64, int(float64(len(samples))*1000/float64(sampleRate)/frameMS)+2)
	count, _, _ := procs.f0.Call(windowsSlicePointer(samples), uintptr(len(samples)), uintptr(sampleRate),
		uintptr(math.Float64bits(frameMS)), windowsSlicePointer(output), uintptr(len(output)),
		windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)))
	runtime.KeepAlive(samples)
	runtime.KeepAlive(output)
	if int32(count) <= 0 {
		return nil, fmt.Errorf("WORLD F0: %s", worldError(errorBuffer))
	}
	return output[:int32(count)], nil
}

type windowsWorldStream struct {
	procs  *windowsWorldProcs
	handle uintptr
}

func (engine *windowsWorldEngine) openStream(sampleRate int, frameMS float64, fftSize, bufferSize, ringSize int) (worldStream, error) {
	procs := engine.extras()
	if procs.open == nil || procs.add == nil || procs.synthesize == nil || procs.close == nil {
		return nil, errWorldStreamUnavailable
	}
	handle, _, _ := procs.open.Call(uintptr(sampleRate), uintptr(math.Float64bits(frameMS)), uintptr(fftSize), uintptr(bufferSize), uintptr(ringSize))
	if handle == 0 {
		return nil, errors.New("WORLD streaming synthesis could not start")
	}
	return &windowsWorldStream{procs: procs, handle: handle}, nil
}

func (s *windowsWorldStream) add(f0, spectrum, aperiodicity []float64) bool {
	ok, _, _ := s.procs.add.Call(s.handle, windowsSlicePointer(f0), uintptr(len(f0)), windowsSlicePointer(spectrum), windowsSlicePointer(aperiodicity))
	runtime.KeepAlive(f0)
	runtime.KeepAlive(spectrum)
	runtime.KeepAlive(aperiodicity)
	return int32(ok) == 1
}

func (s *windowsWorldStream) synthesize(output []float64) bool {
	ok, _, _ := s.procs.synthesize.Call(s.handle, uintptr(unsafe.Pointer(&output[0])))
	runtime.KeepAlive(output)
	return int32(ok) == 1
}

func (s *windowsWorldStream) close() {
	if s.handle != 0 {
		s.procs.close.Call(s.handle)
		s.handle = 0
	}
}
