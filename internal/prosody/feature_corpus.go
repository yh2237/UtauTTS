package prosody

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FeatureCase struct {
	ID       string         `json:"id"`
	Text     string         `json:"text,omitempty"`
	Reading  string         `json:"reading,omitempty"`
	Features []FeatureFrame `json:"features"`
}

type FeatureCorpus struct {
	Version int           `json:"version"`
	Name    string        `json:"name,omitempty"`
	Cases   []FeatureCase `json:"cases"`
}

func LoadFeatureCorpus(path string) (*FeatureCorpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var corpus FeatureCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, err
	}
	if corpus.Version != 1 {
		return nil, fmt.Errorf("unsupported prosody feature file version %d", corpus.Version)
	}
	seen := make(map[string]bool, len(corpus.Cases))
	for index, item := range corpus.Cases {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			return nil, fmt.Errorf("prosody feature case %d has an empty id", index)
		}
		if seen[item.ID] {
			return nil, fmt.Errorf("duplicate prosody feature case %q", item.ID)
		}
		if len(item.Features) == 0 {
			return nil, fmt.Errorf("prosody feature case %q has no frames", item.ID)
		}
		seen[item.ID] = true
		corpus.Cases[index] = item
	}
	if len(corpus.Cases) == 0 {
		return nil, fmt.Errorf("prosody feature corpus has no cases")
	}
	return &corpus, nil
}

func (c *FeatureCorpus) Select(caseID, text, reading string) (*FeatureCase, error) {
	if c == nil {
		return nil, fmt.Errorf("prosody feature corpus is nil")
	}
	caseID = strings.TrimSpace(caseID)
	if caseID != "" {
		for index := range c.Cases {
			if c.Cases[index].ID == caseID {
				return &c.Cases[index], nil
			}
		}
		return nil, fmt.Errorf("prosody feature case %q not found", caseID)
	}
	wantText := normalizeFeatureUtterance(text)
	wantReading := normalizeFeatureUtterance(reading)
	var match *FeatureCase
	for index := range c.Cases {
		item := &c.Cases[index]
		matched := wantText != "" && normalizeFeatureUtterance(item.Text) == wantText
		matched = matched || (wantReading != "" && normalizeFeatureUtterance(item.Reading) == wantReading)
		if !matched {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("multiple prosody feature cases match the utterance; select a case explicitly")
		}
		match = item
	}
	if match != nil {
		return match, nil
	}
	if len(c.Cases) == 1 {
		return &c.Cases[0], nil
	}
	return nil, fmt.Errorf("no prosody feature case matches the text; select the corresponding case explicitly")
}

func normalizeFeatureUtterance(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), "")
}
