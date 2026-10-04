package main

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
)

// reanalyzeRecords uses the same native Open JTalk bridge as internal/openjtalk.
// Timings and aligned vowels remain from the corpus, as in the Python trainer.
func reanalyzeRecords(rows []record, cfg openjtalk.Config) ([]record, error) {
	out := make([]record, 0, len(rows))
	for _, row := range rows {
		a, err := openjtalk.Analyze(row.Text, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", row.ID, err)
		}
		if len(a.Morae) != len(row.Tokens) {
			continue
		}
		aligned := true
		for i, mora := range a.Morae {
			if mora != row.Tokens[i].Mora || (len(a.Features[i]) == 0) != row.Tokens[i].Pause {
				aligned = false
				break
			}
		}
		if !aligned {
			continue
		}
		applyAnalysis(&row, a)
		out = append(out, row)
	}
	return out, nil
}

func applyAnalysis(row *record, a *openjtalk.Analysis) {
	for i, f := range a.Features {
		t := &row.Tokens[i]
		if t.Pause {
			continue
		}
		if f["accent_phrase_start"] == 1 {
			t.AccentStart = true
		} else {
			t.AccentStart = false
		}
		t.AccentEnd = f["accent_phrase_end"] == 1
		t.AccentHigh = f["accent_high"] == 1
		t.WordStart = f["word_start"] == 1
		t.WordEnd = f["word_end"] == 1
		for name := range f {
			if strings.HasPrefix(name, "pos=") {
				t.POS = strings.TrimPrefix(name, "pos=")
			}
			if strings.HasPrefix(name, "pos_group1=") {
				t.POSGroup = strings.TrimPrefix(name, "pos_group1=")
			}
		}
	}
	// Recover integer phrase metadata from the helper's exact sparse ratios.
	start := 0
	for start < len(row.Tokens) {
		if row.Tokens[start].Pause {
			start++
			continue
		}
		end := start + 1
		for end < len(row.Tokens) && !row.Tokens[end].Pause && !row.Tokens[end-1].AccentEnd {
			end++
		}
		length := end - start
		for i := start; i < end; i++ {
			f := a.Features[i]
			row.Tokens[i].AccentPosition = int(math.Round(f["accent_position"] * float64(length)))
			row.Tokens[i].AccentLength = length
			row.Tokens[i].AccentNucleus = int(math.Round(f["accent_nucleus_position"] * float64(length)))
		}
		start = end
	}
	morae, err := frontend.ParseKana(a.Reading)
	if err == nil && len(morae) == len(row.Tokens) {
		for i := range row.Tokens {
			if row.Tokens[i].Vowel == "" {
				row.Tokens[i].Vowel = morae[i].Vowel
			}
		}
	}
}

func reanalyzeSplits(train, valid, test []record, c config) ([]record, []record, []record, error) {
	cfg := openjtalk.Config{HelperPath: c.OpenJTalkHelper, DictionaryPath: c.OpenJTalkDictionary}
	initial := len(train) + len(valid)
	var err error
	train, err = reanalyzeRecords(train, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	valid, err = reanalyzeRecords(valid, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	test, err = reanalyzeRecords(test, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if float64(len(train)+len(valid))/float64(max(1, initial)) < .6 {
		return nil, nil, nil, fmt.Errorf("alignment rate below 0.600")
	}
	return train, valid, test, nil
}
