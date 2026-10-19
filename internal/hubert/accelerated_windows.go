//go:build windows

package hubert

import (
	"os"

	"github.com/yh2237/gograd/tensor"
)

// featuresAcceleratedは、CUDA、DirectML、Vulkanの順に試し、使えなければCPUに戻す。
func (m *Model) featuresAccelerated(wave []float32, layer int) ([][]float32, error) {
	for _, device := range m.acceleratedDevices() {
		rows, err := m.featuresOnDevice(device, wave, layer)
		if err == nil {
			return rows, nil
		}
	}
	return m.Features(wave, layer)
}

// acceleratedDevicesは、設定と環境変数から試す順にデバイスを返す。
func (m *Model) acceleratedDevices() []tensor.Device {
	if os.Getenv("UTAUTTS_NO_GPU") != "" || !m.config.canUseCUDA() {
		return nil
	}
	devices := make([]tensor.Device, 0, 3)
	if os.Getenv("UTAUTTS_NO_CUDA") == "" {
		devices = append(devices, tensor.CUDA)
	}
	if os.Getenv("UTAUTTS_NO_DIRECTML") == "" {
		devices = append(devices, tensor.DirectML)
	}
	if os.Getenv("UTAUTTS_NO_VULKAN") == "" {
		devices = append(devices, tensor.Vulkan)
	}
	return devices
}
