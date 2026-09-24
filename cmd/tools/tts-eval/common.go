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
	contextDuration           bool
	contextDurationStrength   float64
	boundaryTone              bool
	boundaryToneStrength      float64
	stretchAdapt              bool
	stretchAdaptStrength      float64
	pauseContext              bool
	pauseContextStrength      float64
	englishWeakForm           bool
	e2a, e2b                  bool
	timeout                   time.Duration
}

// synthesizeCaseはsynth.Requestを組み立ててService経由で描画する。
// 既定値はrenderer_settingsのcanonicalなspecテーブル（synth側）が解決する。
func synthesizeCase(p prompt, o caseOptions, catalog *plugin.Catalog) (*synth.Result, float64, error) {
	request := synth.Request{
		SpeechTiming:            o.speechTiming,
		Text:                    p.Text,
		Reading:                 p.Reading,
		Language:                p.Language,
		Phonemizer:              p.Phonemizer,
		VoicebankPath:           o.bank,
		Tone:                    "C4",
		AliasPolicy:             voicebank.AliasPolicy(o.aliasPolicy),
		Renderer:                o.rendererID,
		WordBoundaryEnvelope:    o.wordEnvelope,
		SpeechProsodyExperiment: o.experiment,
		MoraDurationMS:          o.moraMS,
		PauseDurationMS:         plan.DefaultPauseDurationMS,
		MoraDurationsMS:         p.MoraDurationsMS,
		PitchCurve:              p.PitchCurve,
		ModelPath:               o.prosodyModelPath,
		ApplyPitch:              o.applyPitch,
		IntonationStrength:      synth.DefaultIntonationStrength,
		ContextDuration:         o.contextDuration,
		ContextDurationStrength: o.contextDurationStrength,
		BoundaryTone:            o.boundaryTone,
		BoundaryToneStrength:    o.boundaryToneStrength,
		StretchAdapt:            o.stretchAdapt,
		StretchAdaptStrength:    o.stretchAdaptStrength,
		PauseContext:            o.pauseContext,
		PauseContextStrength:    o.pauseContextStrength,
		EnglishWeakForm:         o.englishWeakForm,
		Worldline:               render.WorldlineProviderOptions{MixMode: o.mix, GapRepairMode: o.gapRepair, E2A: &o.e2a, E2B: &o.e2b},
	}
	service := synth.NewService(catalog, o.rendererID, o.bridge, "", "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	started := time.Now()
	result, err := service.SynthesizeContext(ctx, request)
	elapsed := float64(time.Since(started).Microseconds()) / 1000
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
		case plan.WorldRenderModeV13Compatible:
			row.V13CompatibleUnits++
		case plan.WorldRenderModeAdaptive:
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
