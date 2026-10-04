// source-span-auto builds an unverified source-phone library from selected spans.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/audio"
)

func read(path string) (map[string]any, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var x map[string]any
	e = json.Unmarshal(b, &x)
	return x, e
}
func hashFile(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h), nil
}
func clipIdentity(path string) (string, float64, error) {
	wav, e := audio.ReadWav(path)
	if e != nil {
		return "", 0, e
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
func checkedPhones(raw any, duration float64) error {
	phones, ok := raw.([]any)
	if !ok || len(phones) == 0 {
		return fmt.Errorf("empty source phone sequence")
	}
	previous := 0.0
	for _, item := range phones {
		p, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid source phone")
		}
		start, a := p["start_ms"].(float64)
		end, b := p["end_ms"].(float64)
		symbol, c := p["symbol"].(string)
		if !a || !b || !c || symbol == "" || math.IsNaN(start) || math.IsNaN(end) || start < previous-.001 || end <= start || end > duration+1 {
			return fmt.Errorf("invalid source phone intervals")
		}
		previous = end
	}
	return nil
}
func build(paths []string, out string, preferLast bool) (map[string]any, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one --spans is required")
	}
	absOut, e := filepath.Abs(out)
	if e != nil {
		return nil, e
	}
	repoOut, e := filepath.Abs("out")
	if e != nil {
		return nil, e
	}
	if rel, e := filepath.Rel(repoOut, absOut); e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("output must be under out/")
	}
	if _, e := os.Stat(out); e == nil {
		return nil, fmt.Errorf("refusing to overwrite %s", out)
	}
	entries := map[string]map[string]any{}
	order := []string{}
	replacements := []any{}
	language := ""
	for _, path := range paths {
		report, e := read(path)
		if e != nil {
			return nil, e
		}
		if report["time_origin"] != "oto-offset" {
			return nil, fmt.Errorf("oto-offset library required")
		}
		lang, _ := report["language"].(string)
		if language != "" && language != lang {
			return nil, fmt.Errorf("mixed library languages")
		}
		language = lang
		units, ok := report["units"].([]any)
		if !ok {
			return nil, fmt.Errorf("missing units")
		}
		reportHash, e := hashFile(path)
		if e != nil {
			return nil, e
		}
		absolute, e := filepath.Abs(path)
		if e != nil {
			return nil, e
		}
		for _, item := range units {
			unit, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid unit")
			}
			clip, _ := unit["original_clip"].(string)
			digest, duration, e := clipIdentity(clip)
			if e != nil {
				return nil, e
			}
			if digest != unit["source_sha256"] {
				return nil, fmt.Errorf("library source PCM changed")
			}
			phones := unit["source_clip_phone_intervals"]
			if e := checkedPhones(phones, duration); e != nil {
				return nil, e
			}
			provenance := map[string]any{"report": absolute, "report_sha256": reportHash, "alias": unit["alias"]}
			row := map[string]any{"source_sha256": digest, "duration_ms": duration, "phones": phones, "acoustic_model": unit["acoustic_model"], "alignment_sha256": unit["alignment_sha256"], "status": "unverified", "training_eligible": false, "provenance": provenance}
			if old, exists := entries[digest]; exists {
				oldPhones, _ := json.Marshal(old["phones"])
				newPhones, _ := json.Marshal(phones)
				if string(oldPhones) != string(newPhones) || old["acoustic_model"] != row["acoustic_model"] {
					if !preferLast {
						return nil, fmt.Errorf("conflicting source hypotheses; choose a library or explicitly prefer the last report")
					}
					replacements = append(replacements, map[string]any{"source_sha256": digest, "previous": old["provenance"], "selected": provenance})
				}
			} else {
				order = append(order, digest)
			}
			entries[digest] = row
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty source library")
	}
	rows := make([]any, len(order))
	for i, key := range order {
		rows[i] = entries[key]
	}
	result := map[string]any{"version": 1, "language": language, "time_origin": "oto-offset", "kind": "experimental-unverified-source-phone-library", "entries": rows, "replacements": replacements}
	b, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(out), 0755); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		return nil, e
	}
	_, e = f.Write(append(b, '\n'))
	closeError := f.Close()
	if e != nil {
		return nil, e
	}
	return result, closeError
}

type spansFlag []string

func (s *spansFlag) String() string     { return strings.Join(*s, ",") }
func (s *spansFlag) Set(v string) error { *s = append(*s, v); return nil }
func main() {
	if len(os.Args) < 2 || os.Args[1] != "build" {
		fmt.Fprintln(os.Stderr, "usage: source-span-auto build --spans PATH [--spans PATH] --out out/library.json")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	var paths spansFlag
	fs.Var(&paths, "spans", "selected spans JSON (repeatable)")
	out := fs.String("out", "", "new library JSON under out/")
	prefer := fs.Bool("prefer-last", false, "replace conflicting source hypothesis")
	fs.Parse(os.Args[2:])
	if *out == "" {
		fmt.Fprintln(os.Stderr, "--out required")
		os.Exit(2)
	}
	result, e := build(paths, *out, *prefer)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("indexed %d sources; explicit replacements: %d\n", len(result["entries"].([]any)), len(result["replacements"].([]any)))
}
