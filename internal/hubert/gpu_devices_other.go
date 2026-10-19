//go:build !windows && !linux

package hubert

import (
	"github.com/yh2237/gograd/gpu"
	"github.com/yh2237/gograd/tensor"
)

// enterGPUDeviceは、この環境ではGPUを使わない。
func enterGPUDevice(device tensor.Device) (func(), error) {
	return nil, gpu.ErrUnavailable
}
