package en

import "strings"

// englishLetterNamesは、頭字語を字読みするときのARPAbet。強勢は最初の文字だけprimary。
var englishLetterNames = map[rune]string{
	'A': "EY1", 'B': "B IY1", 'C': "S IY1", 'D': "D IY1", 'E': "IY1",
	'F': "EH1 F", 'G': "JH IY1", 'H': "EY1 CH", 'I': "AY1", 'J': "JH EY1",
	'K': "K EY1", 'L': "EH1 L", 'M': "EH1 M", 'N': "EH1 N", 'O': "OW1",
	'P': "P IY1", 'Q': "K Y UW1", 'R': "AA1 R", 'S': "EH1 S", 'T': "T IY1",
	'U': "Y UW1", 'V': "V IY1", 'W': "D AH1 B AH0 L Y UW0", 'X': "EH1 K S",
	'Y': "W AY1", 'Z': "Z IY1",
}

// 大文字の並びが3〜4文字で母音を含む場合、字読みか語読みかを決めるための語。
var englishSpelledAcronyms = map[string]bool{
	"USB": true, "CPU": true, "GPU": true, "API": true, "URL": true, "HTML": true,
	"HTTP": true, "SQL": true, "PDF": true, "CEO": true, "CFO": true, "CIA": true,
	"FBI": true, "NBA": true, "NFL": true, "USA": true, "UK": true, "EU": true,
	"GPS": true, "SMS": true, "VIP": true, "ATM": true, "FAQ": true, "DIY": true,
	"DVD": true, "CD": true, "RAM": true, "ROM": true, "SSD": true, "HDD": true,
	"LAN": true, "WAN": true, "VPN": true, "DNS": true, "SSH": true, "FTP": true,
	"JSON": true, "XML": true, "CSS": true, "PHP": true, "SDK": true, "CLI": true,
	"GUI": true, "TTS": true, "STT": true, "ASR": true, "LLM": true, "GPT": true,
	"AI": true, "IT": true, "ID": true, "PC": true, "TV": true, "QR": true,
	"US": true, "AM": true, "PM": true, "HR": true, "PR": true,
}

// 大文字でも語として読む略語。辞書にあればそちらを優先する。
var englishWordAcronyms = map[string]bool{
	"NASA": true, "NATO": true, "UNESCO": true, "AIDS": true, "LASER": true,
	"SCUBA": true, "RADAR": true, "PIN": true, "GIF": true, "JPEG": true,
	"SIM": true, "ASCII": true, "ASAP": true, "AWOL": true, "IKEA": true, "OK": true,
}

// englishAcronymPronunciationは、語中の大文字の並びを字読みする。
// ChatGPT（Chat + GPT）、OpenAI（Open + AI）、GDP のような語に効く。
func englishAcronymPronunciation(word string) (string, bool) {
	start, end, ok := englishAcronymRun(word)
	if !ok {
		return "", false
	}
	var parts []string
	if start > 0 {
		before, err := englishWordPronunciation(word[:start])
		if err != nil || before == "" {
			return "", false
		}
		parts = append(parts, before)
	}
	for i, r := range word[start:end] {
		name, ok := englishLetterNames[r]
		if !ok {
			return "", false
		}
		if i > 0 {
			name = strings.ReplaceAll(name, "1", "2")
		}
		parts = append(parts, name)
	}
	if end < len(word) {
		after, err := englishWordPronunciation(word[end:])
		if err != nil || after == "" {
			return "", false
		}
		parts = append(parts, after)
	}
	return strings.Join(parts, " "), true
}

// englishAcronymRunは、字読みする大文字の並びを返す。
func englishAcronymRun(word string) (int, int, bool) {
	bestStart, bestEnd := -1, -1
	for i := 0; i < len(word); {
		if word[i] < 'A' || word[i] > 'Z' {
			i++
			continue
		}
		start := i
		for i < len(word) && word[i] >= 'A' && word[i] <= 'Z' {
			i++
		}
		if i-start < 2 {
			continue
		}
		run := word[start:i]
		if !englishSpellAcronym(run) {
			continue
		}
		if bestStart < 0 || i-start > bestEnd-bestStart {
			bestStart, bestEnd = start, i
		}
	}
	if bestStart < 0 {
		return 0, 0, false
	}
	return bestStart, bestEnd, true
}

func englishSpellAcronym(run string) bool {
	if englishWordAcronyms[run] {
		return false
	}
	if englishSpelledAcronyms[run] || !englishHasVowel(run) {
		return true
	}
	if reading, _ := lookupEnglishDictionary(strings.ToLower(run)); reading != "" {
		return false
	}
	// 長い大文字語は、略語ではなく名前や強調のことが多い。字読みしない。
	if len(run) > 5 && englishHasVowel(run) {
		return false
	}
	return true
}

func englishHasVowel(run string) bool {
	return strings.ContainsAny(run, "AEIOU")
}
