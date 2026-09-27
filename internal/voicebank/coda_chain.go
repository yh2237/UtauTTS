package voicebank

import (
	"sort"
	"utautts/internal/frontend"
)

// 最大被覆の経路を最大8件残す。接続数だけで早期確定しない。
func selectCodaChains(bank *Bank, edges []frontend.CodaAlias, affix Affix, hasAffix bool) [][]frontend.CodaAlias {
	if len(edges) == 0 {
		return nil
	}
	end, start := 0, edges[0].CodaStart
	usable := make([]bool, len(edges))
	for i, e := range edges {
		if len(e.Phones) == 0 || e.CodaStart < 0 {
			return nil
		}
		start = min(start, e.CodaStart)
		end = max(end, e.CodaStart+len(e.Phones))
		names := explicitAliasCandidates(e.Aliases, AliasOther)
		if hasAffix {
			names = affixCandidatesWithFallback(names, affix, true)
		}
		usable[i] = hasUsableCandidateEntries(bank, names)
	}
	type path struct {
		covered, joins int
		edges          []frontend.CodaAlias
	}
	dp := make([][]path, end+1)
	dp[end] = []path{{}}
	for pos := end - 1; pos >= start; pos-- {
		var candidates []path
		for i, e := range edges {
			if e.CodaStart != pos || (!usable[i] && len(e.Phones) != 1) {
				continue
			}
			for _, next := range dp[pos+len(e.Phones)] {
				p := path{covered: next.covered, joins: next.joins, edges: append([]frontend.CodaAlias{e}, next.edges...)}
				if usable[i] {
					p.covered += len(e.Phones)
					p.joins++
				}
				candidates = append(candidates, p)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].covered != candidates[j].covered {
				return candidates[i].covered > candidates[j].covered
			}
			return candidates[i].joins < candidates[j].joins
		})
		if len(candidates) > 0 {
			best := candidates[0]
			for _, p := range candidates {
				if p.covered != best.covered || p.joins > best.joins+2 || len(dp[pos]) >= 8 {
					break
				}
				dp[pos] = append(dp[pos], p)
			}
		}
	}
	var result [][]frontend.CodaAlias
	for _, p := range dp[start] {
		if p.covered > 0 {
			result = append(result, p.edges)
		}
	}
	return result
}

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
