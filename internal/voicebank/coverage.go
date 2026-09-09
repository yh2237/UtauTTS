package voicebank

import "utautts/internal/frontend"

// Coverage counts usable main-unit positions after normal candidate pruning.
// It is not a measure of pronunciation quality or transition/ending completeness.
type Coverage struct {
	Positions       int                 `json:"positions"`
	Covered         int                 `json:"covered"`
	CandidateCounts []int               `json:"candidate_counts"`
	Missing         []MissingAliasError `json:"missing"`
}

// AuditCoverage keeps the complete linguistic context while continuing past
// missing positions. Pauses have count zero and are excluded from Positions.
// No path is selected across gaps.
func (b *Bank) AuditCoverage(morae []frontend.Mora, tone string) (*Coverage, error) {
	result := &Coverage{CandidateCounts: make([]int, len(morae)), Missing: []MissingAliasError{}}
	layers, err := b.candidateLayersDiagnostic(morae, tone, "", AliasPolicyAuto, "", &result.Missing)
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
