package openjtalk

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"utautts/internal/prosody"
)

// 列順: string, pos, pos_group1, ctype, cform, orig, read, pron, acc, mora_size, chain_rule, chain_flag
type njdNode struct {
	String    string
	Pos       string
	PosGroup1 string
	Ctype     string
	Cform     string
	Orig      string
	Read      string
	Pron      string
	Acc       int
	MoraSize  int
	ChainRule string
	ChainFlag int
}

type moraToken struct {
	Mora                 string
	Vowel                string
	Pause                bool
	AccentPhrasePosition int
	AccentPhraseLength   int
	AccentNucleus        int
	AccentHigh           bool
	AccentPhraseStart    bool
	AccentPhraseEnd      bool
	WordStart            bool
	WordEnd              bool
	Pos                  string
	PosGroup1            string
}

type kanaMora struct {
	mora  string
	vowel string
	pause bool
}

var punctuation = map[rune]bool{
	'、': true, '。': true, '？': true, '！': true,
	',': true, '.': true, '?': true, '!': true,
}

// frontend.ParseKanaと同じ母音対応を使う。
var vowelGroups = []struct {
	characters string
	vowel      string
}{
	{"あかがさざただなはばぱまやらわぁゃゎ", "a"},
	{"いきぎしじちぢにひびぴみりゐぃ", "i"},
	{"うくぐすずつづぬふぶぷむゆるゔぅゅ", "u"},
	{"えけげせぜてでねへべぺめれゑぇ", "e"},
	{"おこごそぞとどのほぼぽもよろをぉょ", "o"},
}

const smallKana = "ぁぃぅぇぉゃゅょゎゕゖ"

func parseNJD(tsv string) ([]njdNode, error) {
	var nodes []njdNode
	for _, line := range strings.Split(tsv, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 12 {
			return nil, fmt.Errorf("NJD row has %d fields, want 12", len(fields))
		}
		acc, err := strconv.Atoi(fields[8])
		if err != nil {
			return nil, fmt.Errorf("invalid NJD acc %q: %v", fields[8], err)
		}
		moraSize, err := strconv.Atoi(fields[9])
		if err != nil {
			return nil, fmt.Errorf("invalid NJD mora_size %q: %v", fields[9], err)
		}
		chainFlag, err := strconv.Atoi(fields[11])
		if err != nil {
			return nil, fmt.Errorf("invalid NJD chain_flag %q: %v", fields[11], err)
		}
		nodes = append(nodes, njdNode{
			String:    unescapeNJD(fields[0]),
			Pos:       unescapeNJD(fields[1]),
			PosGroup1: unescapeNJD(fields[2]),
			Ctype:     unescapeNJD(fields[3]),
			Cform:     unescapeNJD(fields[4]),
			Orig:      unescapeNJD(fields[5]),
			Read:      unescapeNJD(fields[6]),
			Pron:      unescapeNJD(fields[7]),
			Acc:       acc,
			MoraSize:  moraSize,
			ChainRule: unescapeNJD(fields[10]),
			ChainFlag: chainFlag,
		})
	}
	return nodes, nil
}

func unescapeNJD(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			builder.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case '\\':
			builder.WriteByte('\\')
		case 't':
			builder.WriteByte('\t')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		default:
			builder.WriteByte('\\')
			builder.WriteByte(value[index])
		}
	}
	return builder.String()
}

func buildAnalysis(nodes []njdNode) *Analysis {
	refineAccentPhrases(nodes)
	reading, tokens := analyzeNJD(nodes)
	analysis := &Analysis{
		Version:  1,
		Reading:  reading,
		Morae:    make([]string, len(tokens)),
		Features: make([]prosody.FeatureFrame, len(tokens)),
	}
	for index, token := range tokens {
		analysis.Morae[index] = token.Mora
		analysis.Features[index] = token.sparseFeatures()
	}
	return analysis
}

// 聴取とjsut-labelの人手アクセントで確認した規則を適用する。
func refineAccentPhrases(nodes []njdNode) {
	chainAccentPhrases(nodes)
	accentBeforeTopicParticles(nodes)
}

// Open JTalkが分けすぎる補助動詞とサ変の句をつなぐ。
// 前句に核があれば後句の核を消し、なければ後句の核を残す。
// サ変の判定には補助動詞を連結した後の核が必要なため、最後に処理する。
func chainAccentPhrases(nodes []njdNode) {
	chainPhrases(nodes, isAuxiliaryAfterTe)
	chainPhrases(nodes, isAccentedSahenVerb)
}

func chainPhrases(nodes []njdNode, joins func(previous, node njdNode) bool) {
	head, length := -1, 0
	for index := range nodes {
		node := &nodes[index]
		if node.MoraSize == 0 || isPunctuationString(node.String) {
			head, length = -1, 0
			continue
		}
		switch {
		case head >= 0 && node.ChainFlag != 1 && joins(nodes[index-1], *node):
			node.ChainFlag = 1
			if nodes[head].Acc == 0 && node.Acc > 0 {
				nodes[head].Acc = length + node.Acc
			}
		case head < 0 || node.ChainFlag != 1:
			head, length = index, 0
		}
		length += len(pronunciationMorae(node.Pron))
	}
}

func isAuxiliaryAfterTe(previous, node njdNode) bool {
	return previous.Pos == "助詞" && previous.PosGroup1 == "接続助詞" &&
		(previous.String == "て" || previous.String == "で") &&
		node.Pos == "動詞" && node.PosGroup1 == "非自立"
}

// 核は補助動詞を連結した後の句全体の値。
func isAccentedSahenVerb(previous, node njdNode) bool {
	return previous.Pos == "名詞" && previous.PosGroup1 == "サ変接続" &&
		node.Pos == "動詞" && node.Orig == "する" && node.Acc > 0
}

// 平板句の「には」「ても」などは、係助詞の直前に核を置く。
func accentBeforeTopicParticles(nodes []njdNode) {
	head, length := -1, 0
	for index := range nodes {
		node := &nodes[index]
		if node.MoraSize == 0 || isPunctuationString(node.String) {
			head, length = -1, 0
			continue
		}
		if head < 0 || node.ChainFlag != 1 {
			head, length = index, 0
		} else if nodes[head].Acc == 0 && length > 1 && isTopicParticle(*node) && takesAccentBeforeTopic(nodes[index-1]) {
			nodes[head].Acc = length
		}
		length += len(pronunciationMorae(node.Pron))
	}
}

func isTopicParticle(node njdNode) bool {
	return node.Pos == "助詞" && node.PosGroup1 == "係助詞" && (node.String == "は" || node.String == "も")
}

func takesAccentBeforeTopic(previous njdNode) bool {
	if previous.Pos != "助詞" {
		return false
	}
	switch previous.PosGroup1 {
	case "格助詞":
		switch previous.String {
		case "に", "で", "と", "へ", "から", "まで", "より":
			return true
		}
	case "接続助詞":
		return previous.String == "て" || previous.String == "で"
	}
	return false
}

func pronunciationMorae(pron string) []kanaMora {
	var morae []kanaMora
	for _, mora := range splitMorae(pron) {
		if !mora.pause {
			morae = append(morae, mora)
		}
	}
	return morae
}

func analyzeNJD(nodes []njdNode) (string, []moraToken) {
	var readingParts []string
	var result []moraToken
	index := 0
	for index < len(nodes) {
		node := nodes[index]
		if node.MoraSize == 0 || isPunctuationString(node.String) {
			value := node.String
			if value == "" {
				value = "、"
			}
			readingParts = append(readingParts, value)
			if len(result) > 0 && !result[len(result)-1].Pause {
				result = append(result, moraToken{Pause: true})
			}
			index++
			continue
		}

		type phraseNode struct {
			node  njdNode
			morae []kanaMora
		}
		var phraseNodes []phraseNode
		for index < len(nodes) {
			current := nodes[index]
			if current.MoraSize == 0 || isPunctuationString(current.String) {
				break
			}
			if len(phraseNodes) > 0 && current.ChainFlag != 1 {
				break
			}
			pronunciation := strings.NewReplacer("'", "", "’", "").Replace(current.Pron)
			phraseNodes = append(phraseNodes, phraseNode{node: current, morae: pronunciationMorae(pronunciation)})
			readingParts = append(readingParts, pronunciation)
			index++
		}
		if len(phraseNodes) == 0 {
			continue
		}

		phraseLength := 0
		for _, phraseNode := range phraseNodes {
			phraseLength += len(phraseNode.morae)
		}
		accent := phraseNodes[0].node.Acc
		phrasePosition := 0
		for _, phraseNode := range phraseNodes {
			for wordPosition, mora := range phraseNode.morae {
				phrasePosition++
				result = append(result, moraToken{
					Mora:                 mora.mora,
					Vowel:                mora.vowel,
					Pause:                false,
					AccentPhrasePosition: phrasePosition,
					AccentPhraseLength:   phraseLength,
					AccentNucleus:        accent,
					AccentHigh:           isHigh(phrasePosition, accent),
					AccentPhraseStart:    phrasePosition == 1,
					AccentPhraseEnd:      phrasePosition == phraseLength,
					WordStart:            wordPosition == 0,
					WordEnd:              wordPosition == len(phraseNode.morae)-1,
					Pos:                  phraseNode.node.Pos,
					PosGroup1:            phraseNode.node.PosGroup1,
				})
			}
		}
	}
	return strings.Join(readingParts, ""), result
}

func splitMorae(reading string) []kanaMora {
	normalized := norm.NFC.String(strings.NewReplacer("'", "", "’", "").Replace(reading))
	var result []kanaMora
	for _, character := range normalized {
		if unicode.IsSpace(character) || punctuation[character] {
			if len(result) > 0 && !result[len(result)-1].pause {
				result = append(result, kanaMora{pause: true})
			}
			continue
		}
		mora := toHiragana(character)
		if strings.ContainsRune(smallKana, mora) && len(result) > 0 && !result[len(result)-1].pause {
			result[len(result)-1].mora += string(mora)
			result[len(result)-1].vowel = vowelOf(mora, result[len(result)-1].vowel)
			continue
		}
		if mora == 'ー' {
			previous := ""
			if len(result) > 0 && !result[len(result)-1].pause {
				previous = result[len(result)-1].vowel
			}
			result = append(result, kanaMora{mora: "ー", vowel: previous})
			continue
		}
		result = append(result, kanaMora{mora: string(mora), vowel: vowelOf(mora, "")})
	}
	return result
}

func toHiragana(character rune) rune {
	if character >= 0x30A1 && character <= 0x30F6 {
		return character - 0x60
	}
	return character
}

func vowelOf(character rune, fallback string) string {
	for _, group := range vowelGroups {
		if strings.ContainsRune(group.characters, character) {
			return group.vowel
		}
	}
	switch character {
	case 'ん':
		return "n"
	case 'っ':
		return "cl"
	}
	return fallback
}

func isHigh(position, accent int) bool {
	switch {
	case accent == 1:
		return position == 1
	case accent > 1:
		return position >= 2 && position <= accent
	default:
		return position >= 2
	}
}

func (token moraToken) sparseFeatures() prosody.FeatureFrame {
	if token.Pause {
		return prosody.FeatureFrame{}
	}
	phraseLength := token.AccentPhraseLength
	if phraseLength < 1 {
		phraseLength = 1
	}
	phrasePosition := token.AccentPhrasePosition
	nucleus := token.AccentNucleus
	pos := token.Pos
	if pos == "" {
		pos = "*"
	}
	posGroup1 := token.PosGroup1
	if posGroup1 == "" {
		posGroup1 = "*"
	}
	result := prosody.FeatureFrame{
		"accent_position":         float64(phrasePosition) / float64(phraseLength),
		"accent_from_end":         float64(phraseLength-phrasePosition) / float64(phraseLength),
		"accent_nucleus_position": float64(nucleus) / float64(phraseLength),
		"accent_high":             boolFeature(token.AccentHigh),
		"accent_phrase_start":     boolFeature(token.AccentPhraseStart),
		"accent_phrase_end":       boolFeature(token.AccentPhraseEnd),
		"word_start":              boolFeature(token.WordStart),
		"word_end":                boolFeature(token.WordEnd),
		"pos=" + pos:              1.0,
		"pos_group1=" + posGroup1: 1.0,
	}
	switch {
	case nucleus == 0:
		result["accent_type=heiban"] = 1.0
	case phrasePosition < nucleus:
		result["accent_type=before"] = 1.0
	case phrasePosition == nucleus:
		result["accent_type=nucleus"] = 1.0
	default:
		result["accent_type=after"] = 1.0
	}
	return result
}

func boolFeature(value bool) float64 {
	if value {
		return 1.0
	}
	return 0.0
}

func isPunctuationString(value string) bool {
	runes := []rune(value)
	return len(runes) == 1 && punctuation[runes[0]]
}
