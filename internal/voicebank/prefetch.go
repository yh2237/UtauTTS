package voicebank

import (
	"sort"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// 先読み用の候補を返す。WAVの検証と原音選択は合成時に行う。
func (b *Bank) CandidateSourcePaths(morae []frontend.Mora, cfg ResolveConfig) []string {
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = AliasPolicyAuto
	}
	affix, _, hasAffix := b.AffixForToneAndColor(cfg.Tone, cfg.Color)
	paths := make(map[string]bool)
	collect := func(specs []aliasCandidate) {
		if hasAffix {
			specs = affixCandidatesWithFallback(specs, affix, true)
		}
		for _, spec := range specs {
			for _, entry := range b.Entries[spec.name] {
				if entry.Filename != "" {
					paths[entry.Filename] = true
				}
			}
		}
	}
	previous, initial := "", true
	for _, mora := range morae {
		if mora.Pause {
			previous, initial = "", true
			continue
		}
		if mora.Aliases != nil && len(mora.Aliases.Main) > 0 {
			collect(explicitMainAliasCandidates(mora.Aliases.Main, mora.Aliases.MainKinds, mora.Text))
			collect(explicitAliasCandidates(mora.Aliases.Transition, AliasVC))
		} else {
			collect(aliasCandidatesWithPolicy(mora.Text, previous, initial, policy))
			if previous == "cl" {
				collect(aliasCandidatesWithPolicy(mora.Text, "", true, policy))
			}
			consonant := mora.Consonant
			if consonant == "" {
				consonant = frontend.ConsonantOf(mora.Text)
			}
			collect(vcAliasCandidates(previous, consonant, policy))
		}
		if mora.Aliases != nil {
			for _, aliases := range mora.Aliases.Endings {
				collect(explicitAliasCandidates(aliases, AliasOther))
			}
			for _, alternatives := range mora.Aliases.EndingFallbacks {
				for _, alternative := range alternatives {
					collect(explicitAliasCandidates(alternative.Aliases, AliasOther))
				}
			}
		}
		previous, initial = mora.Vowel, false
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
