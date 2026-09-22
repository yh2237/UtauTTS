package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
	"utautts/internal/voicebank"
)

// caseOptionsは1ケースの合成条件。単一モードと掃引モードで共有する。
type caseOptions struct {
	bank, aliasPolicy, bridge string
	model, modelFile          string
	prosodyModelPath          string
	moraMS                    float64
	experiment                string
	wordEnvelope              bool
	rendererID                string
	mix, gapRepair            string
	speechTiming, applyPitch  bool
	timeout                   time.Duration
}

// synthesizeCaseはConfigを組み立ててApplyRendererから描画までを行う。
func synthesizeCase(p prompt, o caseOptions, catalog *plugin.Catalog) (*synth.Result, float64, error) {
	cfg := tts.Config{VoicebankPath: o.bank, Text: p.Text, Reading: p.Reading, Language: p.Language, Phonemizer: p.Phonemizer, Tone: "C4", MoraDurationMS: 120, PauseDurationMS: 180, ApplyPitch: o.applyPitch, IntonationStrength: synth.DefaultIntonationStrength}
	cfg.AliasPolicy = voicebank.AliasPolicy(o.aliasPolicy)
	cfg.SpeechTiming = o.speechTiming
	cfg.SpeechProsodyExperiment = o.experiment
	cfg.WordBoundaryEnvelope = o.wordEnvelope
	cfg.MoraDurationMS = o.moraMS
	cfg.MoraDurationsMS = p.MoraDurationsMS
	cfg.PitchCurve = p.PitchCurve
	cfg.ProsodyModelPath = o.prosodyModelPath
	resolved, err := tts.ApplyRenderer(&cfg, catalog, o.rendererID, o.bridge)
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	cfg.Context = ctx
	started := time.Now()
	var result *synth.Result
	if err == nil {
		providerOptions := render.ProviderOptions{Worldline: render.WorldlineProviderOptions{MixMode: o.mix, GapRepairMode: o.gapRepair}}
		result, err = synth.SynthesizeConfigWithOptions(cfg, resolved, providerOptions)
	}
	elapsed := float64(time.Since(started).Microseconds()) / 1000
	cancel()
	return result, elapsed, err
}

// selectionPlanUnitは選択と時間だけを抜き出した比較用の単位。描画メタや測定F0を含めない。
type selectionPlanUnit struct {
	Position       int     `json:"position"`
	Role           string  `json:"role"`
	Mora           string  `json:"mora"`
	Alias          string  `json:"alias"`
	AliasKind      string  `json:"alias_kind,omitempty"`
	Source         string  `json:"source"`
	Silent         bool    `json:"silent,omitempty"`
	OffsetMS       float64 `json:"offset_ms"`
	ConsonantMS    float64 `json:"consonant_ms"`
	CutoffMS       float64 `json:"cutoff_ms"`
	PreutteranceMS float64 `json:"preutterance_ms"`
	OverlapMS      float64 `json:"overlap_ms"`
	NoteStartMS    float64 `json:"note_start_ms"`
	DurationMS     float64 `json:"duration_ms"`
	FallbackTier   int     `json:"fallback_tier"`
	SubbankID      string  `json:"subbank_id,omitempty"`
	Color          string  `json:"color,omitempty"`
	ResolvedTone   string  `json:"resolved_tone,omitempty"`
}

// selectionPlanDigestは描画メタと測定F0を除いた選択計画の指紋。同じ選択なら同じ値になる。
func selectionPlanDigest(p *plan.Plan) string {
	if p == nil {
		return ""
	}
	units := make([]selectionPlanUnit, len(p.Units))
	for i, u := range p.Units {
		units[i] = selectionPlanUnit{
			Position: u.Position, Role: u.Role, Mora: u.Mora, Alias: u.Alias, AliasKind: u.AliasKind,
			Source: u.Source, Silent: u.Silent,
			OffsetMS: u.OffsetMS, ConsonantMS: u.ConsonantMS, CutoffMS: u.CutoffMS,
			PreutteranceMS: u.PreutteranceMS, OverlapMS: u.OverlapMS,
			NoteStartMS: u.NoteStartMS, DurationMS: u.DurationMS,
			FallbackTier: u.FallbackTier, SubbankID: u.SubbankID, Color: u.Color, ResolvedTone: u.ResolvedTone,
		}
	}
	payload := struct {
		DurationMS float64             `json:"duration_ms"`
		Units      []selectionPlanUnit `json:"units"`
	}{p.DurationMS, units}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

// fillMeasurementは結果から計測値を埋め、描画済みPlanを返す。
func fillMeasurement(row *measurement, result *synth.Result) *plan.Plan {
	row.AudioMS = result.DurationMS
	row.MissingPhoneGroups = len(result.Plan.MissingPhones)
	renderedPlan := result.RenderedPlan()
	for _, unit := range renderedPlan.Units {
		switch unit.WorldRenderMode {
		case "v1.3-compatible":
			row.V13CompatibleUnits++
		case "adaptive":
			row.AdaptiveUnits++
		}
		if unit.WorldGapRepairEligible {
			row.GapRepairUnits++
		}
		if unit.StopBurstApplied {
			row.StopBurstUnits++
			row.MeanStopBurstGain += unit.StopBurstGain
		}
		if unit.StopBurstReason == "transient-unreliable" {
			row.UnreliableTransientUnits++
		}
	}
	if row.StopBurstUnits > 0 {
		row.MeanStopBurstGain /= float64(row.StopBurstUnits)
	}
	if row.AudioMS > 0 {
		row.RTF = row.ElapsedMS / row.AudioMS
	}
	for _, s := range result.Audio.Data {
		x := float64(s) / 32768
		row.Peak = math.Max(row.Peak, math.Abs(x))
		row.RMS += x * x
	}
	if len(result.Audio.Data) > 0 {
		row.RMS = math.Sqrt(row.RMS / float64(len(result.Audio.Data)))
	}
	for _, u := range result.Plan.Units {
		if u.Silent {
			row.SilentUnits++
		}
	}
	return renderedPlan
}
