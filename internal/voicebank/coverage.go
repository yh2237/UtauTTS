package voicebank

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// 主原音の網羅性だけを数える。発音品質や遷移・語尾は評価しない。
type Coverage struct {
	Positions       int                 `json:"positions"`
	Covered         int                 `json:"covered"`
	CandidateCounts []int               `json:"candidate_counts"`
	Missing         []MissingAliasError `json:"missing"`
}

// 休止は数えず、欠落位置を越えても言語文脈を保つ。
func (b *Bank) AuditCoverage(morae []linguistics.Mora, tone string) (*Coverage, error) {
	return b.AuditCoverageWithConfig(morae, ResolveConfig{Tone: tone})
}

// AuditCoverageWithConfigは事前解決したAliasHintsや声色設定を含めて網羅性を数える。
func (b *Bank) AuditCoverageWithConfig(morae []linguistics.Mora, cfg ResolveConfig) (*Coverage, error) {
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = AliasPolicyAuto
	}
	if !policy.valid() {
		return nil, fmt.Errorf("unknown alias policy %q", policy)
	}
	result := &Coverage{CandidateCounts: make([]int, len(morae)), Missing: []MissingAliasError{}}
	layers, err := b.candidateLayersDiagnostic(morae, cfg.Aliases, cfg.Tone, cfg.Color, policy, &result.Missing)
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
