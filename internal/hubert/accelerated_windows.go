//go:build windows

package hubert

import "os"

// featuresAcceleratedは、CUDAが使えればCUDAで特徴量を求める。
// UTAUTTS_NO_CUDAが設定されているとき、HuBERT Base以外のモデル、CUDAで失敗したときはCPUに戻す。
func (m *Model) featuresAccelerated(wave []float32, layer int) ([][]float32, error) {
	if os.Getenv("UTAUTTS_NO_CUDA") != "" || !m.config.canUseCUDA() {
		return m.Features(wave, layer)
	}
	rows, err := m.FeaturesCUDA(wave, layer)
	if err != nil {
		return m.Features(wave, layer)
	}
	return rows, nil
}
