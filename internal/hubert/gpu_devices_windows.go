//go:build windows

package hubert

import (
	"runtime"

	"github.com/yh2237/gograd/cuda"
	_ "github.com/yh2237/gograd/directml"
	"github.com/yh2237/gograd/gpu"
	"github.com/yh2237/gograd/tensor"
	_ "github.com/yh2237/gograd/vulkan"
)

// enterGPUDevice prepares the device and returns a function that releases it.
func enterGPUDevice(device tensor.Device) (func(), error) {
	switch device {
	case tensor.CUDA:
		if !cuda.Available() {
			return nil, cuda.ErrUnavailable
		}
		if err := cuda.SetDevice(0); err != nil {
			return nil, err
		}
		runtime.LockOSThread()
		return func() { runtime.UnlockOSThread() }, nil
	case tensor.DirectML:
		if !gpu.Available("directml") {
			return nil, gpu.ErrUnavailable
		}
	case tensor.Vulkan:
		if !gpu.Available("vulkan") {
			return nil, gpu.ErrUnavailable
		}
	default:
		return nil, gpu.ErrUnavailable
	}
	return func() {}, nil
}
