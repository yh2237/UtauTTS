package main

import (
	"encoding/json"
	"math"
	"math/rand"

	"utautts/cmd/tools/internal/toolutil"
)

// planAugmentは自然時間の発話から、合成時と同じ一定のモーラ長（120ms±15、休止180ms）に並べ直した
// プラン時間の発話を作る（--plan-augment）。F0目標はモーラごとに自然時間の区間を線形に伸縮して写し、
// メル・エネルギー目標はNaNで学習から外す。合成時はプラン時間で推論するため、学習と推論の時間の差を埋める。
func planAugment(items []utterance, dataset string, vocab *vocabulary, seed int64) ([]utterance, error) {
	type line struct {
		ID         string         `json:"id"`
		PlanTiming bool           `json:"plan_timing"`
		Tokens     []datasetToken `json:"tokens"`
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
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed))
	var result []utterance
	for _, item := range items {
		r, ok := records[item.ID]
		if !ok || r.PlanTiming || len(item.F0Target) != item.Frames {
			continue
		}
		tokens := make([]datasetToken, len(r.Tokens))
		copy(tokens, r.Tokens)
		cursor := 100.0
		for index := range tokens {
			length := 120 + (rng.Float64()*2-1)*15
			if tokens[index].Pause {
				length = 180
			}
			tokens[index].StartMS, tokens[index].EndMS = cursor, cursor+length
			cursor += length
		}
		frames := int(math.Round(cursor / 10))
		teacher := make([]float32, frames)
		for t := range teacher {
			teacher[t] = float32(math.NaN())
		}
		for index, token := range tokens {
			source := r.Tokens[index]
			if token.Pause || source.EndMS <= source.StartMS {
				continue
			}
			a, b := int(math.Round(token.StartMS/10)), int(math.Round(token.EndMS/10))
			sa, sb := source.StartMS/10, source.EndMS/10
			for t := a; t < b && t < frames; t++ {
				u := (float64(t-a) + .5) / float64(b-a)
				s := int(math.Floor(sa + u*(sb-sa)))
				if s >= 0 && s < item.Frames {
					teacher[t] = item.F0Target[s]
				}
			}
		}
		augmented, err := featurizePlan(trainingRecord{ID: item.ID + "#plan", Tokens: tokens, PlanTiming: true}, vocab, teacher)
		if err != nil {
			continue
		}
		result = append(result, augmented)
	}
	return result, nil
}
