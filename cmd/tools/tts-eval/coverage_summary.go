package main

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"utautts/internal/atomicfile"
)

// coverageSummaryは言語ごとに音源の原音網羅性を集計する。
// missing_phone_rateは必須音が欠けた位置の割合で、0に近いほど音源が発音を網羅している。
type coverageSummary struct {
	Language           string         `json:"language"`
	Cases              int            `json:"cases"`
	FailedCases        int            `json:"failed_cases"`
	Positions          int            `json:"positions"`
	Covered            int            `json:"covered"`
	CoverageRate       float64        `json:"coverage_rate"`
	MissingPositions   int            `json:"missing_positions"`
	MissingPhoneGroups int            `json:"missing_phone_groups"`
	MissingPhoneRate   float64        `json:"missing_phone_rate"`
	MissingPhones      map[string]int `json:"missing_phones,omitempty"`
	MissingMorae       map[string]int `json:"missing_morae,omitempty"`
}

// summarizeCoverageは診断行を言語別にまとめる。
func summarizeCoverage(rows []diagnostic) []coverageSummary {
	byLanguage := map[string]*coverageSummary{}
	order := []string{}
	for _, row := range rows {
		language := row.Language
		if language == "" {
			language = "unknown"
		}
		summary, found := byLanguage[language]
		if !found {
			summary = &coverageSummary{Language: language, MissingPhones: map[string]int{}, MissingMorae: map[string]int{}}
			byLanguage[language] = summary
			order = append(order, language)
		}
		summary.Cases++
		if row.Error != "" {
			summary.FailedCases++
		}
		if row.Coverage == nil {
			continue
		}
		summary.Positions += row.Coverage.Positions
		summary.Covered += row.Coverage.Covered
		summary.MissingPositions += len(row.Coverage.Missing)
		for _, missing := range row.Coverage.Missing {
			if missing.Mora != "" {
				summary.MissingMorae[missing.Mora]++
			}
		}
		summary.MissingPhoneGroups += len(row.Coverage.MissingPhones)
		for _, gap := range row.Coverage.MissingPhones {
			for _, phone := range gap.Phones {
				if phone != "" {
					summary.MissingPhones[phone]++
				}
			}
		}
	}
	result := make([]coverageSummary, 0, len(order))
	for _, language := range order {
		summary := byLanguage[language]
		if summary.Positions > 0 {
			summary.CoverageRate = float64(summary.Covered) / float64(summary.Positions)
			summary.MissingPhoneRate = float64(summary.MissingPhoneGroups) / float64(summary.Positions)
		}
		result = append(result, *summary)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Language < result[j].Language })
	return result
}

func writeCoverageSummary(out string, rows []diagnostic) error {
	data, err := json.MarshalIndent(summarizeCoverage(rows), "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(out, "coverage_summary.json"), data)
}
