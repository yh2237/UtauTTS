//go:build linux

package hubert

import (
	"github.com/yh2237/gograd/gpu"
	"github.com/yh2237/gograd/tensor"
	_ "github.com/yh2237/gograd/vulkan"
)

// enterGPUDevice prepares the device and returns a function that releases it.
func enterGPUDevice(device tensor.Device) (func(), error) {
	if device != tensor.Vulkan {
		return nil, gpu.ErrUnavailable
	}
	if !gpu.Available("vulkan") {
		return nil, gpu.ErrUnavailable
	}
	return func() {}, nil
}
