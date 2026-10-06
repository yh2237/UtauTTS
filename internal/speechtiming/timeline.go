package speechtiming

import (
	"math"

	"utautts/internal/frontend"
)

type phoneSpan struct {
	start, end float64 // 秒
	label      string
}

// 音素時刻は秒。startsはノート開始、endsは休止直前と発話末。
func phoneTimeline(morae []Mora, marginMS float64, frames int) ([]phoneSpan, []float64, []float64) {
	var phones []phoneSpan
	var starts, ends []float64
	cursor := 0.0
	previousVowel := ""
	for _, mora := range morae {
		note := (marginMS + mora.NoteStartMS) / 1000
		end := (marginMS + mora.NoteStartMS + mora.DurationMS) / 1000
		if len(mora.Spans) > 0 {
			if len(starts) > 0 && note > cursor+1e-3 {
				ends = append(ends, cursor)
			}
			for _, span := range mora.Spans {
				start := (marginMS + span.StartMS) / 1000
				stop := (marginMS + span.StartMS + span.DurationMS) / 1000
				if stop <= start {
					continue
				}
				if start > cursor+1e-3 {
					label := "sil"
					if len(phones) > 0 && start-cursor <= longPauseSec {
						label = phones[len(phones)-1].label
					}
					phones = append(phones, phoneSpan{cursor, start, label})
				}
				phones = append(phones, phoneSpan{start, stop, span.Label})
				if stop > cursor {
					cursor = stop
				}
			}
			if end > cursor+1e-3 {
				label := "sil"
				if len(phones) > 0 && end-cursor <= longPauseSec {
					label = phones[len(phones)-1].label
				}
				phones = append(phones, phoneSpan{cursor, end, label})
				cursor = end
			}
			starts = append(starts, note)
			continue
		}
		labels := moraPhones(mora.Text, previousVowel)
		vowel := labels[len(labels)-1]
		consonants := labels[:len(labels)-1]
		onset := note
		if len(consonants) > 0 {
			onset = note - mora.EffectivePreutteranceMS/1000
		}
		// 子音は前の母音に食い込むため、母音を子音開始で切る。
		if onset < cursor && len(phones) > 0 {
			last := &phones[len(phones)-1]
			onset = math.Max(onset, last.start+FrameMS/1000)
			last.end = math.Min(last.end, onset)
		}
		onset = math.Max(onset, 0)
		if onset > cursor+1e-3 {
			label := "sil"
			if len(phones) > 0 && onset-cursor <= longPauseSec {
				label = phones[len(phones)-1].label
			}
			phones = append(phones, phoneSpan{cursor, onset, label})
		}
		if len(consonants) > 0 && note > onset {
			step := (note - onset) / float64(len(consonants))
			for index, consonant := range consonants {
				phones = append(phones, phoneSpan{onset + float64(index)*step, onset + float64(index+1)*step, consonant})
			}
		}
		end = (marginMS + mora.NoteStartMS + mora.DurationMS) / 1000
		phones = append(phones, phoneSpan{note, end, vowel})
		if len(starts) > 0 && note > cursor+1e-3 {
			ends = append(ends, cursor)
		}
		starts = append(starts, note)
		cursor = end
		switch vowel {
		case "a", "i", "u", "e", "o":
			previousVowel = vowel
		}
	}
	ends = append(ends, cursor)
	if total := float64(frames) * FrameMS / 1000; total > cursor {
		phones = append(phones, phoneSpan{cursor, total, "sil"})
	}
	return phones, starts, ends
}

func moraPhones(text, previousVowel string) []string {
	switch text {
	case "ー":
		if previousVowel == "" {
			return []string{"a"}
		}
		return []string{previousVowel}
	case "っ", "ッ":
		return []string{"cl"}
	case "ん", "ン":
		return []string{"N"}
	case "を", "ヲ":
		return []string{"o"}
	}
	parsed, err := frontend.ParseKana(text)
	if err != nil || len(parsed) == 0 {
		return []string{"<unk>"}
	}
	mora := parsed[0]
	vowel := mora.Vowel
	switch vowel {
	case "a", "i", "u", "e", "o":
	case "n":
		return []string{"N"}
	case "cl":
		return []string{"cl"}
	default:
		return []string{"<unk>"}
	}
	consonant := mora.Consonant
	if consonant == "" {
		consonant = specialConsonant(text)
	}
	if consonant == "" {
		return []string{vowel}
	}
	return []string{consonant, vowel}
}

// frontendで子音が付かない外来音を補う。
func specialConsonant(text string) string {
	runes := []rune(text)
	if len(runes) < 2 {
		return ""
	}
	switch runes[0] {
	case 'て', 'テ':
		return "t"
	case 'で', 'デ':
		return "d"
	case 'ふ', 'フ':
		return "f"
	case 'う', 'ウ':
		return "w"
	case 'ゔ', 'ヴ':
		return "v"
	}
	return ""
}
