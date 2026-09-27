package voicebank

import "utautts/internal/frontend"

// 収録音素数を最大化し、接続数を抑える。不足音素の時刻は保つ。
func selectCodaChain(bank *Bank, edges []frontend.CodaAlias, affix Affix, hasAffix bool) []frontend.CodaAlias {
	if len(edges) == 0 {
		return nil
	}
	end := 0
	start := edges[0].CodaStart
	usable := make([]bool, len(edges))
	for i, edge := range edges {
		end = max(end, edge.CodaStart+len(edge.Phones))
		names := explicitAliasCandidates(edge.Aliases, AliasOther)
		if hasAffix {
			names = affixCandidatesWithFallback(names, affix, true)
		}
		usable[i] = hasUsableCandidateEntries(bank, names)
	}
	type path struct {
		covered, joins int
		edges          []frontend.CodaAlias
		valid          bool
	}
	dp := make([]path, end+1)
	dp[end].valid = true
	for pos := end - 1; pos >= start; pos-- {
		for i, edge := range edges {
			if edge.CodaStart != pos || (!usable[i] && len(edge.Phones) != 1) {
				continue
			}
			next := dp[pos+len(edge.Phones)]
			if !next.valid {
				continue
			}
			candidate := path{covered: next.covered, joins: next.joins, valid: true}
			if usable[i] {
				candidate.covered += len(edge.Phones)
				candidate.joins++
			}
			if !dp[pos].valid || candidate.covered > dp[pos].covered || candidate.covered == dp[pos].covered && candidate.joins < dp[pos].joins {
				candidate.edges = append([]frontend.CodaAlias{edge}, next.edges...)
				dp[pos] = candidate
			}
		}
	}
	if dp[start].covered == 0 {
		return nil
	}
	return dp[start].edges
}
