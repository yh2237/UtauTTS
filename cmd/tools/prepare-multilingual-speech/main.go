package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type point struct{ MS, HZ float64 }

func number(x any) float64     { v, _ := x.(float64); return v }
func stringValue(x any) string { v, _ := x.(string); return v }
func object(x any) (map[string]any, error) {
	v, ok := x.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object")
	}
	return v, nil
}
func list(x any) ([]any, error) {
	v, ok := x.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list")
	}
	return v, nil
}
func readJSON(path string) (map[string]any, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
	var x map[string]any
	e = json.Unmarshal(b, &x)
	return x, e
}
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	x := append([]float64(nil), values...)
	sort.Float64s(x)
	n := len(x)
	if n%2 == 1 {
		return x[n/2]
	}
	return (x[n/2-1] + x[n/2]) / 2
}
func pitchTrack(samples []float64, rate int) []point {
	size := int(math.RoundToEven(float64(rate) * .04))
	hop := max(1, int(math.RoundToEven(float64(rate)*.01)))
	var result []point
	for start := 0; start+size <= len(samples); start += hop {
		block := append([]float64(nil), samples[start:start+size]...)
		sum, sq := 0.0, 0.0
		for _, v := range block {
			sum += v
			sq += v * v
		}
		if math.Sqrt(sq/float64(size)) < .003 {
			continue
		}
		mean := sum / float64(size)
		energy := make([]float64, size+1)
		for i := range block {
			block[i] -= mean
			energy[i+1] = energy[i] + block[i]*block[i]
		}
		lo := max(1, rate/500)
		hi := min(size-2, rate/60)
		values := make([]float64, hi-lo+1)
		for lag := lo; lag <= hi; lag++ {
			corr := 0.0
			for j := 0; j < size-lag; j++ {
				corr += block[j] * block[j+lag]
			}
			norm := math.Sqrt(energy[size-lag] * (energy[size] - energy[lag]))
			values[lag-lo] = corr / math.Max(norm, 1e-12)
		}
		best := math.Inf(-1)
		for i := 1; i+1 < len(values); i++ {
			if values[i] > values[i-1] && values[i] >= values[i+1] {
				best = math.Max(best, values[i])
			}
		}
		if best < .65 {
			continue
		}
		threshold := math.Max(.65, best*.95)
		chosen := -1
		for i := 1; i+1 < len(values); i++ {
			if values[i] > values[i-1] && values[i] >= values[i+1] && values[i] >= threshold {
				chosen = i
				break
			}
		}
		if chosen >= 0 {
			result = append(result, point{(float64(start) + float64(size)/2) * 1000 / float64(rate), float64(rate) / float64(lo+chosen)})
		}
	}
	return result
}
func prepare(template, observation map[string]any, root string) (map[string]any, error) {
	for _, field := range []string{"id", "speaker", "corpus", "license", "audio_path"} {
		if stringValue(observation[field]) == "" {
			return nil, fmt.Errorf("missing provenance %s", field)
		}
	}
	if observation["kind"] != "natural" || (observation["alignment"] != "manual" && observation["alignment"] != "forced") {
		return nil, fmt.Errorf("only explicitly aligned natural recordings are training observations")
	}
	if observation["split"] != "train" && observation["split"] != "validation" && observation["split"] != "test" {
		return nil, fmt.Errorf("explicit train/validation/test split required")
	}
	if number(template["version"]) != 1 || number(template["feature_version"]) != 1 || template["id"] != observation["id"] {
		return nil, fmt.Errorf("template identity/version mismatch")
	}
	path := filepath.Join(root, stringValue(observation["audio_path"]))
	wav, e := audio.ReadWav(path)
	if e != nil {
		return nil, e
	}
	if wav.Channels != 1 {
		return nil, fmt.Errorf("expected mono 16-bit PCM natural WAV")
	}
	rate := wav.SampleRate
	samples := make([]float64, len(wav.Data))
	for i, v := range wav.Data {
		samples[i] = float64(v) / 32768
	}
	totalMS := float64(len(samples)) * 1000 / float64(rate)
	expected, e := list(template["phones"])
	if e != nil {
		return nil, e
	}
	actual, e := list(observation["phones"])
	if e != nil {
		return nil, e
	}
	if len(expected) == 0 || len(expected) != len(actual) {
		return nil, fmt.Errorf("alignment phone count differs from linguistic template")
	}
	type span struct{ Start, End, RMS float64 }
	spans := make([]span, len(actual))
	rmsValues := make([]float64, len(actual))
	last := 0.0
	for i := range actual {
		want, _ := object(expected[i])
		got, _ := object(actual[i])
		for _, field := range []string{"position", "phone_index", "symbol"} {
			if want[field] != got[field] {
				return nil, fmt.Errorf("alignment mismatch at %s", field)
			}
		}
		start, end := number(got["start_ms"]), number(got["end_ms"])
		if math.IsNaN(start+end) || math.IsInf(start+end, 0) || start < last-1e-6 || end <= start || end > totalMS+1e-6 {
			return nil, fmt.Errorf("invalid phone interval")
		}
		a := int(math.RoundToEven(start * float64(rate) / 1000))
		b := int(math.RoundToEven(end * float64(rate) / 1000))
		if b <= a || a < 0 || b > len(samples) {
			return nil, fmt.Errorf("empty measured phone")
		}
		sum := 0.0
		for _, v := range samples[a:b] {
			sum += v * v
		}
		rms := math.Max(1e-6, math.Sqrt(sum/float64(b-a)))
		spans[i] = span{start, end, rms}
		rmsValues[i] = rms
		last = end
	}
	track := pitchTrack(samples, rate)
	var speech []point
	for _, p := range track {
		for _, s := range spans {
			if s.Start <= p.MS && p.MS <= s.End {
				speech = append(speech, p)
				break
			}
		}
	}
	f0s := make([]float64, len(speech))
	for i, p := range speech {
		f0s[i] = p.HZ
	}
	medianF0 := median(f0s)
	medianEnergy := median(rmsValues)
	rows := make([]map[string]any, len(expected))
	for i, raw := range expected {
		want, _ := object(raw)
		s := spans[i]
		row := map[string]any{}
		for k, v := range want {
			row[k] = v
		}
		row["duration_ms"] = s.End - s.Start
		row["energy_log_ratio"] = math.Log(s.RMS / medianEnergy)
		var voiced []point
		for _, p := range speech {
			if s.Start <= p.MS && p.MS <= s.End {
				voiced = append(voiced, p)
			}
		}
		if len(voiced) >= 3 && medianF0 > 0 {
			pitch := []float64{}
			for _, fraction := range []float64{0, .5, 1} {
				ms := s.Start + (s.End-s.Start)*fraction
				nearest := voiced[0]
				for _, p := range voiced[1:] {
					if math.Abs(p.MS-ms) < math.Abs(nearest.MS-ms) {
						nearest = p
					}
				}
				if math.Abs(nearest.MS-ms) > math.Max(20, (s.End-s.Start)*.2) {
					pitch = nil
					break
				}
				pitch = append(pitch, 1200*math.Log2(nearest.HZ/medianF0))
			}
			if pitch != nil {
				row["pitch_cents"] = pitch
			} else {
				row["pitch_cents"] = nil
			}
		} else {
			row["pitch_cents"] = nil
		}
		rows[i] = row
	}
	bytes, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	sha := sha256.Sum256(bytes)
	return map[string]any{"version": 1, "feature_version": 1, "id": template["id"], "language": template["language"], "text": template["text"], "speaker": observation["speaker"], "split": observation["split"], "kind": "natural", "corpus": observation["corpus"], "license": observation["license"], "alignment": observation["alignment"], "audio_sha256": fmt.Sprintf("%x", sha), "pitch_source": "normalized-autocorrelation-v1", "phones": rows}, nil
}
func run(templates []string, observations, out string) ([]map[string]any, error) {
	if clean := filepath.Clean(out); !strings.HasPrefix(clean, "out"+string(filepath.Separator)) {
		return nil, fmt.Errorf("output must be under out/")
	}
	source, e := readJSON(observations)
	if e != nil {
		return nil, e
	}
	obs, e := list(source["utterances"])
	if e != nil {
		return nil, e
	}
	byID := map[string]map[string]any{}
	for _, raw := range obs {
		row, _ := object(raw)
		id := stringValue(row["id"])
		if _, ok := byID[id]; ok {
			return nil, fmt.Errorf("duplicate observation ID")
		}
		byID[id] = row
	}
	var records []map[string]any
	seen := map[string]bool{}
	for _, path := range templates {
		template, err := readJSON(path)
		if err != nil {
			return nil, err
		}
		id := stringValue(template["id"])
		if seen[id] {
			return nil, fmt.Errorf("duplicate template ID")
		}
		seen[id] = true
		row, err := prepare(template, byID[id], filepath.Dir(observations))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		records = append(records, row)
	}
	if e = os.MkdirAll(filepath.Dir(out), 0755); e != nil {
		return nil, e
	}
	if _, e = os.Stat(out); e == nil {
		return nil, fmt.Errorf("refusing to overwrite %s", out)
	}
	f, e := toolutil.CreateExclusive(out)
	if e != nil {
		return nil, e
	}
	for _, row := range records {
		b, e := json.Marshal(row)
		if e != nil {
			f.Close()
			return nil, e
		}
		if _, e = f.Write(append(b, '\n')); e != nil {
			f.Close()
			return nil, e
		}
	}
	return records, f.Close()
}
func main() {
	observations := flag.String("observations", "", "observation JSON")
	out := flag.String("out", "", "new JSONL under out/")
	flag.Parse()
	if *observations == "" || *out == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "templates, --observations, and --out are required")
		os.Exit(2)
	}
	rows, e := run(flag.Args(), *observations, *out)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("wrote %d records\n", len(rows))
}
