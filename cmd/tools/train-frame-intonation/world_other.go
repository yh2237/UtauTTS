//go:build !windows

package main

import "fmt"

func worldF0(_ record, _ float64, _ string) ([]float64, error) {
	return nil, fmt.Errorf("WORLD DLL extraction is available on Windows only; use an existing F0 cache on this platform")
}
