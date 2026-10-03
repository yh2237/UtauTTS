//go:build !windows

package main

import "fmt"

type worldEngine struct{}

func openWorld(path string) (*worldEngine, error) {
	return nil, fmt.Errorf("WORLD DLL analysis requires Windows: %s", path)
}
func (*worldEngine) close() {}
func (*worldEngine) analyze([]float64, int) ([]float64, []float64, int, error) {
	return nil, nil, 0, fmt.Errorf("WORLD DLL analysis requires Windows")
}
