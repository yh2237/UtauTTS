//go:build !windows

package worldffi

import "errors"

// AnalyzeF0はWindows専用。他のOSでは事前作成したF0キャッシュを使う。
func AnalyzeF0(_ string, _ float64, _ string) ([]float64, error) {
	return nil, errors.New("WORLD engine requires Windows")
}
