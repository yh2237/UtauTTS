package voicebank

import (
	"math"
	"sort"

	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func candidateScore(language string, candidateTier int, entry oto.Entry) float64 {
	score := 100 - float64(candidateTier)*10
	if entry.Preutterance >= 0 {
		score += 4
	} else {
		score -= 30 + math.Abs(entry.Preutterance)
	}
	if entry.Fixed >= entry.Preutterance && entry.Fixed >= 0 {
		score += 4
	} else {
		score -= 20 + math.Abs(entry.Preutterance-entry.Fixed)
	}
	if entry.Overlap <= entry.Preutterance || language == frontend.LanguageEnglish {
		// C+V/VCCVの母音はoverlapがpreutteranceを超えても正常。減点すると文頭形が優先される。
		score += 4
	} else {
		score -= 20 + math.Abs(entry.Overlap-entry.Preutterance)
	}
	if entry.Offset >= 0 {
		score += 2
	} else {
		score -= 20
	}
	return score
}

func validatedCandidateScore(language string, candidateTier int, entry oto.Entry, validation EntryValidation) float64 {
	score := candidateScore(language, candidateTier, entry)
	if validation.Status == "degraded" {
		score -= 3
	}
	return score
}

func localCandidateScore(candidate Selection) float64 {
	return candidate.TargetScore + candidate.PreferenceScore
}

// 上限超過時だけ絞り込み、別録音の候補も残す。上限以下では順序を変えない。
func pruneCandidates(candidates []Selection) []Selection {
	if len(candidates) <= maxCandidatesPerPosition {
		return candidates
	}
	ranked := make([]Selection, len(candidates))
	copy(ranked, candidates)
	sort.SliceStable(ranked, func(i, j int) bool {
		return localCandidateScore(ranked[i]) > localCandidateScore(ranked[j])
	})

	limit := maxCandidatesPerPosition
	reserve := min(minDistinctSourceCandidates, limit)
	selected := make([]Selection, 0, limit)
	chosen := make([]bool, len(ranked))

	for index := 0; index < len(ranked) && len(selected) < limit-reserve; index++ {
		selected = append(selected, ranked[index])
		chosen[index] = true
	}

	sources := make(map[string]bool, limit)
	for _, candidate := range selected {
		sources[candidate.Entry.Filename] = true
	}
	for index := 0; index < len(ranked) && len(selected) < limit; index++ {
		if chosen[index] {
			continue
		}
		filename := ranked[index].Entry.Filename
		if filename == "" || sources[filename] {
			continue
		}
		sources[filename] = true
		selected = append(selected, ranked[index])
		chosen[index] = true
	}

	for index := 0; index < len(ranked) && len(selected) < limit; index++ {
		if chosen[index] {
			continue
		}
		selected = append(selected, ranked[index])
		chosen[index] = true
	}
	return selected
}

func applyCompositePreferences(candidates []Selection, policy AliasPolicy) {
	hasComposite := false
	for _, candidate := range candidates {
		if candidate.Composite {
			hasComposite = true
			break
		}
	}
	if !hasComposite {
		return
	}
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Composite {
			candidate.PreferenceScore = compositePreferenceScore(policy)
			continue
		}
		if candidate.Kind == AliasVCV && policy != AliasPolicyCVVCPrefer {
			candidate.PreferenceScore = 10
		}
	}
}

func compositePreferenceScore(policy AliasPolicy) float64 {
	switch policy {
	case AliasPolicyVCVPrefer:
		return 22
	case AliasPolicyCVVCPrefer:
		return 12
	default:
		return 12
	}
}

func policyTier(policy AliasPolicy, tier int, kind AliasKind) int {
	if policy == AliasPolicyVCVPrefer && kind != AliasVCV {
		return tier + 2
	}
	if policy == AliasPolicyCVVCPrefer && kind == AliasVCV {
		return tier + 2
	}
	return tier
}
