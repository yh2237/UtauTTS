package zh

import (
	"context"
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/g2p"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

type analyzer struct{}

func init() {
	g2p.Register(analyzer{})
}

func (analyzer) Language() string          { return linguistics.LanguageChinese }
func (analyzer) DefaultPhonemizer() string { return linguistics.PhonemizerChinese }
func (analyzer) Phonemizers() []string     { return []string{linguistics.PhonemizerChinese} }
func (analyzer) Normalize(text string) string {
	return Normalize(text)
}

func (analyzer) Analyze(_ context.Context, input g2p.Input) (linguistics.Utterance, error) {
	phonemizer := strings.ToLower(strings.TrimSpace(input.Phonemizer))
	if phonemizer == "" {
		phonemizer = linguistics.PhonemizerChinese
	}
	if phonemizer != linguistics.PhonemizerChinese {
		return linguistics.Utterance{}, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, linguistics.LanguageChinese)
	}
	utterance, err := Parse(input.Text, input.Reading, input.Dictionary)
	if err != nil {
		return linguistics.Utterance{}, err
	}
	utterance.Phonemizer = phonemizer
	return utterance, nil
}

// Parseは中国語の文またはPinyin読みから共通中間表現を組み立てる。
func Parse(text, reading string, dictionary map[string]string) (linguistics.Utterance, error) {
	var syllables []string
	var tokens []chineseToken
	if strings.TrimSpace(reading) != "" {
		syllables = strings.Fields(reading)
	} else if value := dictionary[text]; value != "" {
		syllables = strings.Fields(value)
	} else {
		var err error
		tokens, err = chineseReadingTokens(text, dictionary)
		if err != nil {
			return linguistics.Utterance{}, err
		}
		for _, token := range tokens {
			syllables = append(syllables, token.reading)
		}
	}

	// プレビューの読みと一致する場合だけ再解析し、手動の読みは保つ。
	if len(tokens) == 0 && text != "" {
		if inferred, err := chineseReadingTokens(text, dictionary); err == nil {
			var values []string
			for _, token := range inferred {
				values = append(values, token.reading)
			}
			if strings.Join(values, " ") == strings.Join(syllables, " ") {
				tokens = inferred
			}
		}
	}

	var morae []linguistics.Mora
	for syllableIndex, raw := range syllables {
		if raw == "|" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				morae = append(morae, linguistics.Mora{Pause: true})
			}
			continue
		}
		tone := PinyinTone(raw)
		syllable := NormalizePinyin(raw)
		initial, final := SplitPinyin(syllable)
		if final == "" {
			return linguistics.Utterance{}, fmt.Errorf("invalid Pinyin syllable %q", raw)
		}
		mora := linguistics.Mora{
			Language:  linguistics.LanguageChinese,
			WordIndex: syllableIndex,
			WordEnd:   true,
			Text:      syllable,
			Consonant: initial,
			Vowel:     final,
			Tone:      tone,
		}
		if len(tokens) == len(syllables) {
			mora.SourceText = tokens[syllableIndex].source
			mora.WordIndex = tokens[syllableIndex].word
			mora.WordEnd = tokens[syllableIndex].end
		}
		phoneInitial, phoneFinal := SplitPinyin(syllable)
		if expanded := map[string]string{"iu": "iou", "ui": "uei", "un": "uen"}[phoneFinal]; expanded != "" {
			phoneFinal = expanded
		}
		if phoneFinal == "uen" && (phoneInitial == "j" || phoneInitial == "q" || phoneInitial == "x" || phoneInitial == "y") {
			phoneFinal = "vn"
		}
		if phoneFinal == "ue" && (phoneInitial == "j" || phoneInitial == "q" || phoneInitial == "x" || phoneInitial == "y" || syllable == "nue" || syllable == "lue") {
			phoneFinal = "ve"
		}
		if phoneInitial != "" {
			mora.Phones = append(mora.Phones, linguistics.Phone{Symbol: phoneInitial, Role: "onset"})
		}
		nucleus, coda := phoneFinal, ""
		if strings.HasSuffix(phoneFinal, "ng") {
			nucleus, coda = strings.TrimSuffix(phoneFinal, "ng"), "ng"
		} else if strings.HasSuffix(phoneFinal, "n") {
			nucleus, coda = strings.TrimSuffix(phoneFinal, "n"), "n"
		}
		if nucleus == "" {
			nucleus, coda = phoneFinal, ""
		}
		mora.Phones = append(mora.Phones, rhymeParts(nucleus)...)
		if coda != "" {
			mora.Phones = append(mora.Phones, linguistics.Phone{Symbol: coda, Role: "coda"})
		}
		morae = append(morae, mora)
	}
	if len(morae) == 0 {
		return linguistics.Utterance{}, fmt.Errorf("Pinyin reading is empty")
	}
	joined := strings.Join(syllables, " ")
	return linguistics.Utterance{
		Language:       linguistics.LanguageChinese,
		Phonemizer:     linguistics.PhonemizerChinese,
		Text:           text,
		Reading:        joined,
		SurfaceReading: SurfaceReading(joined, morae),
		Morae:          morae,
	}, nil
}

func rhymeParts(final string) []linguistics.Phone {
	parts := map[string][]linguistics.Phone{
		"ai":  {{Symbol: "a", Role: "nucleus"}, {Symbol: "i", Role: "offglide"}},
		"ei":  {{Symbol: "e", Role: "nucleus"}, {Symbol: "i", Role: "offglide"}},
		"ao":  {{Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		"ou":  {{Symbol: "o", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		"ia":  {{Symbol: "i", Role: "medial"}, {Symbol: "a", Role: "nucleus"}},
		"ie":  {{Symbol: "i", Role: "medial"}, {Symbol: "e", Role: "nucleus"}},
		"ua":  {{Symbol: "u", Role: "medial"}, {Symbol: "a", Role: "nucleus"}},
		"uo":  {{Symbol: "u", Role: "medial"}, {Symbol: "o", Role: "nucleus"}},
		"ve":  {{Symbol: "v", Role: "medial"}, {Symbol: "e", Role: "nucleus"}},
		"ue":  {{Symbol: "u", Role: "medial"}, {Symbol: "e", Role: "nucleus"}},
		"iao": {{Symbol: "i", Role: "medial"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		"iou": {{Symbol: "i", Role: "medial"}, {Symbol: "o", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		"uai": {{Symbol: "u", Role: "medial"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "i", Role: "offglide"}},
		"uei": {{Symbol: "u", Role: "medial"}, {Symbol: "e", Role: "nucleus"}, {Symbol: "i", Role: "offglide"}},
	}
	if p := parts[final]; p != nil {
		return append([]linguistics.Phone(nil), p...)
	}
	return []linguistics.Phone{{Symbol: final, Role: "nucleus"}}
}

// NormalizePinyinは声調数字を取り除きüをvへ統一した小文字ピンインを返す。
func NormalizePinyin(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimRight(value, "12345")
	value = strings.ReplaceAll(value, "u:", "v")
	value = strings.ReplaceAll(value, "ü", "v")
	return value
}

// PinyinToneはピンイン末尾の声調（1〜5）を返す。指定がなければ0を返す。
func PinyinTone(value string) int {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	last := value[len(value)-1]
	if last >= '1' && last <= '5' {
		return int(last - '0')
	}
	return 0
}

// SplitPinyinは正規化済みピンイン音節を声母と韻母へ分ける。
func SplitPinyin(syllable string) (string, string) {
	initials := []string{"zh", "ch", "sh", "b", "p", "m", "f", "d", "t", "n", "l", "g", "k", "h", "j", "q", "x", "r", "z", "c", "s", "y", "w"}
	for _, initial := range initials {
		if strings.HasPrefix(syllable, initial) && len(syllable) > len(initial) {
			return initial, syllable[len(initial):]
		}
	}
	return "", syllable
}
