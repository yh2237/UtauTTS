// datasetパッケージはUTAU音源から学習例を構築する。
package dataset

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"utautts/internal/connection"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

const SchemaVersion = 3

type ConnectionConfig struct {
	NegativesPerPositive int
	Limit                int
	NegativeStrategy     string
}

type EntryReference struct {
	Alias          string  `json:"alias"`
	AliasKey       string  `json:"alias_key"`
	Source         string  `json:"source"`
	OtoPath        string  `json:"oto_path"`
	Line           int     `json:"line"`
	OffsetMS       float64 `json:"offset_ms"`
	PreutteranceMS float64 `json:"preutterance_ms"`
	OverlapMS      float64 `json:"overlap_ms"`
}

type ConnectionRecord struct {
	SchemaVersion    int                         `json:"schema_version"`
	RecordID         string                      `json:"record_id"`
	Voicebank        string                      `json:"voicebank"`
	GroupID          string                      `json:"group_id"`
	Label            int                         `json:"label"`
	LabelSource      string                      `json:"label_source,omitempty"`
	Weight           float64                     `json:"weight,omitempty"`
	Provenance       string                      `json:"provenance"`
	Previous         EntryReference              `json:"previous"`
	Current          EntryReference              `json:"current"`
	Features         connection.LearningFeatures `json:"features"`
	HandcraftedScore float64                     `json:"handcrafted_score"`
}

type ConnectionReport struct {
	SchemaVersion          int    `json:"schema_version"`
	Voicebank              string `json:"voicebank"`
	Entries                int    `json:"entries"`
	NaturalPairs           int    `json:"natural_pairs"`
	PositiveRecords        int    `json:"positive_records"`
	NegativeRecords        int    `json:"negative_records"`
	PairsWithoutNegative   int    `json:"pairs_without_negative"`
	InvalidPositiveRecords int    `json:"invalid_positive_records"`
	InvalidNegativeRecords int    `json:"invalid_negative_records"`
	NegativeStrategy       string `json:"negative_strategy"`
}

type naturalPair struct {
	left, right oto.Entry
	groupID     string
}

// BuildConnectionsは同一録音の隣接ユニットを弱い正例として使う。
// 負例は右側のaliasクラスを保ったまま別のWAVから取る。
func BuildConnections(bank *voicebank.Bank, config ConnectionConfig) ([]ConnectionRecord, ConnectionReport) {
	if config.NegativesPerPositive < 0 {
		config.NegativesPerPositive = 0
	}
	if config.NegativeStrategy == "" {
		config.NegativeStrategy = "hard"
	}
	entries := sortedEntries(bank)
	pairs := naturalPairs(entries, bank.Root)
	if config.Limit > 0 && len(pairs) > config.Limit {
		pairs = pairs[:config.Limit]
	}
	report := ConnectionReport{SchemaVersion: SchemaVersion, Voicebank: bank.Name, Entries: len(entries), NaturalPairs: len(pairs), NegativeStrategy: config.NegativeStrategy}
	byAlias := map[string][]oto.Entry{}
	for _, entry := range entries {
		key := AliasKey(entry.Alias)
		byAlias[key] = append(byAlias[key], entry)
	}
	extractor := connection.NewExtractor()
	records := make([]ConnectionRecord, 0, len(pairs)*(config.NegativesPerPositive+1))
	for pairIndex, pair := range pairs {
		positive := makeRecord(bank, extractor, pair.groupID, 1, "natural_continuation", pair.left, pair.right)
		records = append(records, positive)
		report.PositiveRecords++
		if invalidFeatures(positive.Features) {
			report.InvalidPositiveRecords++
		}

		candidates := negativeCandidates(byAlias[AliasKey(pair.right.Alias)], pair)
		if config.NegativeStrategy == "hard" {
			sort.SliceStable(candidates, func(i, j int) bool {
				leftScore := connection.HandcraftedScore(extractor.Pair(pair.left, candidates[i]))
				rightScore := connection.HandcraftedScore(extractor.Pair(pair.left, candidates[j]))
				return leftScore > rightScore
			})
		} else if config.NegativeStrategy != "rotating" {
			config.NegativeStrategy = "rotating"
			report.NegativeStrategy = "rotating"
		}
		if len(candidates) == 0 && config.NegativesPerPositive > 0 {
			report.PairsWithoutNegative++
			continue
		}
		count := min(config.NegativesPerPositive, len(candidates))
		start := 0
		if config.NegativeStrategy == "rotating" && len(candidates) > 0 {
			start = pairIndex % len(candidates)
		}
		for index := 0; index < count; index++ {
			current := candidates[(start+index)%len(candidates)]
			record := makeRecord(bank, extractor, pair.groupID, 0, "replaced_right", pair.left, current)
			records = append(records, record)
			report.NegativeRecords++
			if invalidFeatures(record.Features) {
				report.InvalidNegativeRecords++
			}
		}
	}
	return records, report
}

func sortedEntries(bank *voicebank.Bank) []oto.Entry {
	seen := map[oto.Entry]bool{}
	var result []oto.Entry
	for _, candidates := range bank.Entries {
		for _, entry := range candidates {
			if !seen[entry] {
				seen[entry] = true
				result = append(result, entry)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Filename != result[j].Filename {
			return result[i].Filename < result[j].Filename
		}
		if result[i].Offset != result[j].Offset {
			return result[i].Offset < result[j].Offset
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Alias < result[j].Alias
	})
	return result
}

func naturalPairs(entries []oto.Entry, root string) []naturalPair {
	var result []naturalPair
	for index := 1; index < len(entries); index++ {
		left, right := entries[index-1], entries[index]
		if !connection.SameSource(left.Filename, right.Filename) || right.Offset <= left.Offset {
			continue
		}
		source, err := filepath.Rel(root, right.Filename)
		if err != nil {
			source = right.Filename
		}
		result = append(result, naturalPair{
			left: left, right: right,
			groupID: fmt.Sprintf("%s:%d-%d", filepath.ToSlash(source), left.Line, right.Line),
		})
	}
	return result
}

func negativeCandidates(entries []oto.Entry, pair naturalPair) []oto.Entry {
	result := make([]oto.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry == pair.right || connection.SameSource(entry.Filename, pair.left.Filename) {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func makeRecord(bank *voicebank.Bank, extractor *connection.Extractor, groupID string, label int, provenance string, left, right oto.Entry) ConnectionRecord {
	features := extractor.Pair(left, right)
	return ConnectionRecord{
		SchemaVersion: SchemaVersion, RecordID: recordID(bank.Root, groupID, left, right), Voicebank: bank.Name, GroupID: groupID,
		Label: label, LabelSource: "weak_recording_continuity", Weight: 1, Provenance: provenance,
		Previous: reference(bank.Root, left), Current: reference(bank.Root, right),
		Features: connection.ToLearningFeatures(features), HandcraftedScore: connection.HandcraftedScore(features),
	}
}

func recordID(root, groupID string, left, right oto.Entry) string {
	leftRef, rightRef := reference(root, left), reference(root, right)
	return fmt.Sprintf("%s|%s:%d|%s:%d", groupID, leftRef.Source, leftRef.Line, rightRef.Source, rightRef.Line)
}

func reference(root string, entry oto.Entry) EntryReference {
	source, err := filepath.Rel(root, entry.Filename)
	if err != nil {
		source = entry.Filename
	}
	otoPath, err := filepath.Rel(root, entry.OtoPath)
	if err != nil {
		otoPath = entry.OtoPath
	}
	return EntryReference{
		Alias: entry.Alias, AliasKey: AliasKey(entry.Alias), Source: filepath.ToSlash(source),
		OtoPath: filepath.ToSlash(otoPath), Line: entry.Line, OffsetMS: entry.Offset,
		PreutteranceMS: entry.Preutterance, OverlapMS: entry.Overlap,
	}
}

func invalidFeatures(features connection.LearningFeatures) bool {
	return !features.Valid()
}

// AliasKeyはCVとVCVのaliasからモーラを含む共通末尾を抽出する。
func AliasKey(alias string) string {
	fields := strings.Fields(strings.TrimSpace(alias))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
