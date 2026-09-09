package frontend

import (
	"fmt"
	"github.com/mozillazg/go-pinyin"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A deliberately small, independently written seed lexicon. User entries win.
// This is not a statistical polyphone disambiguator; unknown words use go-pinyin.
var chineseWords = map[string]string{
	"银行": "yin2 hang2", "銀行": "yin2 hang2", "行长": "hang2 zhang3", "行長": "hang2 zhang3",
	"行业": "hang2 ye4", "行業": "hang2 ye4", "行为": "xing2 wei2", "行为人": "xing2 wei2 ren2",
	"行走": "xing2 zou3", "旅行": "lv3 xing2", "进行": "jin4 xing2",
	"重庆": "chong2 qing4", "重慶": "chong2 qing4", "重新": "chong2 xin1", "重复": "chong2 fu4",
	"重要": "zhong4 yao4", "重量": "zhong4 liang4", "长大": "zhang3 da4", "長大": "zhang3 da4",
	"长江": "chang2 jiang1", "長江": "chang2 jiang1", "长度": "chang2 du4",
	"音乐": "yin1 yue4", "音樂": "yin1 yue4", "乐器": "yue4 qi4", "快乐": "kuai4 le4",
	"了解": "liao3 jie3", "觉得": "jue2 de5", "睡觉": "shui4 jiao4", "便宜": "pian2 yi5",
	"方便": "fang1 bian4", "角色": "jue2 se4", "还给": "huan2 gei3", "还有": "hai2 you3",
	"你好": "ni3 hao3", "很好": "hen3 hao3", "可以": "ke3 yi3", "准备": "zhun3 bei4",
	"我们": "wo3 men5", "你们": "ni3 men5", "他们": "ta1 men5", "什么": "shen2 me5",
	"怎么": "zen3 me5", "这个": "zhe4 ge5", "那个": "na4 ge5", "妈妈": "ma1 ma5", "爸爸": "ba4 ba5",
	"朋友": "peng2 you5", "东西": "dong1 xi5", "知道": "zhi1 dao5",
	"吗": "ma5", "呢": "ne5", "吧": "ba5", "的": "de5", "了": "le5",
}

type chineseToken struct {
	reading, source string
	word            int
	end             bool
}

func chineseReadingTokens(text string, dictionary map[string]string) ([]chineseToken, error) {
	entries := make(map[string]string, len(chineseWords)+len(dictionary))
	for k, v := range chineseWords {
		entries[k] = v
	}
	for k, v := range dictionary {
		if k != "" && v != "" {
			entries[k] = v
		}
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	args := pinyin.NewArgs()
	args.Style = pinyin.Tone3
	var result []chineseToken
	word := 0
	text = normalizeChineseText(text)
	for len(text) > 0 {
		matched := false
		for _, key := range keys {
			if !strings.HasPrefix(text, key) {
				continue
			}
			fields := strings.Fields(entries[key])
			runes := []rune(key)
			for i, field := range fields {
				source := ""
				if len(runes) == len(fields) {
					source = string(runes[i])
				}
				result = append(result, chineseToken{field, source, word, i+1 == len(fields)})
			}
			text = strings.TrimPrefix(text, key)
			word++
			matched = true
			break
		}
		if matched {
			continue
		}
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsPunct(r) {
			result = append(result, chineseToken{reading: "|", word: word, end: true})
			word++
			continue
		}
		values := pinyin.SinglePinyin(r, args)
		if len(values) == 0 {
			return nil, fmt.Errorf("cannot convert %q to Pinyin; specify --reading", string(r))
		}
		value := values[0]
		if pinyinTone(value) == 0 {
			value += "5"
		}
		result = append(result, chineseToken{value, string(r), word, true})
		word++
	}
	return result, nil
}
