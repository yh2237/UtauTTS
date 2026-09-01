//go:build !windows

package main

import "fmt"

func invokeCUDAWorldFeatureMix(string, []float64, int, []cudaWorldUnit,
	[]float64, []float64, []float64, []float64, []float64, []float64) error {
	return fmt.Errorf("CUDA WORLD feature mix is only available on Windows")
}
