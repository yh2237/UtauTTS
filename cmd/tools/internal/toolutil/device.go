package toolutil

import (
	"fmt"

	"github.com/yh2237/gograd/cuda"
	"github.com/yh2237/gograd/tensor"
)

// ResolveDeviceはauto/cuda/cpu指定を検証して学習デバイスを返す。
func ResolveDevice(name string) (tensor.Device, error) {
	switch name {
	case "auto":
		if cuda.Available() {
			return tensor.CUDA, nil
		}
		return tensor.CPU, nil
	case "cuda":
		if !cuda.Available() {
			return tensor.CPU, fmt.Errorf("CUDA unavailable")
		}
		return tensor.CUDA, nil
	case "cpu":
		return tensor.CPU, nil
	default:
		return tensor.CPU, fmt.Errorf("invalid device %q", name)
	}
}
