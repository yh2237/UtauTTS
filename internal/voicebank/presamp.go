package voicebank

import (
	"sort"
	"strconv"
	"strings"

	"utautts/internal/frontend"
)

type Presamp struct {
	Path         string
	Vowels       map[string]string
	Consonants   map[string]string
	Replacements map[string]string
	Endings      []string
}

func (p *Presamp) FrontendConfig() frontend.PresampConfig {
	if p == nil {
		return frontend.PresampConfig{}
	}
	return frontend.PresampConfig{
		Vowels: p.Vowels, Consonants: p.Consonants,
		Replacements: p.Replacements, Endings: p.Endings,
	}
}

func (b *Bank) loadPresamp() {
	path := findRootFile(b.Root, "presamp.ini")
	if path == "" {
		return
	}
	text, err := readMetadata(path)
	if err != nil {
		b.Diagnostics = append(b.Diagnostics, Diagnostic{Path: path, Message: err.Error()})
		return
	}
	presamp := &Presamp{
		Path: path, Vowels: map[string]string{}, Consonants: map[string]string{},
		Replacements: map[string]string{},
	}
	section := ""
	endingTypes := map[int][]string{}
	endingFlag := 0
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToUpper(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		switch {
		case section == "VOWEL":
			parsePresampClass(line, 2, presamp.Vowels)
		case section == "CONSONANT":
			parsePresampClass(line, 1, presamp.Consonants)
		case section == "REPLACE":
			key, value, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				presamp.Replacements[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		case strings.HasPrefix(section, "ENDTYPE"):
			endingTypes[presampEndingType(section)] = append(endingTypes[presampEndingType(section)], line)
		case section == "ENDFLAG":
			endingFlag, _ = strconv.Atoi(line)
		}
	}
	indices := make([]int, 0, len(endingTypes))
	for index := range endingTypes {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		if endingFlag == 0 || endingFlag&(1<<(index-1)) != 0 {
			presamp.Endings = append(presamp.Endings, endingTypes[index]...)
		}
	}
	b.Presamp = presamp
}

func presampEndingType(section string) int {
	suffix := strings.TrimPrefix(section, "ENDTYPE")
	if index, err := strconv.Atoi(suffix); err == nil && index > 0 {
		return index
	}
	return 1
}

func parsePresampClass(line string, aliasesIndex int, destination map[string]string) {
	parts := strings.Split(line, "=")
	if len(parts) <= aliasesIndex {
		return
	}
	class := strings.TrimSpace(parts[0])
	if class == "" {
		return
	}
	for _, alias := range strings.Split(parts[aliasesIndex], ",") {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			destination[alias] = class
		}
	}
}
