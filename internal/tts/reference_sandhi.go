package tts

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// chineseSurfaceReadingは、ピンインの読みへ連続変調（三声変調、不・一の変調）を反映する。
// 参照TTSは変調済みの音素で学習されているため、辞書の声調のまま渡すと不自然になる。
// 読みとモーラの数が合わないときは、変調せずにそのまま返す。
func chineseSurfaceReading(reading string, morae []frontend.Mora) string {
	fields := strings.Fields(reading)
	if len(fields) != len(morae) {
		return reading
	}
	tones := mandarinSurfaceTones(morae)
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

// withPinyinToneは、ピンインの末尾の声調の数字を置き換える。数字がなければ足す。
func withPinyinTone(syllable string, tone int) string {
	if syllable == "" {
		return syllable
	}
	if last := syllable[len(syllable)-1]; last >= '1' && last <= '5' {
		syllable = syllable[:len(syllable)-1]
	}
	return syllable + string(rune('0'+tone))
}
