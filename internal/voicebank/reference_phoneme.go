package voicebank

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
	"golang.org/x/text/unicode/norm"
)

// ReferencePhonemeはエイリアスを参照索引用の音素ラベルへ直す。
// ラベルは録音単位なので、CVは子音と母音をつないだ形（kaなど）で表す。
// 対応: あ/ア/a/- あ/a あ→a、か/カ/ka/- か/a か→ka。
// 他の仮名もfrontend.ParseKanaの子音+母音へ直す（し→shi、ち→chi、つ→tsu、ふ→fu、じ/ぢ→ji）。
// 子音が未定義の仮名は平仮名で保持する（ウィ/うぃ→うぃ）。
// ん/ン/n→n、っ/ッ/cl→cl。VCの「a k」はkへ直す。
// 文脈の母音・語頭記号は除き、判別できない別名は空にする。
func (b *Bank) ReferencePhoneme(alias string) string {
	alias = strings.TrimSpace(alias)
	best := Affix{}
	for _, subbank := range b.Subbanks {
		candidate := Affix{Prefix: subbank.Prefix, Suffix: subbank.Suffix}
		if len(candidate.Prefix)+len(candidate.Suffix) > len(best.Prefix)+len(best.Suffix) && strings.HasPrefix(alias, candidate.Prefix) && strings.HasSuffix(alias, candidate.Suffix) {
			best = candidate
		}
	}
	for _, candidate := range b.PrefixMap {
		if len(candidate.Prefix)+len(candidate.Suffix) > len(best.Prefix)+len(best.Suffix) && strings.HasPrefix(alias, candidate.Prefix) && strings.HasSuffix(alias, candidate.Suffix) {
			best = candidate
		}
	}
	alias = strings.TrimSuffix(strings.TrimPrefix(alias, best.Prefix), best.Suffix)
	if base, _, ok := splitToneSuffix(alias); ok {
		alias = base
	}
	parts := strings.Fields(alias)
	if len(parts) == 2 && (parts[0] == "-" || parts[0] == "*" || isVowelContext(parts[0])) {
		alias = parts[1]
	} else if len(parts) != 1 {
		return ""
	}
	if containsKana(alias) {
		alias = strings.Map(func(r rune) rune {
			if r >= 'ァ' && r <= 'ヶ' {
				return r - 0x60
			}
			return r
		}, norm.NFC.String(alias))
		morae, err := frontend.ParseKana(alias)
		if err != nil || len(morae) != 1 || morae[0].Pause {
			return ""
		}
		mora := morae[0]
		if mora.Text != alias {
			return ""
		}
		if mora.Consonant == "" && (len([]rune(mora.Text)) != 1 || !strings.Contains("あいうえおぁぃぅぇぉゐゑ", mora.Text)) {
			// frontendに子音の定義がない拗音・外来音は平仮名で残し、別の母音へ混ぜない。
			return mora.Text
		}
		if mora.Vowel == "n" || mora.Vowel == "cl" {
			return mora.Vowel
		}
		return mora.Consonant + mora.Vowel
	}
	alias = strings.ToLower(alias)
	// ローマ字は同じ子音集合へ寄せる。未知の語や装飾付き別名は推測しない。
	switch alias {
	case "si":
		return "shi"
	case "ti":
		return "chi"
	case "tu":
		return "tsu"
	case "hu":
		return "fu"
	case "zi", "di":
		return "ji"
	case "du":
		return "zu"
	}
	if alias == "cl" || alias == "n" || isConsonantContext(alias) {
		return alias
	}
	for _, vowel := range []string{"a", "i", "u", "e", "o"} {
		if alias == vowel {
			return alias
		}
		for _, consonant := range []string{"k", "g", "s", "sh", "z", "j", "t", "ch", "ts", "d", "n", "ny", "h", "hy", "f", "b", "by", "p", "py", "m", "my", "y", "r", "ry", "w", "v", "vy", "ky", "gy"} {
			if alias == consonant+vowel {
				return alias
			}
		}
	}
	return ""
}

// ReferenceRecordingPhonemeは同じ録音の別名が同じ音素を指す場合だけラベルを返す。
// 複数の音素を含む連続音の録音を、単一の音素と誤認して検索を絞らない。
func (b *Bank) ReferenceRecordingPhoneme(entries []oto.Entry) string {
	phoneme := ""
	for _, entry := range entries {
		current := b.ReferencePhoneme(entry.Alias)
		if current == "" {
			continue
		}
		if phoneme != "" && current != phoneme {
			return ""
		}
		phoneme = current
	}
	return phoneme
}
