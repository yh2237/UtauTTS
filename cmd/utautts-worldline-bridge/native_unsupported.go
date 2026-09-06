//go:build !windows && ((!linux && !darwin) || !cgo)

package main

import "fmt"

func openNativeLibrary(string) (nativeLibrary, error) {
	return nil, fmt.Errorf("native worldline bridge is unavailable on this platform")
}
