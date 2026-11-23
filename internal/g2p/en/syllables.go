package en

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

var englishVowels = map[string]bool{
	"aa": true, "ae": true, "ah": true, "ao": true, "aw": true, "ax": true, "ay": true,
	"eh": true, "er": true, "ey": true, "ih": true, "iy": true, "ow": true,
	"oy": true, "uh": true, "uw": true,
}

var englishConsonants = map[string]bool{
	"b": true, "ch": true, "d": true, "dh": true, "dx": true, "f": true, "g": true,
	"hh": true, "jh": true, "k": true, "l": true, "m": true, "n": true, "ng": true,
	"p": true, "r": true, "s": true, "sh": true, "t": true, "th": true,
	"v": true, "w": true, "y": true, "z": true, "zh": true,
}

var cvEnglishConsonants = map[string]bool{
	"b": true, "ch": true, "d": true, "dh": true, "dx": true, "f": true, "g": true,
	"hh": true, "jh": true, "k": true, "l": true, "m": true, "n": true, "ng": true,
	"p": true, "q": true, "r": true, "s": true, "sh": true, "t": true, "th": true,
	"v": true, "w": true, "y": true, "z": true, "zh": true,
}

var englishOnsets = map[string]bool{
	"b l": true, "b r": true, "d r": true, "d w": true, "f l": true, "f r": true,
	"g l": true, "g r": true, "g w": true, "k l": true, "k r": true, "k w": true,
	"p l": true, "p r": true, "sh r": true, "s f": true, "s k": true, "s l": true,
	"s m": true, "s n": true, "s p": true, "s t": true, "s w": true, "t r": true,
	"t w": true, "th r": true, "th w": true, "v r": true,
	"s k l": true, "s k r": true, "s k w": true, "s p l": true, "s p r": true,
	"s t r": true,
}

func IsVowel(phone string) bool            { return englishVowels[phone] }
func IsARPAbetConsonant(phone string) bool { return englishConsonants[phone] }
func IsCVConsonant(phone string) bool      { return cvEnglishConsonants[phone] }

func NormalizeARPAbet(value string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(value)), "0123456789")
}

func ARPAbetStress(value string) int {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	last := value[len(value)-1]
	if last >= '0' && last <= '2' {
		return int(last - '0')
	}
	return 0
}

func ARPAbetHasStress(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) > 0 && value[len(value)-1] >= '0' && value[len(value)-1] <= '2'
}

// Syllableは英語1音節の音韻単位。
type Syllable struct {
	Onset       []string
	Vowel       string
	Coda        []string
	Stress      int
	StressKnown bool
}

// SyllablePhonesは1音節の音素区分（onset/nucleus/coda）を返す。
func SyllablePhones(s Syllable) []linguistics.Phone {
	var phones []linguistics.Phone
	for _, symbol := range s.Onset {
		phones = append(phones, linguistics.Phone{Symbol: symbol, Role: "onset"})
	}
	phones = append(phones, linguistics.Phone{Symbol: s.Vowel, Role: "nucleus"})
	for _, symbol := range s.Coda {
		phones = append(phones, linguistics.Phone{Symbol: symbol, Role: "coda"})
	}
	return phones
}

// NormalizePhonesはARPAbet列から強勢番号を取り除いて小文字化する。
func NormalizePhones(raw []string) []string {
	result := make([]string, 0, len(raw))
	for _, phone := range raw {
		if normalized := NormalizeARPAbet(phone); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

// SyllabifyWordは1語のARPAbet列を音節列へ分割する。
func SyllabifyWord(raw []string) ([]Syllable, error) {
	phones := NormalizePhones(raw)
	for _, phone := range phones {
		if !englishVowels[phone] && !englishConsonants[phone] {
			return nil, fmt.Errorf("unsupported ARPAbet phoneme %q", phone)
		}
	}
	var vowels []int
	for i, phone := range phones {
		if englishVowels[phone] {
			vowels = append(vowels, i)
		}
	}
	if len(vowels) == 0 {
		return nil, nil
	}
	syllables := make([]Syllable, len(vowels))
	for i, vowelAt := range vowels {
		syllables[i].Vowel = phones[vowelAt]
		syllables[i].Stress = ARPAbetStress(raw[vowelAt])
		syllables[i].StressKnown = ARPAbetHasStress(raw[vowelAt])
		if i == 0 {
			syllables[i].Onset = append([]string(nil), phones[:vowelAt]...)
			continue
		}
		cluster := phones[vowels[i-1]+1 : vowelAt]
		onsetCount := englishOnsetSuffixLength(cluster)
		syllables[i-1].Coda = append([]string(nil), cluster[:len(cluster)-onsetCount]...)
		syllables[i].Onset = append([]string(nil), cluster[len(cluster)-onsetCount:]...)
	}
	syllables[len(syllables)-1].Coda = append([]string(nil), phones[vowels[len(vowels)-1]+1:]...)
	return syllables, nil
}

func englishOnsetSuffixLength(cluster []string) int {
	for count := len(cluster); count > 0; count-- {
		if validEnglishOnset(cluster[len(cluster)-count:]) {
			return count
		}
	}
	return 0
}

func validEnglishOnset(cluster []string) bool {
	if len(cluster) == 1 {
		return cluster[0] != "ng"
	}
	return englishOnsets[strings.Join(cluster, " ")]
}

func wordsPronunciation(words []string, dictionary map[string]string) (string, [][]string, error) {
	if len(words) == 0 {
		return "", nil, fmt.Errorf("English text contains no words")
	}
	result := make([][]string, 0, len(words))
	parts := make([]string, 0, len(words))
	for index, word := range words {
		if word == "<pause>" {
			result = append(result, []string{"SP"})
			parts = append(parts, "SP")
			continue
		}
		value := dictionary[word]
		if value == "" {
			value = dictionary[strings.ToLower(word)]
		}
		if value == "" {
			previous, next := "", ""
			if index > 0 {
				previous = words[index-1]
			}
			if index+1 < len(words) {
				next = words[index+1]
			}
			resolved, err := ResolveInContext(word, previous, next)
			if err != nil {
				return "", nil, err
			}
			value = resolved.Phones
		}
		fields := strings.Fields(value)
		if len(fields) > 0 {
			result = append(result, fields)
			parts = append(parts, strings.Join(fields, " "))
		}
	}
	if len(result) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading is empty")
	}
	return strings.Join(parts, " | "), result, nil
}

func parseExplicitReading(reading string) ([][]string, error) {
	var result [][]string
	for _, part := range strings.Split(reading, "|") {
		fields := strings.Fields(part)
		if len(fields) > 0 {
			result = append(result, fields)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("ARPAbet reading is empty")
	}
	return result, nil
}

// MarkPhoneCodasは音素単位モーラ列で単語末尾母音より後ろの子音をcodaとして印付ける。
func MarkPhoneCodas(morae []linguistics.Mora) {
	for i := len(morae) - 1; i >= 0; {
		if morae[i].Pause {
			i--
			continue
		}
		word := morae[i].WordIndex
		trailing := true
		for i >= 0 && !morae[i].Pause && morae[i].WordIndex == word {
			m := &morae[i]
			if m.Vowel != "" {
				trailing = false
			} else if trailing && len(m.Phones) == 1 {
				m.Phones[0].Role = "coda"
				m.DurationScale = linguistics.PhoneWeight(m.Phones[0].Symbol, "coda")
			}
			i--
		}
	}
}
