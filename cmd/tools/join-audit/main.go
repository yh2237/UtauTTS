// join-audit exports acoustic features for the boundaries in a synthesis plan.
// The output is intended for listening-label collection and model training.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"utautts/internal/connection"
	"utautts/internal/oto"
	"utautts/internal/plan"
)

const maximumJoinGapMS = 80.0

func main() {
	planPath := flag.String("plan", "", "synthesis plan JSON")
	outPath := flag.String("out", "", "output JSON path (default: stdout)")
	joinModelPath := flag.String("join-model", "", "optional join model JSON to include predictions")
	flag.Parse()
	if *planPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	synthesisPlan, err := readPlan(*planPath)
	if err != nil {
		fail(err)
	}
	var model *connection.JoinModel
	if *joinModelPath != "" {
		model, err = connection.LoadJoinModel(*joinModelPath)
		if err != nil {
			fail(err)
		}
	}
	rows := auditPlan(synthesisPlan, model)
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *outPath == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fail(err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil && filepath.Dir(*outPath) != "." {
		fail(err)
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "join audit: %d boundaries -> %s\n", len(rows), *outPath)
}

func readPlan(path string) (*plan.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var synthesisPlan plan.Plan
	if err := json.Unmarshal(data, &synthesisPlan); err != nil {
		return nil, fmt.Errorf("decode plan %s: %w", path, err)
	}
	if len(synthesisPlan.Units) == 0 {
		return nil, fmt.Errorf("plan %s contains no units", path)
	}
	return &synthesisPlan, nil
}

func auditPlan(synthesisPlan *plan.Plan, model *connection.JoinModel) []connection.JoinAuditRow {
	if synthesisPlan == nil {
		return nil
	}
	extractor := connection.NewExtractorWithModel(model)
	rows := make([]connection.JoinAuditRow, 0, len(synthesisPlan.Units)-1)
	var previous *plan.Unit
	previousIndex := -1
	for index := range synthesisPlan.Units {
		current := &synthesisPlan.Units[index]
		if current.Silent || current.Source == "" {
			previous = nil
			previousIndex = -1
			continue
		}
		if previous == nil {
			previous = current
			previousIndex = index
			continue
		}
		gap := current.NoteStartMS - (previous.NoteStartMS + previous.DurationMS)
		if gap > maximumJoinGapMS {
			previous = current
			previousIndex = index
			continue
		}
		features := extractor.Pair(entryFromUnit(*previous), entryFromUnit(*current))
		prediction := connection.JoinPrediction{Baseline: connection.HandcraftedScore(features), Score: connection.HandcraftedScore(features)}
		if model != nil {
			prediction = model.Predict(features)
		}
		row := connection.JoinAuditRow{
			SchemaVersion:    connection.JoinAuditSchemaVersion,
			GroupID:          fmt.Sprintf("%d-%d", previousIndex, index),
			Voicebank:        synthesisPlan.Voicebank,
			Reading:          synthesisPlan.Reading,
			Language:         synthesisPlan.Language,
			JoinCostMode:     synthesisPlan.JoinCostMode,
			Previous:         auditUnit(*previous, previousIndex),
			Current:          auditUnit(*current, index),
			GapMS:            gap,
			Features:         features,
			HandcraftedScore: prediction.Baseline,
			RiskFlags:        connection.JoinRiskFlags(features),
		}
		if model != nil {
			row.ModelScore = floatPointer(prediction.Score)
			row.ModelProbability = floatPointer(prediction.Probability)
			row.ModelConfidence = floatPointer(prediction.Confidence)
			row.ModelApplied = prediction.Applied
			row.RiskFlags = append(row.RiskFlags, modelRiskFlags(prediction)...)
		}
		rows = append(rows, row)
		previous = current
		previousIndex = index
	}
	return rows
}

func floatPointer(value float64) *float64 {
	return &value
}

func modelRiskFlags(prediction connection.JoinPrediction) []string {
	if prediction.Applied {
		return []string{"learned-applied"}
	}
	return []string{"learned-fallback"}
}

func entryFromUnit(unit plan.Unit) oto.Entry {
	return oto.Entry{
		Filename: unit.Source, Alias: unit.Alias, Offset: unit.OffsetMS,
		Fixed: unit.ConsonantMS, Blank: unit.CutoffMS,
		Preutterance: unit.PreutteranceMS, Overlap: unit.OverlapMS,
		OtoPath: unit.OtoPath, Line: unit.OtoLine,
	}
}

func auditUnit(unit plan.Unit, index int) connection.JoinAuditUnit {
	return connection.JoinAuditUnit{
		Index: index, Position: unit.Position, Role: unit.Role,
		Mora: unit.Mora, Alias: unit.Alias, AliasKind: unit.AliasKind,
		Source: unit.Source, OtoPath: unit.OtoPath, OtoLine: unit.OtoLine,
		NoteStartMS: unit.NoteStartMS, DurationMS: unit.DurationMS,
		OffsetMS: unit.OffsetMS, ConsonantMS: unit.ConsonantMS,
		CutoffMS: unit.CutoffMS, PreutteranceMS: unit.PreutteranceMS,
		OverlapMS: unit.OverlapMS, FallbackTier: unit.FallbackTier,
		CandidateCount: unit.CandidateCount,
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
