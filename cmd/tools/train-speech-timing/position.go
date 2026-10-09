package main

import (
	"encoding/json"
	"math"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
)

// 文内の位置の特徴（F0ブランチの入力、--f0-position）。ランタイムのtts.unifiedProsodyPositionと同じ定義。
//
//	0 発話のモーラの通し位置（0〜1）
//	1 アクセント句の通し位置（0〜1）
//	2 最後のアクセント句
//	3 最後の息継ぎ区間（休止で区切った最後の区間）
//	4 疑問文（文末が？または?）
//
// 休止は疑問文だけを持つ。
const f0PositionFeatures = 5

// f0PositionInputは文内の位置の特徴をF0ブランチへ入れるか（--f0-position）。
var f0PositionInput bool

func f0ContextWidth() int {
	if f0PositionInput {
		return f0ContextFeatures + f0PositionFeatures
	}
	return f0ContextFeatures
}

func positionFeatures(tokens []datasetToken, question bool) [][f0PositionFeatures]float32 {
	result := make([][f0PositionFeatures]float32, len(tokens))
	speech, phrases, lastGroup := 0, 0, 0
	for index, token := range tokens {
		if token.Pause {
			continue
		}
		if token.AccentPhraseStart || phrases == 0 {
			phrases++
		}
		if index > 0 && tokens[index-1].Pause {
			lastGroup = speech
		}
		speech++
	}
	position, phrase := 0, -1
	for index, token := range tokens {
		if question {
			result[index][4] = 1
		}
		if token.Pause {
			continue
		}
		if token.AccentPhraseStart || phrase < 0 {
			phrase++
		}
		result[index][0] = ratio(position, speech)
		result[index][1] = ratio(phrase, phrases)
		if phrase == phrases-1 {
			result[index][2] = 1
		}
		if position >= lastGroup {
			result[index][3] = 1
		}
		position++
	}
	return result
}

func ratio(index, count int) float32 {
	if count <= 1 {
		return 0
	}
	return float32(index) / float32(count-1)
}

func questionText(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasSuffix(text, "？") || strings.HasSuffix(text, "?")
}

// attachPositionsは特徴量キャッシュの発話へ、データセットのトークンから文内の位置をフレームごとに付ける。
// キャッシュを作り直さずに位置の特徴を試せるよう、学習時に計算する。
func attachPositions(items []utterance, dataset string) error {
	type line struct {
		ID     string         `json:"id"`
		Text   string         `json:"text"`
		Tokens []datasetToken `json:"tokens"`
	}
	records := map[string]line{}
	if err := toolutil.ScanJSONL(dataset, func(data []byte) error {
		var r line
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		records[r.ID] = r
		return nil
	}); err != nil {
		return err
	}
	for index := range items {
		item := &items[index]
		item.Position = make([]float32, item.Frames*f0PositionFeatures)
		r, ok := records[item.ID]
		if !ok {
			continue
		}
		features := positionFeatures(r.Tokens, questionText(r.Text))
		for tokenIndex, token := range r.Tokens {
			if token.EndMS <= token.StartMS {
				continue
			}
			a := max(0, int(math.Round(token.StartMS/10)))
			b := min(item.Frames, max(int(math.Round(token.EndMS/10)), a+1))
			for t := a; t < b; t++ {
				copy(item.Position[t*f0PositionFeatures:(t+1)*f0PositionFeatures], features[tokenIndex][:])
			}
		}
	}
	return nil
}

// fillF0Contextは1フレーム分のF0ブランチ入力（音素内位置・対数長・抑揚の特徴・文内の位置）を書く。
func fillF0Context(dst []float32, item utterance, frame int) {
	dst[0] = item.Cont[frame*item.Continuous]
	dst[1] = item.Cont[frame*item.Continuous+1]
	copy(dst[2:2+f0ExtraFeatures], item.F0Extra[frame*f0ExtraFeatures:(frame+1)*f0ExtraFeatures])
	if f0PositionInput && len(item.Position) == item.Frames*f0PositionFeatures {
		copy(dst[2+f0ExtraFeatures:2+f0ExtraFeatures+f0PositionFeatures], item.Position[frame*f0PositionFeatures:(frame+1)*f0PositionFeatures])
	}
}
