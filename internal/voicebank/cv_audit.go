package voicebank

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
)

// SingleCVAudit はボイスバンクの単独音向けタイミング情報を表す
// trim後の音声とoto.iniだけを使うためRenderer選択前にも利用できる
type SingleCVAudit struct {
	Voicebank             string         `json:"voicebank"`
	EntryCount            int            `json:"entry_count"`
	CVEntries             int            `json:"cv_entries"`
	InitialContextEntries int            `json:"initial_context_entries"`
	ContextVCVEntries     int            `json:"context_vcv_entries"`
	VCEntries             int            `json:"vc_entries"`
	SingleCVBank          bool           `json:"single_cv_bank"`
	WarningEntryCount     int            `json:"warning_entry_count"`
	Entries               []CVEntryAudit `json:"entries"`
}

type CVEntryAudit struct {
	Alias           string   `json:"alias"`
	Kind            string   `json:"kind"`
	InitialContext  bool     `json:"initial_context,omitempty"`
	Source          string   `json:"source"`
	OtoPath         string   `json:"oto_path"`
	OtoLine         int      `json:"oto_line"`
	OffsetMS        float64  `json:"offset_ms"`
	FixedMS         float64  `json:"fixed_ms"`
	BlankMS         float64  `json:"blank_ms"`
	PreutteranceMS  float64  `json:"preutterance_ms"`
	OverlapMS       float64  `json:"overlap_ms"`
	TrimmedLengthMS float64  `json:"trimmed_length_ms,omitempty"`
	VowelTailMS     float64  `json:"vowel_tail_ms,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

// AuditSingleCV は単独音の子音欠落や母音末尾欠落につながる値を監査する
func (b *Bank) AuditSingleCV() (*SingleCVAudit, error) {
	if b == nil {
		return nil, fmt.Errorf("voicebank is nil")
	}
	result := &SingleCVAudit{Voicebank: b.Root}
	aliases := b.Aliases()
	for _, alias := range aliases {
		entries := append([]oto.Entry(nil), b.Entries[alias]...)
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].OtoPath != entries[j].OtoPath {
				return entries[i].OtoPath < entries[j].OtoPath
			}
			return entries[i].Line < entries[j].Line
		})
		kind := ClassifyAlias(alias)
		for _, entry := range entries {
			result.EntryCount++
			switch {
			case IsInitialContextAlias(alias):
				result.InitialContextEntries++
			case IsContextVCVAlias(alias):
				result.ContextVCVEntries++
			case kind == AliasCV:
				result.CVEntries++
			case kind == AliasVCV:
				result.ContextVCVEntries++
			case kind == AliasVC:
				result.VCEntries++
			}
			row := CVEntryAudit{
				Alias: alias, Kind: string(kind), InitialContext: IsInitialContextAlias(alias),
				Source: relativePath(b.Root, entry.Filename), OtoPath: relativePath(b.Root, entry.OtoPath), OtoLine: entry.Line,
				OffsetMS: entry.Offset, FixedMS: entry.Fixed, BlankMS: entry.Blank,
				PreutteranceMS: entry.Preutterance, OverlapMS: entry.Overlap,
			}
			row.Warnings = append(row.Warnings, auditTimingWarnings(entry)...)
			if entry.Filename == "" {
				row.Warnings = append(row.Warnings, "missing-source")
			} else if pcm, err := audio.ReadWav(filepath.Clean(entry.Filename)); err != nil {
				row.Warnings = append(row.Warnings, "wav-read")
			} else if trimmed, err := audio.TrimPCM(pcm, entry.Offset, entry.Blank); err != nil {
				row.Warnings = append(row.Warnings, "trim-range")
			} else if pcm.SampleRate <= 0 || pcm.Channels <= 0 {
				row.Warnings = append(row.Warnings, "invalid-format")
			} else {
				frames := len(trimmed.Data) / trimmed.Channels
				row.TrimmedLengthMS = float64(frames) * 1000 / float64(trimmed.SampleRate)
				row.VowelTailMS = math.Max(0, row.TrimmedLengthMS-math.Max(0, entry.Fixed))
				if row.VowelTailMS < 40 {
					row.Warnings = append(row.Warnings, "short-vowel-tail")
				}
				if acoustic.RMS(acoustic.Mono(trimmed)) < 1e-5 {
					row.Warnings = append(row.Warnings, "trim-silent")
				}
			}
			row.Warnings = uniqueAuditWarnings(row.Warnings)
			if len(row.Warnings) > 0 {
				result.WarningEntryCount++
			}
			result.Entries = append(result.Entries, row)
		}
	}
	result.SingleCVBank = result.ContextVCVEntries == 0 && result.VCEntries == 0
	return result, nil
}

func auditTimingWarnings(entry oto.Entry) []string {
	warnings := make([]string, 0, 4)
	if entry.Preutterance > entry.Fixed+1 {
		warnings = append(warnings, "preutterance-after-fixed")
	}
	if entry.Overlap > entry.Preutterance+1 {
		warnings = append(warnings, "overlap-after-preutterance")
	}
	if entry.Preutterance > 120 {
		warnings = append(warnings, "long-preutterance")
	}
	if entry.Fixed > 200 {
		warnings = append(warnings, "long-fixed")
	}
	return warnings
}

func uniqueAuditWarnings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
