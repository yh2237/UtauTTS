package linguistics

import (
	"fmt"
	"strings"
)

const (
	SyllablePrimaryStress   = "ˈ"
	SyllableSecondaryStress = "ˌ"
	syllableNucleusRunes    = "aeiouɛɔæɐəAIWYOʊyũõĩ"
)

// SyllablePauseSymbolは文字が音節言語の句読点休止に該当する場合に標準記号を返す。
func SyllablePauseSymbol(r rune) string {
	switch r {
	case ',', '、':
		return ","
	case ';':
		return ";"
	case ':':
		return ":"
	case '.', '。':
		return "."
	case '?', '？':
		return "?"
	case '!', '！':
		return "!"
	case '…':
		return "…"
	}
	return ""
}

// SyllablePieceは音節表記の断片と、対応するモーラ番号を表す。
// 語の間の空白と文末の記号はPartを-1にする。
type SyllablePiece struct {
	Symbols string
	Part    int
}

var syllablePauses = map[string]string{
	",": PauseKindComma, ";": PauseKindComma, ":": PauseKindComma,
	".": PauseKindPeriod, "?": PauseKindQuestion, "!": PauseKindPeriod, "…": PauseKindEllipsis,
}

// IsSyllablePauseは、音節で読む言語の読みの語が休止の記号かを返す。
func IsSyllablePause(field string) bool {
	return syllablePauses[field] != ""
}

// ParseSyllableReadingは、音節を「.」で、語を空白で区切った読みをモーラ列と音節断片へ分解する。
// 文末の記号はモーラに含めず、断片としてPart=-1で返す。
func ParseSyllableReading(reading, language string) ([]Mora, []SyllablePiece, error) {
	fields := strings.Fields(reading)
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("no syllables in %q", reading)
	}
	last := len(fields) - 1
	for last >= 0 && syllablePauses[fields[last]] != "" {
		last--
	}
	var morae []Mora
	var pieces []SyllablePiece
	word := 0
	for i, field := range fields {
		if kind := syllablePauses[field]; kind != "" {
			if i > last {
				pieces = append(pieces, SyllablePiece{Symbols: field, Part: -1})
				continue
			}
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				part := len(morae)
				morae = append(morae, Mora{Language: language, Text: field, Pause: true, PauseKind: kind})
				pieces = append(pieces, SyllablePiece{Symbols: field, Part: part})
			}
			continue
		}
		if len(pieces) > 0 {
			pieces = append(pieces, SyllablePiece{Symbols: " ", Part: -1})
		}
		syllables := strings.Split(field, ".")
		for j, syllable := range syllables {
			plain, stress := stripSyllableStress(syllable)
			if plain == "" {
				return nil, nil, fmt.Errorf("empty syllable in %q", field)
			}
			part := len(morae)
			onset, rest := splitSyllableOnset(plain)
			morae = append(morae, Mora{
				Language: language, WordIndex: word, WordEnd: j == len(syllables)-1,
				Text: plain, Consonant: onset, Vowel: rest, Stress: stress, StressKnown: true,
			})
			pieces = append(pieces, SyllablePiece{Symbols: syllable, Part: part})
		}
		if i < last {
			word++
		}
	}
	if len(morae) == 0 {
		return nil, nil, fmt.Errorf("no syllables in %q", reading)
	}
	return morae, pieces, nil
}

func stripSyllableStress(syllable string) (string, int) {
	stress := 0
	if strings.Contains(syllable, SyllablePrimaryStress) {
		stress = 1
	} else if strings.Contains(syllable, SyllableSecondaryStress) {
		stress = 2
	}
	return strings.NewReplacer(SyllablePrimaryStress, "", SyllableSecondaryStress, "").Replace(syllable), stress
}

func splitSyllableOnset(syllable string) (string, string) {
	for i, r := range syllable {
		if strings.ContainsRune(syllableNucleusRunes, r) {
			return syllable[:i], syllable[i:]
		}
	}
	return syllable, ""
}
