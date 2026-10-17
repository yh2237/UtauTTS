package voicebank

import "github.com/yh2237/UtauTTS/internal/frontend"

// 主原音の網羅性だけを数える。発音品質や遷移・語尾は評価しない。
type Coverage struct {
	Positions       int                 `json:"positions"`
	Covered         int                 `json:"covered"`
	CandidateCounts []int               `json:"candidate_counts"`
	Missing         []MissingAliasError `json:"missing"`
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
		}
	}
	return result, nil
}
