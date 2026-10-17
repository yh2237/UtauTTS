//go:build windows

package hubert

import "os"

// featuresAcceleratedは、UTAUTTS_CUDAが設定されていればCUDAで特徴量を求める。
// CUDAが使えないときはCPUに戻す。
func (m *Model) featuresAccelerated(wave []float32, layer int) ([][]float32, error) {
	if os.Getenv("UTAUTTS_CUDA") == "" {
		return m.Features(wave, layer)
	}
	rows, err := m.FeaturesCUDA(wave, layer)
	if err != nil {
		return m.Features(wave, layer)
	}
	return rows, nil
}
