//go:build !windows

package main

import "fmt"

func worldF0(_ []float64, _ int, _ float64, _ string) ([]float64, error) {
	return nil, fmt.Errorf("WORLD DLL extraction is available on Windows only; use cached observations on this platform")
}
