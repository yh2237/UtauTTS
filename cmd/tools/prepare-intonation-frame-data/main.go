// prepare-intonation-frame-data creates uniformly timed Japanese mora records.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
)

func activeBounds(samples []int16, rate int) (float64, float64, error) {
	frame := int(math.RoundToEven(float64(rate) * .02))
	if frame < 1 {
		frame = 1
	}
	energies := []float64{}
	peak := 0.0
	for start := 0; start < len(samples); start += frame {
		end := min(len(samples), start+frame)
		sum := 0.0
		for _, s := range samples[start:end] {
			sum += float64(s) * float64(s)
		}
		v := math.Sqrt(sum / float64(end-start))
		energies = append(energies, v)
		peak = math.Max(peak, v)
	}
	threshold := math.Max(80, peak*math.Pow(10, -35.0/20))
	first, last := -1, -1
	for i, v := range energies {
		if v >= threshold {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return 0, 0, fmt.Errorf("no active speech detected")
	}
	return math.Max(0, float64(first*frame)*1000/float64(rate)-30), math.Min(float64(len(samples))*1000/float64(rate), float64((last+1)*frame)*1000/float64(rate)+30), nil
}

func tokensFor(a *openjtalk.Analysis, start, end float64) ([]map[string]any, error) {
	kana, err := frontend.ParseKana(a.Reading)
	if err != nil {
		return nil, err
	}
	if len(kana) != len(a.Features) {
		return nil, fmt.Errorf("Open JTalk mora count differs")
	}
	tokens := make([]map[string]any, len(a.Features))
	spoken, pauses := 0, 0
	for i, f := range a.Features {
		if len(f) == 0 {
			tokens[i] = map[string]any{"mora": "", "pause": true}
			pauses++
			continue
		}
		spoken++
		tokens[i] = map[string]any{"mora": a.Morae[i], "vowel": kana[i].Vowel, "pause": false, "accent_high": f["accent_high"] == 1, "accent_phrase_start": f["accent_phrase_start"] == 1, "accent_phrase_end": f["accent_phrase_end"] == 1, "word_start": f["word_start"] == 1, "word_end": f["word_end"] == 1}
		for k := range f {
			if strings.HasPrefix(k, "pos=") {
				tokens[i]["pos"] = strings.TrimPrefix(k, "pos=")
			}
			if strings.HasPrefix(k, "pos_group1=") {
				tokens[i]["pos_group1"] = strings.TrimPrefix(k, "pos_group1=")
			}
		}
	}
	if spoken == 0 {
		return nil, fmt.Errorf("Open JTalk produced no morae")
	}
	for i := 0; i < len(tokens); {
		if tokens[i]["pause"] == true {
			i++
			continue
		}
		j := i + 1
		for j < len(tokens) && tokens[j]["pause"] == false && tokens[j-1]["accent_phrase_end"] == false {
			j++
		}
		length := j - i
		for k := i; k < j; k++ {
			f := a.Features[k]
			tokens[k]["accent_phrase_position"] = int(math.Round(f["accent_position"] * float64(length)))
			tokens[k]["accent_phrase_length"] = length
			tokens[k]["accent_nucleus"] = int(math.Round(f["accent_nucleus_position"] * float64(length)))
		}
		i = j
	}
	pauseMS := 0.0
	if pauses > 0 {
		pauseMS = math.Min(180, (end-start)*.08)
	}
	moraMS := (end - start - pauseMS*float64(pauses)) / float64(spoken)
	if moraMS <= 20 {
		return nil, fmt.Errorf("audio is too short for its mora count")
	}
	cursor := start
	for _, t := range tokens {
		d := moraMS
		if t["pause"] == true {
			d = pauseMS
		}
		t["start_ms"] = cursor
		t["end_ms"] = cursor + d
		t["duration_ms"] = d
		cursor += d
	}
	tokens[len(tokens)-1]["end_ms"] = end
	tokens[len(tokens)-1]["duration_ms"] = end - tokens[len(tokens)-1]["start_ms"].(float64)
	return tokens, nil
}

func run(corpus, out string, limit int, allowMismatch bool, cfg openjtalk.Config) (int, int, error) {
	if !allowMismatch {
		return 0, 0, fmt.Errorf("native Open JTalk helper does not expose the g2p phone stream; use --allow-reading-mismatch for kana-reading corpora")
	}
	clean := filepath.Clean(out)
	if !strings.HasPrefix(clean, "out"+string(filepath.Separator)) {
		return 0, 0, fmt.Errorf("output must be under out/")
	}
	if _, err := os.Stat(out); err == nil {
		return 0, 0, fmt.Errorf("refusing to overwrite %s", out)
	}
	f, err := os.Open(filepath.Join(corpus, "metadata.csv"))
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '|'
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, 0, err
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	records := []map[string]any{}
	skipped := 0
	for _, row := range rows {
		if len(row) < 3 {
			skipped++
			continue
		}
		id, sourceText, supplied := row[0], row[1], row[2]
		text := strings.Join(strings.Fields(sourceText), "")
		wavPath := filepath.Join(corpus, "wavs", id+".wav")
		if _, e := os.Stat(wavPath); e != nil {
			skipped++
			continue
		}
		a, e := openjtalk.Analyze(text, cfg)
		if e != nil {
			skipped++
			continue
		}
		// The native helper exposes mora features but not pyopenjtalk.g2p's phone stream.
		wav, e := audio.ReadWav(wavPath)
		if e != nil || wav.Channels != 1 {
			skipped++
			continue
		}
		start, end, e := activeBounds(wav.Data, wav.SampleRate)
		if e != nil {
			skipped++
			continue
		}
		tokens, e := tokensFor(a, start, end)
		if e != nil {
			skipped++
			continue
		}
		abs, e := filepath.Abs(wavPath)
		if e != nil {
			return 0, 0, e
		}
		records = append(records, map[string]any{"version": 1, "id": id, "text": text, "source_text": sourceText, "audio_path": abs, "tokens": tokens, "accent_source": "openjtalk", "alignment_source": "uniform_mora_with_energy_bounds", "source_reading": supplied, "openjtalk_reading": a.Reading})
	}
	if len(records) == 0 {
		return 0, skipped, fmt.Errorf("no usable records")
	}
	if e := os.MkdirAll(filepath.Dir(out), 0755); e != nil {
		return 0, skipped, e
	}
	outFile, e := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		return 0, skipped, e
	}
	defer outFile.Close()
	for _, record := range records {
		line, e := json.Marshal(record)
		if e != nil {
			return 0, skipped, e
		}
		if _, e = outFile.Write(append(line, '\n')); e != nil {
			return 0, skipped, e
		}
	}
	return len(records), skipped, nil
}
func main() {
	corpus := flag.String("corpus", "", "corpus directory")
	out := flag.String("out", "", "new JSONL under out/")
	limit := flag.Int("limit", 0, "row limit")
	allow := flag.Bool("allow-reading-mismatch", false, "permit missing G2P phone validation")
	alignment := flag.String("alignment", "uniform", "uniform mora timing (Viterbi retired)")
	helper := flag.String("openjtalk-helper", "", "native helper path")
	dictionary := flag.String("openjtalk-dictionary", "", "dictionary path")
	flag.Parse()
	if *corpus == "" || *out == "" || *alignment != "uniform" {
		fmt.Fprintln(os.Stderr, "corpus, out, and --alignment uniform are required")
		os.Exit(2)
	}
	n, skip, err := run(*corpus, *out, *limit, *allow, openjtalk.Config{HelperPath: *helper, DictionaryPath: *dictionary})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d records; skipped %d\n", n, skip)
}
