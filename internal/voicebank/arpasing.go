package voicebank

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type arpasingDictionary struct {
	Entries []struct {
		Grapheme string   `yaml:"grapheme"`
		Phonemes []string `yaml:"phonemes"`
	} `yaml:"entries"`
}

func (b *Bank) loadARPAsing() {
	dictionary, path, err := LoadARPAsingDictionary(b.Root)
	if path == "" {
		return
	}
	if err != nil {
		b.Diagnostics = append(b.Diagnostics, Diagnostic{Path: path, Message: err.Error()})
		return
	}
	b.ARPAsing = dictionary
}

func LoadARPAsingDictionary(root string) (map[string]string, string, error) {
	path := findRootFile(root, "arpasing.yaml")
	if path == "" {
		return nil, "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	var source arpasingDictionary
	if err := yaml.Unmarshal(data, &source); err != nil {
		return nil, path, err
	}
	result := make(map[string]string, len(source.Entries))
	for _, entry := range source.Entries {
		grapheme := strings.ToLower(strings.TrimSpace(entry.Grapheme))
		if grapheme != "" && len(entry.Phonemes) > 0 {
			result[grapheme] = strings.Join(entry.Phonemes, " ")
		}
	}
	return result, path, nil
}
