// Package sourcephone は原音のPCM同一性と音素区間を扱う。
package sourcephone

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type Object = map[string]any

func Map(v any) Object     { x, _ := v.(map[string]any); return x }
func List(v any) []any     { x, _ := v.([]any); return x }
func String(v any) string  { x, _ := v.(string); return x }
func Number(v any) float64 { x, _ := v.(float64); return x }
func Int(v any) int        { return int(Number(v)) }
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
func HashFile(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
func ClipIdentity(file string) (string, float64, error) {
	wav, err := audio.ReadWav(file)
	if err != nil {
		return "", 0, err
	}
	if wav.Channels < 1 || wav.SampleRate < 1 {
		return "", 0, fmt.Errorf("16-bit PCM source required")
	}
	h := sha256.New()
	fmt.Fprintf(h, "v1/%d/%d/", wav.SampleRate, wav.Channels)
	for _, s := range wav.Data {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(s))
		h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil)), float64(len(wav.Data)/wav.Channels) * 1000 / float64(wav.SampleRate), nil
}
func Normalized(phone string) string {
	return strings.ToLower(regexp.MustCompile(`[012]$`).ReplaceAllString(phone, ""))
}
func Absolute(file string) string { v, _ := filepath.Abs(file); return v }
func ResolvedSource(report, clip string) string {
	if filepath.IsAbs(clip) {
		return Absolute(clip)
	}
	return Absolute(filepath.Join(filepath.Dir(report), clip))
}
func CheckedPhones(phones []any, duration float64) error {
	if len(phones) == 0 {
		return fmt.Errorf("empty source phone sequence")
	}
	previous := 0.0
	for _, value := range phones {
		p := Map(value)
		start, end := Number(p["start_ms"]), Number(p["end_ms"])
		if String(p["symbol"]) == "" || math.IsNaN(start) || math.IsInf(start, 0) || math.IsNaN(end) || math.IsInf(end, 0) || start < previous-.001 || end <= start || end > duration+1 {
			return fmt.Errorf("invalid source phone intervals")
		}
		previous = end
	}
	return nil
}
func UnitByIndex(units []any) map[int]Object {
	m := map[int]Object{}
	for _, raw := range units {
		u := Map(raw)
		m[Int(u["unit_index"])] = u
	}
	return m
}
func UniqueStrings(values []string) []string {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	result := make([]string, 0, len(m))
	for v := range m {
		result = append(result, v)
	}
	sortStrings(result)
	return result
}
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
