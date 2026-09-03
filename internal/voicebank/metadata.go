package voicebank

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"utautts/internal/oto"
)

type Affix struct {
	Prefix string
	Suffix string
}

func (b *Bank) loadMetadata() {
	if subbanks, path, diagnostics := loadCharacterYAML(b.Root); path != "" {
		b.CharacterYAML = path
		b.Subbanks = subbanks
		b.Diagnostics = append(b.Diagnostics, diagnostics...)
		if text, err := readMetadata(path); err == nil {
			for _, line := range strings.Split(text, "\n") {
				key, value, ok := splitYAMLField(strings.TrimSpace(line))
				if ok && strings.EqualFold(key, "default_phonemizer") {
					b.DefaultPhonemizer = parseYAMLScalar(value)
					break
				}
			}
		}
	}
	if path := findRootFile(b.Root, "character.txt"); path != "" {
		if text, err := readMetadata(path); err == nil {
			for _, line := range strings.Split(text, "\n") {
				parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "name") {
					if name := strings.TrimSpace(parts[1]); name != "" {
						b.Name = name
					}
				}
			}
		}
	}
	path := findRootFile(b.Root, "prefix.map")
	if path == "" {
		return
	}
	text, err := readMetadata(path)
	if err != nil {
		b.Diagnostics = append(b.Diagnostics, Diagnostic{Path: path, Message: err.Error()})
		return
	}
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			b.Diagnostics = append(b.Diagnostics, Diagnostic{Path: path, Line: index + 1, Message: "invalid prefix.map line"})
			continue
		}
		tone := strings.ToUpper(strings.TrimSpace(fields[0]))
		affix := Affix{Prefix: fields[1]}
		if len(fields) >= 3 {
			affix.Suffix = fields[2]
		}
		b.PrefixMap[tone] = affix
	}
}

func readMetadata(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text, _, err := oto.Decode(data)
	return text, err
}

func findRootFile(root, name string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), name) {
			return filepath.Join(root, entry.Name())
		}
	}
	return ""
}

func (b *Bank) AffixForTone(tone string) (Affix, bool) {
	affix, _, ok := b.AffixForToneAndColor(tone, "")
	return affix, ok
}

func (b *Bank) AffixForToneAndColor(tone, color string) (Affix, Subbank, bool) {
	tone = strings.ToUpper(strings.TrimSpace(tone))
	if tone == "" {
		tone = "C4"
	}
	if len(b.Subbanks) > 0 {
		if subbank, ok := selectSubbank(b.Subbanks, tone, color); ok {
			if subbank.Tone == "" {
				subbank.Tone = tone
			}
			return Affix{Prefix: subbank.Prefix, Suffix: subbank.Suffix}, subbank, true
		}
		if strings.TrimSpace(color) != "" {
			return Affix{}, Subbank{}, false
		}
	}
	if len(b.PrefixMap) == 0 {
		return Affix{}, Subbank{}, false
	}
	if affix, ok := b.PrefixMap[tone]; ok {
		return affix, Subbank{ID: "prefix.map", Prefix: affix.Prefix, Suffix: affix.Suffix, Tone: tone}, true
	}
	target, ok := toneNumber(tone)
	if !ok {
		return Affix{}, Subbank{}, false
	}
	bestDistance := int(^uint(0) >> 1)
	bestTone := ""
	var best Affix
	found := false
	for candidate, affix := range b.PrefixMap {
		number, valid := toneNumber(candidate)
		if !valid {
			continue
		}
		distance := number - target
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance || (distance == bestDistance && (bestTone == "" || candidate < bestTone)) {
			bestDistance = distance
			bestTone = candidate
			best = affix
			found = true
		}
	}
	return best, Subbank{ID: "prefix.map", Prefix: best.Prefix, Suffix: best.Suffix, Tone: bestTone}, found
}

func selectSubbank(subbanks []Subbank, tone, color string) (Subbank, bool) {
	color = strings.TrimSpace(color)
	var best Subbank
	found := false
	bestDistance := int(^uint(0) >> 1)
	target, targetOK := toneNumber(tone)
	for _, candidate := range subbanks {
		if strings.TrimSpace(candidate.Color) != color {
			continue
		}
		distance := 0
		if targetOK {
			distance = subbankToneDistance(candidate, target)
		}
		if !found || distance < bestDistance || (distance == bestDistance && candidate.Order < best.Order) {
			best, bestDistance, found = candidate, distance, true
			if targetOK {
				best.Tone = toneAtSubbankDistance(candidate, target)
			}
		}
	}
	return best, found
}

func toneAtSubbankDistance(subbank Subbank, target int) string {
	if len(subbank.ToneRanges) == 0 {
		return toneName(target)
	}
	best := target
	bestDistance := int(^uint(0) >> 1)
	for _, toneRange := range subbank.ToneRanges {
		value := target
		if value < toneRange.Low {
			value = toneRange.Low
		} else if value > toneRange.High {
			value = toneRange.High
		}
		distance := value - target
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best, bestDistance = value, distance
		}
	}
	return toneName(best)
}

func toneName(number int) string {
	if number < 0 {
		return ""
	}
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	octave := number/12 - 1
	return names[number%12] + strconv.Itoa(octave)
}

func subbankToneDistance(subbank Subbank, target int) int {
	if len(subbank.ToneRanges) == 0 {
		return 0
	}
	best := int(^uint(0) >> 1)
	for _, toneRange := range subbank.ToneRanges {
		distance := 0
		if target < toneRange.Low {
			distance = toneRange.Low - target
		} else if target > toneRange.High {
			distance = target - toneRange.High
		}
		if distance < best {
			best = distance
		}
	}
	return best
}

func toneNumber(tone string) (int, bool) {
	tone = strings.ToUpper(strings.TrimSpace(tone))
	if len(tone) < 2 {
		return 0, false
	}
	semitones := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
	semitone, ok := semitones[tone[0]]
	if !ok {
		return 0, false
	}
	index := 1
	if index < len(tone) && (tone[index] == '#' || tone[index] == 'B') {
		if tone[index] == '#' {
			semitone++
		} else {
			semitone--
		}
		index++
	}
	octave, err := strconv.Atoi(tone[index:])
	if err != nil {
		return 0, false
	}
	return (octave+1)*12 + semitone, true
}
