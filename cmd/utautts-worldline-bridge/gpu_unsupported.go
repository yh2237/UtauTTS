//go:build !windows

package main

import "fmt"

func mixClassicGPU(_ string, _ []classicSegment, _, _ int) ([]float32, error) {
	return nil, fmt.Errorf("faithful GPU renderer is available on Windows only")
}
