// Package sourcephone は整列ツールが使うJSONとファイルの補助関数。
package sourcephone

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/yh2237/UtauTTS/cmd/tools/internal/toolutil"
)

type Object = map[string]any

func Map(v any) Object     { x, _ := v.(map[string]any); return x }
func List(v any) []any     { x, _ := v.([]any); return x }
func String(v any) string  { x, _ := v.(string); return x }
func Number(v any) float64 { x, _ := v.(float64); return x }
func Bool(v any) bool      { x, _ := v.(bool); return x }

func Read(file string) (Object, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var value Object
	err = json.Unmarshal(data, &value)
	return value, err
}
func Write(file string, value any) error {
	if err := toolutil.RequireUnderOut(file, "output", false); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := toolutil.CreateExclusive(file)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func FreshDir(dir string) error {
	if err := toolutil.RequireUnderOut(dir, "output", false); err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("use a fresh output directory: %s", dir)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0755)
}
func Absolute(file string) string { v, _ := filepath.Abs(file); return v }
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
func Finite(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
