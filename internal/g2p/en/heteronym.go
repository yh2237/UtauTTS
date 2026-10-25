package en

import "strings"

var enDeterminers = map[string]bool{
	"the": true, "a": true, "an": true, "this": true, "that": true, "these": true, "those": true,
	"my": true, "your": true, "his": true, "her": true, "its": true, "our": true, "their": true,
	"no": true, "any": true, "some": true, "every": true, "each": true,
}

var enPronouns = map[string]bool{
	"i": true, "we": true, "they": true, "you": true, "he": true, "she": true, "who": true, "people": true,
}

var enReadBaseContexts = map[string]bool{
	"to": true, "please": true, "can": true, "could": true, "will": true, "would": true,
	"shall": true, "should": true, "may": true, "might": true, "must": true,
	"do": true, "does": true, "did": true, "don't": true, "doesn't": true, "didn't": true,
}

var enVerbContexts = map[string]bool{
	"to": true, "please": true, "i": true, "we": true, "they": true, "you": true, "he": true, "she": true,
	"can": true, "could": true, "will": true, "would": true, "shall": true, "should": true,
	"may": true, "might": true, "must": true, "do": true, "does": true, "did": true,
	"don't": true, "doesn't": true, "didn't": true,
}

// heteronymReadingは、前後の語から同綴異音語の読みを選ぶ。
// context-freeの辞書読み（CMUdictの基本形）を既定にし、確かな文脈だけ上書きする。
func heteronymReading(word, previous, next string) (string, bool) {
	word = strings.ToLower(strings.TrimSpace(word))
	previous = contextWord(previous)
	next = contextWord(next)
	switch word {
	case "read":
		if previous == "" || enReadBaseContexts[previous] {
			return "R IY1 D", true
		}
		return "R EH1 D", true
	case "live":
		if enPronouns[previous] || next == "in" || next == "at" || next == "on" || next == "with" ||
			next == "near" || next == "here" || next == "there" || next == "alone" || next == "together" {
			return "L IH1 V", true
		}
		return "L AY1 V", true
	case "lead":
		if enLeadMetal[next] {
			return "L EH1 D", true
		}
		if enVerbContexts[previous] || enDeterminers[next] || next == "to" || next == "us" || next == "them" {
			return "L IY1 D", true
		}
		return "L EH1 D", true
	case "use":
		if enDeterminers[previous] && (next == "" || next == "of" || next == "for" || next == "in" || next == "as" || next == "by" || next == "with") {
			return "Y UW1 S", true
		}
		if previous == "" || enPronouns[previous] || enVerbContexts[previous] {
			return "Y UW1 Z", true
		}
		return "Y UW1 S", true
	case "record":
		if enDeterminers[previous] {
			return "R EH1 K ER0 D", true
		}
		return "R AH0 K AO1 R D", true
	case "project":
		if previous == "" || enVerbContexts[previous] {
			return "P R AH0 JH EH1 K T", true
		}
		return "P R AA1 JH EH0 K T", true
	case "present":
		if previous == "" || enVerbContexts[previous] {
			return "P R IH0 Z EH1 N T", true
		}
		return "P R EH1 Z AH0 N T", true
	case "object":
		if enVerbContexts[previous] {
			return "AH0 B JH EH1 K T", true
		}
		return "AA1 B JH EH0 K T", true
	case "content":
		if next == "with" || previous == "am" || previous == "is" || previous == "are" ||
			previous == "was" || previous == "were" || previous == "be" || previous == "feel" || previous == "feels" {
			return "K AH0 N T EH1 N T", true
		}
		return "K AA1 N T EH0 N T", true
	case "wind":
		if enDeterminers[previous] {
			return "W IH1 N D", true
		}
		return "W AY1 N D", true
	case "refuse":
		if enDeterminers[previous] {
			return "R EH1 F Y UW2 S", true
		}
		return "R AH0 F Y UW1 Z", true
	case "desert":
		if previous == "to" || enPronouns[previous] || next == "ed" || next == "ing" {
			return "D IH0 Z ER1 T", true
		}
		return "D EH1 Z ER0 T", true
	}
	return "", false
}

var enLeadMetal = map[string]bool{
	"pipe": true, "pipes": true, "paint": true, "poisoning": true, "free": true, "acid": true,
	"battery": true, "batteries": true, "bullet": true, "bullets": true, "pencil": true,
	"pencils": true, "weight": true, "weights": true, "exposure": true, "contamination": true,
}

func contextWord(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "<pause>" {
		return ""
	}
	return strings.Trim(word, ".,!?;:\"'()[]")
}
