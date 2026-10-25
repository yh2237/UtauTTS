package g2p

import "strings"

// chineseLetterNamesは、中国語の文に混ざったラテン文字を字読みするときのPinyin。
// 標準的なPinyinにない音（kei、faiなど）は、近い音に置き換える。
var chineseLetterNames = map[rune]string{
	'a': "ei1", 'b': "bi1", 'c': "xi1", 'd': "di1", 'e': "yi1",
	'f': "ai4 fu1", 'g': "ji1", 'h': "ai4 chi1", 'i': "ai4", 'j': "jie2",
	'k': "kai1", 'l': "ai4 lu4", 'm': "ai4 mu4", 'n': "en1", 'o': "ou1",
	'p': "pi1", 'q': "qi1", 'r': "a1", 's': "ai4 si1", 't': "ti1",
	'u': "you1", 'v': "wei1", 'w': "da4 bu4 liu2", 'x': "ai4 ke4 si1",
	'y': "wai1", 'z': "zei2",
	'0': "ling2", '1': "yi1", '2': "er4", '3': "san1", '4': "si4",
	'5': "wu3", '6': "liu4", '7': "qi1", '8': "ba1", '9': "jiu3",
}

// ChineseLatinReadingは、ラテン文字の並びを中国語の字読み（Pinyin）にする。
// 外来語辞書にない語でも、エラーにせず読めるようにするための最後の手段。
func ChineseLatinReading(text string) string {
	var parts []string
	for _, r := range strings.ToLower(text) {
		if name, ok := chineseLetterNames[r]; ok {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, " ")
}
