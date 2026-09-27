package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"utautts/internal/plan"
	"utautts/internal/tts"
)

func main() {
	path := flag.String("plan", "", "synthesis plan with linguistic phones")
	out := flag.String("out", "", "unobserved training and boundary-review template JSON")
	id := flag.String("id", "", "natural utterance ID to align")
	base := flag.Float64("base-ms", 120, "rule phone-duration baseline")
	flag.Parse()
	if err := run(*path, *out, *id, *base); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path, out, id string, base float64) error {
	if path == "" || out == "" || id == "" || base <= 0 || math.IsNaN(base) || math.IsInf(base, 0) {
		return fmt.Errorf("--plan --out --id and positive --base-ms required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Language != "en" && p.Language != "zh" {
		return fmt.Errorf("speech templates support en/zh")
	}
	if len(p.Morae) == 0 {
		analysis, err := tts.Analyze(tts.Config{Language: p.Language, Phonemizer: p.Phonemizer, Reading: p.Reading, Text: p.Text})
		if err != nil {
			return err
		}
		p.Morae = analysis.Morae
	}
	phones := tts.SpeechTrainingPhones(p.Morae, base)
	if len(phones) == 0 {
		return fmt.Errorf("plan has no linguistic phones")
	}
	if len(phones) != len(p.PhoneTimings) {
		return fmt.Errorf("linguistic phones differ from plan timings; regenerate plan")
	}
	for i, row := range phones {
		timing := p.PhoneTimings[i]
		if row.Position != timing.Position || row.Symbol != timing.Symbol || row.Role != timing.Role {
			return fmt.Errorf("linguistic phone identity differs from plan at %d; regenerate plan", i)
		}
	}
	text := p.Text
	if text == "" {
		text = p.Reading
	}
	value := map[string]any{"version": 1, "feature_version": 1, "id": id, "language": p.Language, "text": text,
		"baseline_base_ms": base, "observation_status": "unobserved", "phones": phones, "source_units": p.Units}
	data, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), 0644)
}
