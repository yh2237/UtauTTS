//go:build !windows

package main

func relaunchElevatedIfNeeded(string, []string) (bool, error) {
	return false, nil
}
