package voicebank

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ToneRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

// Subbankは原音選択に必要なcharacter.yamlの情報を保持する。
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

// SubbankOptionsは選択可能なサブバンクのコピーを宣言順で返す。
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

func loadCharacterYAML(root string) ([]Subbank, string, []Diagnostic) {
	subbanks, path, diagnostics, _ := loadCharacterYAMLData(root)
	return subbanks, path, diagnostics
}

func loadCharacterYAMLData(root string, rootEntries ...[]os.DirEntry) ([]Subbank, string, []Diagnostic, string) {
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

	var (
		subbanks []Subbank
		current  *Subbank
		inList   bool
		inRanges bool
		diags    []Diagnostic
	)
	finish := func() {
		if current == nil {
			return
		}
		if len(current.ToneRanges) == 0 {
			// 音域未指定は全音域として扱う。
			current.ToneRanges = []ToneRange{{Low: -1 << 30, High: 1<<30 - 1}}
		}
		subbanks = append(subbanks, *current)
		current = nil
	}

	for lineNumber, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(stripYAMLComment(strings.TrimSuffix(raw, "\r")))
		if line == "" {
			continue
		}
		if line == "subbanks:" {
			finish()
			inList, inRanges = true, false
			continue
		}
		if !inList {
			continue
		}

		if strings.HasPrefix(line, "-") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if strings.HasPrefix(rest, "color:") {
				finish()
				current = &Subbank{ID: fmt.Sprintf("subbank-%d", len(subbanks)), Source: path, Order: len(subbanks)}
				inRanges = false
				if err := assignSubbankField(current, rest); err != nil {
					diags = append(diags, Diagnostic{Path: path, Line: lineNumber + 1, Message: err.Error()})
				}
				continue
			}
			if current != nil && inRanges {
				rangeValue := parseYAMLScalar(rest)
				toneRange, err := parseToneRange(rangeValue)
				if err != nil {
					diags = append(diags, Diagnostic{Path: path, Line: lineNumber + 1, Message: err.Error()})
				} else {
					current.ToneRanges = append(current.ToneRanges, toneRange)
				}
				continue
			}
		}
		if current == nil {
			continue
		}
		key, value, ok := splitYAMLField(line)
		if !ok {
			continue
		}
		if key == "tone_ranges" {
			inRanges = true
			for _, item := range parseInlineYAMLList(value) {
				toneRange, err := parseToneRange(item)
				if err != nil {
					diags = append(diags, Diagnostic{Path: path, Line: lineNumber + 1, Message: err.Error()})
				} else {
					current.ToneRanges = append(current.ToneRanges, toneRange)
				}
			}
			continue
		}
		if err := assignSubbankField(current, line); err != nil {
			diags = append(diags, Diagnostic{Path: path, Line: lineNumber + 1, Message: err.Error()})
		}
	}
	finish()
	return subbanks, path, diags, text
}

func assignSubbankField(subbank *Subbank, line string) error {
	key, value, ok := splitYAMLField(line)
	if !ok {
		return nil
	}
	value = parseYAMLScalar(value)
	switch key {
	case "color":
		subbank.Color = value
	case "prefix":
		subbank.Prefix = value
	case "suffix":
		subbank.Suffix = value
	case "tone_ranges":
	default:
		// 未知フィールドは互換性のため無視する。
	}
	return nil
}

func splitYAMLField(line string) (key, value string, ok bool) {
	index := strings.IndexByte(line, ':')
	if index < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:index])
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(line[index+1:]), true
}

func parseYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}

func parseInlineYAMLList(value string) []string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil
	}
	value = value[1 : len(value)-1]
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = parseYAMLScalar(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseToneRange(value string) (ToneRange, error) {
	value = parseYAMLScalar(value)
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return ToneRange{}, fmt.Errorf("invalid tone range %q", value)
	}
	low, lowOK := toneNumber(parts[0])
	high, highOK := toneNumber(parts[1])
	if !lowOK || !highOK || low > high {
		return ToneRange{}, fmt.Errorf("invalid tone range %q", value)
	}
	return ToneRange{Low: low, High: high}, nil
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

func stripYAMLComment(value string) string {
	quoted := byte(0)
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '\'', '"':
			if quoted == 0 {
				quoted = value[index]
			} else if quoted == value[index] {
				quoted = 0
			}
		case '#':
			if quoted == 0 && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
				return value[:index]
			}
		}
	}
	return value
}
