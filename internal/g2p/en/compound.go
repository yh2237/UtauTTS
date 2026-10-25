package en

import "strings"

// englishCompoundPronunciationは、辞書にある2語の組み合わせとして読める語を分解する。
// OpenAI（open + ai）やMiku（mi + ku）のように、固有名詞の一部が辞書にある場合に効く。
func englishCompoundPronunciation(word string) (string, error) {
	lower := strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	if len(lower) < 4 || !englishLettersOnly(lower) {
		return "", nil
	}
	for split := len(lower) - 2; split >= 2; split-- {
		left, right := lower[:split], lower[split:]
		if len(right) < 2 {
			continue
		}
		leftReading, err := lookupEnglishDictionary(left)
		if err != nil {
			return "", err
		}
		if leftReading == "" {
			continue
		}
		rightReading, err := lookupEnglishDictionary(right)
		if err != nil {
			return "", err
		}
		if rightReading == "" {
			continue
		}
		reading := leftReading + " " + rightReading
		if englishReadingPlausible(lower, reading) {
			return reading, nil
		}
	}
	return "", nil
}

// englishNeuralFallbackは辞書・屈折・接頭辞・複合語で読めない語を、同梱のGRUで読む。
// 全大文字の略語や、大文字が3文字以上続く語は、現行の綴り規則（字読み）へ任せる。
func englishNeuralFallback(word string) (string, bool) {
	lower := strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	if len(lower) < 3 || !englishLettersOnly(lower) || englishAcronymLike(word) {
		return "", false
	}
	// 小文字だけの語は、作った語や擬音を綴り規則で読む方が安定する。
	// 固有名詞は先頭か途中が大文字なので、そのときだけ神経の読みを使う。
	if word == lower {
		return "", false
	}
	reading, ok := englishNeuralPronunciation(lower)
	if !ok || !englishReadingPlausible(lower, reading) {
		return "", false
	}
	return reading, true
}

// englishAcronymLikeは、字読みの方が適する語かを返す。
func englishAcronymLike(word string) bool {
	letters := 0
	upper := 0
	run := 0
	longest := 0
	for _, r := range word {
		if r < 'A' || r > 'Z' {
			if r >= 'a' && r <= 'z' {
				letters++
			}
			if run > longest {
				longest = run
			}
			run = 0
			continue
		}
		letters++
		upper++
		run++
	}
	if run > longest {
		longest = run
	}
	if letters > 1 && upper == letters {
		return true
	}
	return longest >= 3
}

func englishLettersOnly(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func englishReadingPlausible(word, reading string) bool {
	phones := strings.Fields(reading)
	if len(phones) == 0 {
		return false
	}
	vowels := 0
	for _, phone := range phones {
		if phone == "<unk>" || phone == "<pad>" || phone == "<s>" || phone == "</s>" {
			return false
		}
		if englishVowelPhone(phone) {
			vowels++
		}
	}
	if vowels == 0 {
		return false
	}
	// 文字数に対して音素数が少なすぎる読みは、子音を落とした失敗とみなす。
	// 逆に多すぎる読みは、短い部分を字読みした失敗（ph + lame など）とみなす。
	letters := len([]rune(word))
	return len(phones)*2 >= letters && len(phones)*10 <= letters*11
}

func englishVowelPhone(phone string) bool {
	base := strings.TrimRight(phone, "0123456789")
	switch base {
	case "AA", "AE", "AH", "AO", "AW", "AY", "EH", "ER", "EY", "IH", "IY", "OW", "OY", "UH", "UW":
		return true
	}
	return false
}
