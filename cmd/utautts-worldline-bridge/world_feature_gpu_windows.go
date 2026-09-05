//go:build windows

package main

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
)

type cudaWorldFeatureLibrary struct {
	dll       *syscall.DLL
	available *syscall.Proc
	mix       *syscall.Proc
}

var cudaWorldFeatureLibraries = struct {
	sync.Mutex
	items map[string]*cudaWorldFeatureLibrary
}{items: make(map[string]*cudaWorldFeatureLibrary)}

func loadCUDAWorldFeatureLibrary(path string) (*cudaWorldFeatureLibrary, error) {
	cudaWorldFeatureLibraries.Lock()
	defer cudaWorldFeatureLibraries.Unlock()
	if library := cudaWorldFeatureLibraries.items[path]; library != nil {
		return library, nil
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	available, err := dll.FindProc("UtauTTSGPUAvailable")
	if err != nil {
		_ = dll.Release()
		return nil, err
	}
	mix, err := dll.FindProc("UtauTTSGPUWorldFeatureMixV2")
	if err != nil {
		_ = dll.Release()
		return nil, err
	}
	library := &cudaWorldFeatureLibrary{dll: dll, available: available, mix: mix}
	cudaWorldFeatureLibraries.items[path] = library
	return library, nil
}

func invokeCUDAWorldFeatureMix(path string, inputF0 []float64, fftSize int, units []cudaWorldUnit,
	sourceF0, sourceSpectrum, sourceAP, outputF0, outputSpectrum, outputAP []float64) error {
	library, err := loadCUDAWorldFeatureLibrary(path)
	if err != nil {
		return err
	}
	errorBuffer := make([]byte, 512)
	available, _, _ := library.available.Call(windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)))
	if available == 0 {
		return fmt.Errorf("CUDA is unavailable: %s", worldError(errorBuffer))
	}
	ok, _, _ := library.mix.Call(
		windowsSlicePointer(inputF0), uintptr(len(inputF0)), uintptr(fftSize),
		windowsSlicePointer(units), uintptr(len(units)),
		windowsSlicePointer(sourceF0), windowsSlicePointer(sourceSpectrum), windowsSlicePointer(sourceAP),
		windowsSlicePointer(outputF0), windowsSlicePointer(outputSpectrum), windowsSlicePointer(outputAP),
		windowsSlicePointer(errorBuffer), uintptr(len(errorBuffer)),
	)
	runtime.KeepAlive(inputF0)
	runtime.KeepAlive(units)
	runtime.KeepAlive(sourceF0)
	runtime.KeepAlive(sourceSpectrum)
	runtime.KeepAlive(sourceAP)
	runtime.KeepAlive(outputF0)
	runtime.KeepAlive(outputSpectrum)
	runtime.KeepAlive(outputAP)
	if ok == 0 {
		return fmt.Errorf("%s", worldError(errorBuffer))
	}
	return nil
}
