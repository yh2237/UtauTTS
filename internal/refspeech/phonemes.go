// Package refspeechは英語の文を参照TTS（Kokoro）で読み、音源のフレームで合成し直す。
package refspeech

import (
	"strings"
	"unicode"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

var kokoroVowels = map[string]string{
	"AA": "ɑ", "AE": "æ", "AO": "ɔ", "AW": "W", "AY": "I", "EH": "ɛ", "EY": "A", "IH": "ɪ",
	"IY": "i", "OW": "O", "OY": "Y", "UH": "ʊ", "UW": "u",
}

var kokoroConsonants = map[string]string{
	"B": "b", "CH": "ʧ", "D": "d", "DH": "ð", "F": "f", "G": "ɡ", "HH": "h", "JH": "ʤ", "K": "k",
	"L": "l", "M": "m", "N": "n", "NG": "ŋ", "P": "p", "R": "ɹ", "S": "s", "SH": "ʃ", "T": "t",
	"TH": "θ", "V": "v", "W": "w", "Y": "j", "Z": "z", "ZH": "ʒ",
}

// Pieceは音素列の断片。Partは読みの何番目の語（SPを含む）か。語の間の空白と文末の記号は-1。
type Piece struct {
	Symbols string
	Part    int
}

// KokoroPhonemesは文の語をmisakiの辞書で引いてKokoroの音素列にする。
// 辞書にない語と利用者辞書にある語は、読み（「HH AH0 L OW1 | SP | ...」形式のARPAbet）から直す。
// 語の並びが読みと合わないとき（読みを直接指定したときなど）は、すべて読みから直す。
func KokoroPhonemes(lexicon *Lexicon, text, reading string, dictionary map[string]string) string {
	var b strings.Builder
	for _, piece := range KokoroPieces(lexicon, text, reading, dictionary) {
		b.WriteString(piece.Symbols)
	}
	return b.String()
}

func KokoroPieces(lexicon *Lexicon, text, reading string, dictionary map[string]string) []Piece {
	parts := readingWords(reading)
	words := frontend.EnglishWords(text)
	if lexicon == nil || len(words) != len(parts) {
		words = nil
	}
	symbols := make([]string, len(parts))
	context := wordContext{}
	for i := len(parts) - 1; i >= 0; i-- {
		if len(parts[i]) == 1 && parts[i][0] == "SP" {
			symbols[i] = ","
			context = context.next("", ",")
			continue
		}
		word := ""
		if words != nil {
			word = words[i]
		}
		phonemes, ok := "", false
		if word != "" && dictionary[word] == "" && dictionary[strings.ToLower(word)] == "" {
			phonemes, ok = lexicon.Word(word, context)
		}
		if !ok {
			phonemes = arpabetWord(parts[i])
		}
		symbols[i] = strings.NewReplacer("ɾ", "T", "ʔ", "t").Replace(phonemes)
		context = context.next(word, phonemes)
	}
	var pieces []Piece
	spoken := false
	for i, symbol := range symbols {
		if symbol == "," {
			if spoken && i < len(symbols)-1 && pieces[len(pieces)-1].Symbols != "," {
				pieces = append(pieces, Piece{",", i})
			}
			continue
		}
		if spoken {
			pieces = append(pieces, Piece{" ", -1})
		}
		pieces = append(pieces, Piece{symbol, i})
		spoken = true
	}
	end := "."
	if strings.HasSuffix(strings.TrimSpace(text), "?") {
		end = "?"
	}
	return append(pieces, Piece{end, -1})
}

func readingWords(reading string) [][]string {
	var words [][]string
	for _, word := range strings.Split(reading, "|") {
		if phones := strings.Fields(word); len(phones) > 0 {
			words = append(words, phones)
		}
	}
	return words
}

// arpabetWordはARPAbetの1語をKokoroの記号にする。強勢は母音の前に置く。
func arpabetWord(phones []string) string {
	var b strings.Builder
	for _, phone := range phones {
		base := strings.TrimRightFunc(phone, unicode.IsDigit)
		stress := strings.TrimPrefix(phone, base)
		if symbol, ok := kokoroConsonants[base]; ok {
			b.WriteString(symbol)
			continue
		}
		switch stress {
		case "1":
			b.WriteRune(primaryStress)
		case "2":
			b.WriteRune(secondaryStress)
		}
		switch base {
		case "AH":
			b.WriteString(map[bool]string{true: "ə", false: "ʌ"}[stress == "0"])
		case "ER":
			b.WriteString(map[bool]string{true: "əɹ", false: "ɜɹ"}[stress == "0"])
		default:
			b.WriteString(kokoroVowels[base])
		}
	}
	return b.String()
}
