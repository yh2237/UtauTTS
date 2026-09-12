package connection

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// JoinAuditUnit identifies one selected unit without depending on plan.Plan.
// Keeping this DTO in connection lets both the audit command and the trainer
// use the same file format without creating an import cycle.
type JoinAuditUnit struct {
	Index          int     `json:"index"`
	Position       int     `json:"position"`
	Role           string  `json:"role"`
	Mora           string  `json:"mora"`
	Alias          string  `json:"alias"`
	AliasKind      string  `json:"alias_kind,omitempty"`
	Source         string  `json:"source"`
	OtoPath        string  `json:"oto_path,omitempty"`
	OtoLine        int     `json:"oto_line,omitempty"`
	NoteStartMS    float64 `json:"note_start_ms"`
	DurationMS     float64 `json:"duration_ms"`
	OffsetMS       float64 `json:"offset_ms"`
	ConsonantMS    float64 `json:"consonant_ms"`
	CutoffMS       float64 `json:"cutoff_ms"`
	PreutteranceMS float64 `json:"preutterance_ms"`
	OverlapMS      float64 `json:"overlap_ms"`
	FallbackTier   int     `json:"fallback_tier,omitempty"`
	CandidateCount int     `json:"candidate_count,omitempty"`
}

// JoinAuditRow is one adjacent boundary from a rendered plan. Label is nil
// until a listener marks the join, 1 means preferred/continuous and 0 means
// rejected/discontinuous.
type JoinAuditRow struct {
	SchemaVersion    int           `json:"schema_version"`
	GroupID          string        `json:"group_id"`
	Voicebank        string        `json:"voicebank,omitempty"`
	Reading          string        `json:"reading,omitempty"`
	Language         string        `json:"language,omitempty"`
	JoinCostMode     string        `json:"join_cost_mode,omitempty"`
	Previous         JoinAuditUnit `json:"previous"`
	Current          JoinAuditUnit `json:"current"`
	GapMS            float64       `json:"gap_ms"`
	Features         PairFeatures  `json:"features"`
	HandcraftedScore float64       `json:"handcrafted_score"`
	ModelScore       *float64      `json:"model_score,omitempty"`
	ModelProbability *float64      `json:"model_probability,omitempty"`
	ModelConfidence  *float64      `json:"model_confidence,omitempty"`
	ModelApplied     bool          `json:"model_applied,omitempty"`
	RiskFlags        []string      `json:"risk_flags,omitempty"`
	Label            *float64      `json:"label"`
}

const JoinAuditSchemaVersion = 1

// JoinRiskFlags are triage hints for listening and are deliberately not used
// as training labels. Thresholds are conservative and can be changed without
// changing the stored feature format.
func JoinRiskFlags(features PairFeatures) []string {
	flags := make([]string, 0, 5)
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		flags = append(flags, "missing-frame")
	}
	if features.SpectrumDelta >= 8 {
		flags = append(flags, "spectrum-jump")
	}
	if features.RMSDelta >= 6 {
		flags = append(flags, "level-jump")
	}
	if features.F0DeltaCents >= 80 {
		flags = append(flags, "pitch-jump")
	}
	if features.VoicingMismatch {
		flags = append(flags, "voicing-change")
	}
	if features.WaveformCorrelation < 0.2 {
		flags = append(flags, "low-correlation")
	}
	if features.SameSource && !features.ForwardInSource {
		flags = append(flags, "source-backtrack")
	}
	if features.CurrentVCV {
		flags = append(flags, "vcv-boundary")
	}
	return flags
}

// ReadJoinAuditRows reads the JSON array emitted by join-audit or one row per
// line JSONL files, which makes it easy to merge several listening sessions.
func ReadJoinAuditRows(path string) ([]JoinAuditRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(data)
	trimmed = bytes.TrimPrefix(trimmed, []byte{0xef, 0xbb, 0xbf})
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("join audit %s is empty", path)
	}
	if trimmed[0] == '[' {
		var rows []JoinAuditRow
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, fmt.Errorf("decode join audit %s: %w", path, err)
		}
		return validateAuditRows(rows, path)
	}
	if trimmed[0] == '{' {
		var row JoinAuditRow
		if err := json.Unmarshal(trimmed, &row); err == nil {
			return validateAuditRows([]JoinAuditRow{row}, path)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var rows []JoinAuditRow
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		data := bytes.TrimSpace(scanner.Bytes())
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		if len(data) == 0 || data[0] == '#' {
			continue
		}
		var row JoinAuditRow
		if err := json.Unmarshal(data, &row); err != nil {
			return nil, fmt.Errorf("decode join audit %s line %d: %w", path, line, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return validateAuditRows(rows, path)
}

func validateAuditRows(rows []JoinAuditRow, path string) ([]JoinAuditRow, error) {
	for index := range rows {
		if rows[index].SchemaVersion == 0 {
			rows[index].SchemaVersion = JoinAuditSchemaVersion
		}
		if rows[index].SchemaVersion != JoinAuditSchemaVersion {
			return nil, fmt.Errorf("join audit %s row %d has unsupported schema version %d", path, index+1, rows[index].SchemaVersion)
		}
	}
	return rows, nil
}
