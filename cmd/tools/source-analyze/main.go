package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

type observation struct {
	UnitIndex       int                      `json:"unit_index"`
	Position        int                      `json:"position"`
	Role            string                   `json:"role"`
	Alias           string                   `json:"alias"`
	Source          string                   `json:"source"`
	SourceClip      string                   `json:"source_clip"`
	RequestedPhones []plan.PhoneTiming       `json:"requested_context"`
	AssignedCoda    []string                 `json:"assigned_coda_phones,omitempty"`
	Oto             oto.Entry                `json:"oto"`
	Analysis        voicebank.SourceAnalysis `json:"analysis"`
	SourceAnchors   []float64                `json:"source_anchors_ms,omitempty"`
	TargetAnchors   []float64                `json:"target_anchors_ms,omitempty"`
	Status          string                   `json:"annotation_status"`
}

func main() {
	path := flag.String("plan", "", "rendered synthesis plan")
	out := flag.String("out", "", "observation output directory")
	positions := flag.String("positions", "", "optional comma-separated linguistic positions")
	flag.Parse()
	if err := run(*path, *out, *positions); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path, out, positions string) error {
	if path == "" || out == "" {
		return fmt.Errorf("--plan and --out required")
	}
	selected := map[int]bool{}
	if positions != "" {
		for _, s := range strings.Split(positions, ",") {
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || i < 0 {
				return fmt.Errorf("invalid position %q", s)
			}
			selected[i] = true
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var p plan.Plan
	if err = json.Unmarshal(data, &p); err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	bank := &voicebank.Bank{}
	rows := []observation{}
	for i, u := range p.Units {
		if u.Silent || (len(selected) > 0 && !selected[u.Position]) {
			continue
		}
		entry := oto.Entry{Alias: u.Alias, Filename: u.Source, Offset: u.OffsetMS, Blank: u.CutoffMS, Fixed: u.ConsonantMS, Preutterance: u.PreutteranceMS, Overlap: u.OverlapMS}
		analysis, err := bank.AnalyzeSpeechSource(entry)
		if err != nil {
			return fmt.Errorf("unit %d %q: %w", i, u.Alias, err)
		}
		pcm, err := audio.ReadWav(u.Source)
		if err != nil {
			return err
		}
		pcm, err = audio.TrimPCM(pcm, u.OffsetMS, u.CutoffMS)
		if err != nil {
			return err
		}
		clip := fmt.Sprintf("unit-%03d-source.wav", i)
		if err = audio.WriteWav(filepath.Join(out, clip), pcm); err != nil {
			return err
		}
		phones := []plan.PhoneTiming{}
		for _, phone := range p.PhoneTimings {
			if phone.Position == u.Position {
				phones = append(phones, phone)
			}
		}
		rows = append(rows, observation{UnitIndex: i, Position: u.Position, Role: u.Role, Alias: u.Alias, Source: u.Source, SourceClip: clip,
			RequestedPhones: phones, AssignedCoda: u.CodaPhones, Oto: entry, Analysis: analysis, SourceAnchors: u.SpeechSourceAnchorsMS,
			TargetAnchors: u.SpeechTargetAnchorsMS, Status: "unobserved-phone-boundaries"})
	}
	if len(rows) == 0 {
		return fmt.Errorf("no source units selected")
	}
	value := map[string]any{"version": 1, "language": p.Language, "text": p.Text, "reading": p.Reading, "voicebank": p.Voicebank,
		"phonemizer": p.Phonemizer, "source_phone_symbols": frontend.EnglishSourceSymbols(p.Phonemizer),
		"analysis_kind": "acoustic-observation-not-phone-alignment", "units": rows,
		"missing_phones": p.MissingPhones, "annotation_status": "unobserved", "reference_audio": "",
		"observed_phone_intervals": []any{}}
	data, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "observations.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("observed %d source units; phone boundaries remain unverified: %s\n", len(rows), out)
	return nil
}
