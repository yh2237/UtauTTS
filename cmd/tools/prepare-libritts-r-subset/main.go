package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type item struct {
	ID        string  `json:"id"`
	Speaker   string  `json:"speaker"`
	AudioPath string  `json:"audio_path"`
	Duration  float64 `json:"duration_s"`
	Text      string  `json:"text"`
}

func run(archivePath, out string, speakerLimit, perSpeaker int) ([]item, error) {
	if speakerLimit < 1 || perSpeaker < 1 {
		return nil, fmt.Errorf("speaker and utterance limits must be positive")
	}
	if !toolutil.UnderOutChild(out) {
		return nil, fmt.Errorf("output must be under out/")
	}
	if _, e := os.Stat(out); e == nil {
		return nil, fmt.Errorf("refusing to overwrite %s", out)
	}
	f, e := os.Open(archivePath)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return nil, e
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	if e = os.MkdirAll(out, 0755); e != nil {
		return nil, e
	}
	texts := map[string]string{}
	chosen := map[string]item{}
	order := []string{}
	counts := map[string]int{}
	speakers := map[string]bool{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("archive incomplete: %w", err)
		}
		parts := strings.Split(strings.TrimPrefix(header.Name, "./"), "/")
		if len(parts) != 5 || parts[1] != "train-clean-100" || !header.FileInfo().Mode().IsRegular() {
			continue
		}
		speaker, name := parts[2], parts[4]
		if !speakers[speaker] {
			if len(speakers) >= speakerLimit {
				break
			}
			speakers[speaker] = true
		}
		if strings.HasSuffix(name, ".normalized.txt") {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			texts[strings.TrimSuffix(name, ".normalized.txt")] = strings.TrimSpace(string(data))
		} else if strings.HasSuffix(name, ".wav") && counts[speaker] < perSpeaker {
			path := filepath.Join(out, speaker, name)
			if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
				return nil, e
			}
			dst, err := toolutil.CreateExclusive(path)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(dst, tarReader)
			closeErr := dst.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			wav, err := audio.ReadWav(path)
			if err != nil {
				return nil, err
			}
			duration := float64(len(wav.Data)) / float64(wav.Channels*wav.SampleRate)
			if duration >= 1 && duration <= 12 {
				abs, _ := filepath.Abs(path)
				id := strings.TrimSuffix(name, ".wav")
				if _, ok := chosen[id]; !ok {
					order = append(order, id)
				}
				chosen[id] = item{ID: id, Speaker: speaker, AudioPath: abs, Duration: duration}
				counts[speaker]++
			} else {
				if err = os.Remove(path); err != nil {
					return nil, err
				}
			}
		}
	}
	rows := []item{}
	for _, id := range order {
		row := chosen[id]
		text, ok := texts[id]
		if !ok {
			continue
		}
		row.Text = text
		lab := filepath.Join(out, row.Speaker, id+".lab")
		if e = os.WriteFile(lab, []byte(text), 0644); e != nil {
			return nil, e
		}
		rows = append(rows, row)
	}
	data, e := json.MarshalIndent(rows, "", "  ")
	if e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(out, "manifest.json"), data, 0644); e != nil {
		return nil, e
	}
	return rows, nil
}
func main() {
	archive := flag.String("archive", "", "LibriTTS-R tar.gz")
	out := flag.String("out", "", "new output directory under out/")
	speakers := flag.Int("speakers", 32, "maximum speakers")
	perSpeaker := flag.Int("per-speaker", 40, "maximum utterances per speaker")
	flag.Parse()
	if *archive == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "--archive and --out are required")
		os.Exit(2)
	}
	rows, e := run(*archive, *out, *speakers, *perSpeaker)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	seen := map[string]bool{}
	hours := 0.0
	for _, r := range rows {
		seen[r.Speaker] = true
		hours += r.Duration / 3600
	}
	fmt.Printf("speakers %d utterances %d hours %.6f\n", len(seen), len(rows), hours)
}
