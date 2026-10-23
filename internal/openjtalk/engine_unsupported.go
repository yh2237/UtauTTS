//go:build !js && !windows && !((linux || darwin) && cgo)

package openjtalk

import "errors"

func openNativeEngine(string) (nativeEngine, error) {
	return nil, errors.New("Open JTalk engine is unavailable on this platform")
}
