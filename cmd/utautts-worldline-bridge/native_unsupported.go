//go:build !windows && (!linux || !cgo)

package main

import "fmt"

func openNativeLibrary(string) (nativeLibrary, error) {
	return nil, fmt.Errorf("native worldline bridge is unavailable on this platform")
}
