package voicebank

import "utautts/internal/frontend"

// 主原音の網羅性だけを数える。発音品質や遷移・語尾は評価しない。
type Coverage struct {
	Positions       int                 `json:"positions"`
	Covered         int                 `json:"covered"`
	CandidateCounts []int               `json:"candidate_counts"`
	Missing         []MissingAliasError `json:"missing"`
	MissingPhones   []SpeechGap         `json:"missing_phones,omitempty"`
}

// 休止は数えず、欠落位置を越えても言語文脈を保つ。
func (b *Bank) AuditCoverage(morae []frontend.Mora, tone string) (*Coverage, error) {
	result := &Coverage{CandidateCounts: make([]int, len(morae)), Missing: []MissingAliasError{}}
	layers, err := b.candidateLayersDiagnostic(morae, tone, "", AliasPolicyAuto, &result.Missing)
	if err != nil {
		return nil, err
	}
	for i, mora := range morae {
		if mora.Pause {
			continue
		}
		result.Positions++
		result.CandidateCounts[i] = len(layers[i])
		if len(layers[i]) > 0 {
			result.Covered++
			// 任意の解放ではなく、最も欠落の少ない利用可能な候補を報告する。
			best := layers[i][0]
			for _, candidate := range layers[i][1:] {
				if len(candidate.MissingPhones) < len(best.MissingPhones) {
					best = candidate
				}
			}
			result.MissingPhones = append(result.MissingPhones, best.MissingPhones...)
		}
	}
	return result, nil
}
