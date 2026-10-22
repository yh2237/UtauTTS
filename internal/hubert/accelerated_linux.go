//go:build linux

package hubert

import (
	"os"

	"github.com/yh2237/gograd/tensor"
)

// featuresAcceleratedは、Vulkanが使えればVulkanで特徴量を求め、使えなければCPUに戻す。
func (m *Model) featuresAccelerated(wave []float32, layer int, stats *GroupStats) ([][]float32, error) {
	if os.Getenv("UTAUTTS_NO_GPU") == "" && os.Getenv("UTAUTTS_NO_VULKAN") == "" && m.config.canUseCUDA() {
		if rows, err := m.featuresOnDevice(tensor.Vulkan, wave, layer, stats); err == nil {
			return rows, nil
		}
	}
	return m.features(wave, layer, stats)
}
