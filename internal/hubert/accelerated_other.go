//go:build !windows && !linux

package hubert

// featuresAcceleratedは、この環境ではCPUで特徴量を求める。
func (m *Model) featuresAccelerated(wave []float32, layer int) ([][]float32, error) {
	return m.Features(wave, layer)
}
