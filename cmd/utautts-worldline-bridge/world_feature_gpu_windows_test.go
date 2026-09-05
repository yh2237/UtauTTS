//go:build windows

package main

import (
	"math"
	"os"
	"testing"
)

func TestCUDAWorldFeatureMixMatchesCPU(t *testing.T) {
	path := os.Getenv("UTAUTTS_TEST_WORLD_GPU")
	if path == "" {
		t.Skip("set UTAUTTS_TEST_WORLD_GPU to the rebuilt V2 DLL")
	}
	input, prepared, fftSize := worldMixFixture()
	// 同じ音源を再利用しても、2ユニット目の配置時刻を失わないことを確認する。
	prepared[1] = prepared[0]
	for i := range prepared {
		for frame := range prepared[i].cached.features.F0 {
			if (frame+i)%3 == 0 {
				prepared[i].cached.features.F0[frame] = 0
			}
		}
	}
	cpu := mixWorldFeatures(input, prepared, fftSize, 1)
	gpu, err := mixWorldFeaturesCUDA(path, input, prepared, fftSize)
	if err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string][2][]float64{"f0": {cpu.F0, gpu.F0}, "spectrum": {cpu.Spectrum, gpu.Spectrum}, "ap": {cpu.Aperiodicity, gpu.Aperiodicity}} {
		for i, expected := range pair[0] {
			actual := pair[1][i]
			if math.IsNaN(actual) || math.Abs(actual-expected) > 1e-8*math.Max(1, math.Abs(expected)) {
				t.Fatalf("%s[%d]: CPU=%g GPU=%g", name, i, expected, actual)
			}
		}
	}
}
