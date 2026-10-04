//go:build windows

package main

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"syscall"
	"unsafe"
)

func worldF0(samples []float64, rate int, frameMs float64, path string) ([]float64, error) {
	if len(samples) < 2 {
		return nil, nil
	}
	if path == "" {
		path = "runtime/utautts-world-engine.dll"
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	defer dll.Release()
	proc, err := dll.FindProc("UtauTTSWorldF0")
	if err != nil {
		return nil, err
	}
	f0 := make([]float64, int(math.Floor(1000*float64(len(samples))/float64(rate)/frameMs))+2)
	message := make([]byte, 1024)
	step := math.Float64bits(frameMs)
	count, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&samples[0])), uintptr(len(samples)), uintptr(rate),
		uintptr(step), uintptr(unsafe.Pointer(&f0[0])), uintptr(len(f0)),
		uintptr(unsafe.Pointer(&message[0])), uintptr(len(message)),
	)
	runtime.KeepAlive(samples)
	runtime.KeepAlive(f0)
	runtime.KeepAlive(message)
	if int32(count) <= 0 {
		return nil, fmt.Errorf("UtauTTSWorldF0: %s", bytes.TrimRight(message, "\x00"))
	}
	return f0[:count], nil
}
