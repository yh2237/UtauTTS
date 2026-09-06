//go:build !windows && ((!linux && !darwin) || !cgo)

package main

import "fmt"

func openWorldEngine(string) (worldEngine, error) {
	return nil, fmt.Errorf("UtauTTS WORLD engine is unavailable on this platform")
}
