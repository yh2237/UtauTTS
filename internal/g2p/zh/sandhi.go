package zh

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// SurfaceReadingは、ピンインの読みへ連続変調（三声変調、不・一の変調）を反映する。
// 読みとモーラの数が合わないときは、変調せずにそのまま返す。
func SurfaceReading(reading string, morae []linguistics.Mora) string {
	fields := strings.Fields(reading)
	if len(fields) != len(morae) {
		return reading
	}
	tones := SurfaceTones(morae)
	result := make([]string, len(fields))
	for i, field := range fields {
		if field == "|" || tones[i] <= 0 {
			result[i] = field
			continue
		}
		result[i] = withPinyinTone(field, tones[i])
	}
	return strings.Join(result, " ")
}

func withPinyinTone(syllable string, tone int) string {
	if syllable == "" {
		return syllable
	}
	if last := syllable[len(syllable)-1]; last >= '1' && last <= '5' {
		syllable = syllable[:len(syllable)-1]
	}
	return syllable + string(rune('0'+tone))
}

// SurfaceTonesは語彙上の声調を残したまま連続変調後の声調列を求める。
func SurfaceTones(morae []linguistics.Mora) []int {
	tones := make([]int, len(morae))
	for i, m := range morae {
		tones[i] = m.Tone
	}
	for i, m := range morae {
		if m.Pause || i+1 == len(morae) || morae[i+1].Pause {
			continue
		}
		next := morae[i+1].Tone
		if m.SourceText == "不" && m.Tone == 4 && next == 4 {
			tones[i] = 2
		}
		if m.SourceText == "一" && m.Tone == 1 && !(i > 0 && morae[i-1].SourceText == "第") {
			// 数列と年号ではyi1を保つ。
			if i+1 < len(morae) && strings.Contains("零一二三四五六七八九十", morae[i+1].SourceText) && morae[i+1].SourceText != "" {
				continue
			}
			if next == 4 {
				tones[i] = 2
			} else if next >= 1 && next <= 3 {
				tones[i] = 4
			}
		}
	}
	for i := 0; i+1 < len(morae); i++ {
		if !morae[i].Pause && !morae[i+1].Pause && !morae[i].WordEnd && morae[i].WordIndex == morae[i+1].WordIndex && tones[i] == 3 && tones[i+1] == 3 {
			tones[i] = 2
		}
	}
	for i := len(morae) - 2; i >= 0; i-- {
		if !morae[i].Pause && !morae[i+1].Pause && tones[i] == 3 && tones[i+1] == 3 {
			tones[i] = 2
		}
	}
	return tones
}
