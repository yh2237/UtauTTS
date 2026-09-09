package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"utautts/internal/atomicfile"
	"utautts/internal/frontend"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

func validatePrompts(prompts []prompt) error {
	seen := map[string]bool{}
	for _, p := range prompts {
		if strings.TrimSpace(p.ID) == "" || seen[p.ID] {
			return fmt.Errorf("empty or duplicate case ID %q", p.ID)
		}
		seen[p.ID] = true
		if strings.TrimSpace(p.Text) == "" && strings.TrimSpace(p.Reading) == "" {
			return fmt.Errorf("case %s has no text or reading", p.ID)
		}
		if _, _, err := frontend.ResolveLanguage(p.Language, p.Phonemizer); err != nil {
			return fmt.Errorf("case %s: %w", p.ID, err)
		}
		for _, duration := range p.MoraDurationsMS {
			if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
				return fmt.Errorf("case %s: invalid mora duration", p.ID)
			}
		}
		if p.PitchCurve != nil {
			if p.PitchCurve.FrameMS <= 0 || math.IsNaN(p.PitchCurve.FrameMS) || math.IsInf(p.PitchCurve.FrameMS, 0) || len(p.PitchCurve.Cents) == 0 {
				return fmt.Errorf("case %s: invalid pitch curve", p.ID)
			}
			for _, cent := range p.PitchCurve.Cents {
				if math.IsNaN(cent) || math.IsInf(cent, 0) {
					return fmt.Errorf("case %s: nonfinite pitch", p.ID)
				}
			}
		}
	}
	return nil
}

type diagnostic struct {
	ID           string                  `json:"id"`
	Text         string                  `json:"text"`
	InputReading string                  `json:"input_reading,omitempty"`
	Language     string                  `json:"language"`
	Phonemizer   string                  `json:"phonemizer"`
	Reading      string                  `json:"reading"`
	Units        []frontend.Mora         `json:"units"`
	Lattice      *voicebank.LatticeAudit `json:"lattice,omitempty"`
	Coverage     *voicebank.Coverage     `json:"coverage,omitempty"`
	Stage        string                  `json:"error_stage,omitempty"`
	Error        string                  `json:"error,omitempty"`
}

func diagnoseCorpus(bankPath, out string, prompts []prompt) error {
	bank, err := voicebank.Load(bankPath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(out, 0755); err != nil {
		return err
	}
	rows := make([]diagnostic, 0, len(prompts))
	failed := false
	for _, p := range prompts {
		row := diagnostic{ID: p.ID, Text: p.Text, InputReading: p.Reading}
		row.Language, row.Phonemizer, row.Reading, row.Units, err = tts.ResolvePronunciation(tts.Config{Text: p.Text, Reading: p.Reading, Language: p.Language, Phonemizer: p.Phonemizer, Voicebank: bank})
		if err != nil {
			row.Stage = "frontend"
		} else {
			row.Coverage, err = bank.AuditCoverage(row.Units, "C4")
			if err != nil {
				return err
			}
			row.Lattice, err = bank.AuditLattice(row.Units, "C4", nil)
			if err != nil {
				row.Stage = "candidate_selection"
			}
			if err == nil && len(row.Coverage.MissingPhones) > 0 {
				row.Stage = "phoneme_coverage"
				err = fmt.Errorf("%d required phone groups are missing", len(row.Coverage.MissingPhones))
			}
		}
		if err != nil {
			row.Error = err.Error()
			failed = true
		}
		rows = append(rows, row)
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicfile.WriteFile(filepath.Join(out, "diagnostics.json"), data); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("some diagnostic cases failed; see diagnostics.json")
	}
	return nil
}
