package voicebank

import (
	"fmt"
	"os"
	"strings"

	"github.com/yh2237/utauio/character"
)

type ToneRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

type Subbank struct {
	ID         string
	Color      string
	Prefix     string
	Suffix     string
	ToneRanges []ToneRange
	Source     string
	Order      int
	Tone       string
}

type SubbankOption struct {
	ID         string      `json:"id"`
	Color      string      `json:"color"`
	Prefix     string      `json:"prefix,omitempty"`
	Suffix     string      `json:"suffix,omitempty"`
	ToneRanges []ToneRange `json:"tone_ranges,omitempty"`
}

func (b *Bank) SubbankOptions() []SubbankOption {
	if b == nil || len(b.Subbanks) == 0 {
		return nil
	}
	options := make([]SubbankOption, 0, len(b.Subbanks))
	for index, subbank := range b.Subbanks {
		id := strings.TrimSpace(subbank.ID)
		if id == "" {
			id = fmt.Sprintf("subbank-%d", index)
		}
		ranges := append([]ToneRange(nil), subbank.ToneRanges...)
		options = append(options, SubbankOption{
			ID:         id,
			Color:      subbank.Color,
			Prefix:     subbank.Prefix,
			Suffix:     subbank.Suffix,
			ToneRanges: ranges,
		})
	}
	return options
}

func loadCharacterYAMLMetadata(root string, rootEntries ...[]os.DirEntry) ([]Subbank, string, []Diagnostic, string) {
	path := findRootFile(root, "character.yaml", rootEntries...)
	if path == "" {
		path = findRootFile(root, "character.yml", rootEntries...)
	}
	if path == "" {
		return nil, "", nil, ""
	}
	text, err := readMetadata(path)
	if err != nil {
		return nil, path, []Diagnostic{{Path: path, Message: fmt.Sprintf("read character.yaml: %v", err)}}, ""
	}
	config, parsedDiagnostics := character.ParseYAML(text)
	var subbanks []Subbank
	for index, parsed := range config.Subbanks {
		ranges := make([]ToneRange, len(parsed.ToneRanges))
		for i, span := range parsed.ToneRanges {
			ranges[i] = ToneRange{Low: span.Low, High: span.High}
		}
		subbanks = append(subbanks, Subbank{
			ID: fmt.Sprintf("subbank-%d", index), Source: path, Order: index,
			Color: parsed.Color, Prefix: parsed.Prefix, Suffix: parsed.Suffix, ToneRanges: ranges,
		})
	}
	var diagnostics []Diagnostic
	for _, parsed := range parsedDiagnostics {
		diagnostics = append(diagnostics, Diagnostic{Path: path, Line: parsed.Line, Message: parsed.Message})
	}
	return subbanks, path, diagnostics, config.DefaultPhonemizer
}

func (r ToneRange) Contains(tone string) bool {
	number, ok := toneNumber(tone)
	return ok && number >= r.Low && number <= r.High
}

func (s Subbank) ContainsTone(tone string) bool {
	if len(s.ToneRanges) == 0 {
		return true
	}
	for _, toneRange := range s.ToneRanges {
		if toneRange.Contains(tone) {
			return true
		}
	}
	return false
}
