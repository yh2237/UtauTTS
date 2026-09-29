//go:build !js && !windows && ((!linux && !darwin) || !cgo)

package worldrender

import "fmt"

func openWorldEngine(string) (worldEngine, error) {
	return nil, fmt.Errorf("UtauTTS WORLD engine is unavailable on this platform")
}
