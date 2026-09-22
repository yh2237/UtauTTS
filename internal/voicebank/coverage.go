package voicebank

import "utautts/internal/frontend"

// Coverageは通常の候補絞り込み後の利用可能な主ユニット数を数える。
// 発音品質や遷移・語尾の網羅性の指標ではない。
type Coverage struct {
	Positions       int                 `json:"positions"`
	Covered         int                 `json:"covered"`
	CandidateCounts []int               `json:"candidate_counts"`
	Missing         []MissingAliasError `json:"missing"`
	MissingPhones   []SpeechGap         `json:"missing_phones,omitempty"`
}

// AuditCoverageは欠落位置を越えて続けつつ言語文脈を完全に保つ。
// ポーズはカウント0でPositionsから除外し、欠落をまたぐパスは選ばない。
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
