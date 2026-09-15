package render

import (
	"fmt"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
)

// UnitRendererは選択済みUnit Planを描画する。
type UnitRenderer interface {
	ProviderID() engine.ProviderID
	Render(*plan.Plan, Config) (*UnitRenderResult, error)
}

// UnitRenderResultは音声と描画結果を保持する。
type UnitRenderResult struct {
	Audio  *audio.PCM
	Report RenderReport
}

// RenderReportは描画時に得た診断情報を保持する。
type RenderReport struct {
	TargetF0                *F0Track `json:"target_f0,omitempty"`
	Provider                engine.ProviderID
	LeadingMarginMS         float64
	BoundaryBridgeMS        float64
	BoundaryBridgeThreshold float64
	BoundaryBridges         []plan.BoundaryBridge
	BoundaryRepairDecisions []plan.BoundaryRepairDecision
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	Diagnostics             []RenderDiagnostic `json:"diagnostics,omitempty"`
	Units                   []UnitRenderReport
}

// F0Trackは有声判定前のWORLD用目標値。StartMSはPlan基準で負値は文頭余白を表す。0は無声。
type F0Track struct {
	StartMS float64   `json:"start_ms"`
	FrameMS float64   `json:"frame_ms"`
	Hz      []float64 `json:"hz"`
}

// RenderDiagnosticはproviderからの診断情報を示す。
type RenderDiagnostic struct {
	Severity string `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

// UnitRenderReportはunitごとの描画結果を示す。
type UnitRenderReport struct {
	WorldRenderMode         string
	WorldRenderReason       string
	WorldGapRepairEligible  bool
	WorldGapRepairReason    string
	SpeechJoinApplied       bool
	SpeechRetimeApplied     bool
	StopBurstApplied        bool
	StopBurstGain           float64
	StopBurstReason         string
	CVTimingApplied         bool
	CVTimingWarnings        []string
	BoundaryEnvelope        string
	Index                   int
	TimingScale             float64
	EffectivePreutteranceMS float64
	EffectiveConsonantMS    float64
	EffectiveOverlapMS      float64
	SourceF0Hz              float64
	TargetF0Hz              float64
	IntonationFactor        float64
}

type builtinUnitRenderer struct {
	provider engine.ProviderID
}

func (renderer builtinUnitRenderer) ProviderID() engine.ProviderID {
	return renderer.provider
}

func (renderer builtinUnitRenderer) Render(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	workingPlan := plan.Clone(synthesisPlan)
	cfg.Backend = string(renderer.provider)
	targetF0 := &F0Track{}
	cfg.targetF0 = targetF0
	pcm, err := renderMutable(workingPlan, cfg)
	if err != nil {
		return nil, err
	}
	result := &UnitRenderResult{
		Audio:  pcm,
		Report: reportFromPlan(renderer.provider, workingPlan),
	}
	if len(targetF0.Hz) > 0 {
		result.Report.TargetF0 = targetF0
	}
	return result, nil
}

// UnitRendererForProviderは組み込みrendererを返す。
func UnitRendererForProvider(provider string) (UnitRenderer, error) {
	if provider == "" {
		provider = "waveform"
	}
	if _, found := rendererImplementations[provider]; !found {
		return nil, fmt.Errorf("unknown unit renderer provider %q", provider)
	}
	return builtinUnitRenderer{provider: engine.ProviderID(provider)}, nil
}

// RenderWithReportはPlanのコピーを描画する。
func RenderWithReport(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	renderer, err := UnitRendererForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return renderer.Render(synthesisPlan, cfg)
}

// UnitRendererForConfigは設定に対応するrendererを返す。
func UnitRendererForConfig(cfg Config) (UnitRenderer, error) {
	definition := cfg.Engine.Definition
	if definition.Protocol == "utautts-provider" {
		if definition.Contract != engine.ContractUnitRenderer {
			return nil, fmt.Errorf("external provider contract %q is not a unit-renderer", definition.Contract)
		}
		return newExternalUnitRenderer(definition), nil
	}
	return UnitRendererForProvider(cfg.Backend)
}

func reportFromPlan(provider engine.ProviderID, synthesisPlan *plan.Plan) RenderReport {
	report := RenderReport{Provider: provider}
	if synthesisPlan == nil {
		return report
	}
	report.LeadingMarginMS = synthesisPlan.LeadingMarginMS
	report.BoundaryBridgeMS = synthesisPlan.BoundaryBridgeMS
	report.BoundaryBridgeThreshold = synthesisPlan.BoundaryBridgeThreshold
	report.BoundaryBridges = append([]plan.BoundaryBridge(nil), synthesisPlan.BoundaryBridges...)
	report.BoundaryRepairDecisions = append([]plan.BoundaryRepairDecision(nil), synthesisPlan.BoundaryRepairDecisions...)
	report.CVVCTiming = synthesisPlan.CVVCTiming
	report.CVVCTransitionGain = synthesisPlan.CVVCTransitionGain
	report.CVVCPreBoundaryFade = synthesisPlan.CVVCPreBoundaryFade
	report.Units = make([]UnitRenderReport, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		report.Units[index] = UnitRenderReport{
			WorldRenderMode:         unit.WorldRenderMode,
			WorldRenderReason:       unit.WorldRenderReason,
			WorldGapRepairEligible:  unit.WorldGapRepairEligible,
			WorldGapRepairReason:    unit.WorldGapRepairReason,
			SpeechJoinApplied:       unit.SpeechJoinApplied,
			SpeechRetimeApplied:     unit.SpeechRetimeApplied,
			StopBurstApplied:        unit.StopBurstApplied,
			StopBurstGain:           unit.StopBurstGain,
			StopBurstReason:         unit.StopBurstReason,
			CVTimingApplied:         unit.CVTimingApplied,
			CVTimingWarnings:        append([]string(nil), unit.CVTimingWarnings...),
			BoundaryEnvelope:        unit.BoundaryEnvelope,
			Index:                   index,
			TimingScale:             unit.TimingScale,
			EffectivePreutteranceMS: unit.EffectivePreutteranceMS,
			EffectiveConsonantMS:    unit.EffectiveConsonantMS,
			EffectiveOverlapMS:      unit.EffectiveOverlapMS,
			SourceF0Hz:              unit.SourceF0Hz,
			TargetF0Hz:              unit.TargetF0Hz,
			IntonationFactor:        unit.IntonationFactor,
		}
	}
	return report
}

// ApplyToは診断情報を出力用Planへ反映する。
func (report RenderReport) ApplyTo(synthesisPlan *plan.Plan) {
	if synthesisPlan == nil {
		return
	}
	synthesisPlan.LeadingMarginMS = report.LeadingMarginMS
	synthesisPlan.BoundaryBridgeMS = report.BoundaryBridgeMS
	synthesisPlan.BoundaryBridgeThreshold = report.BoundaryBridgeThreshold
	synthesisPlan.BoundaryBridges = append([]plan.BoundaryBridge(nil), report.BoundaryBridges...)
	synthesisPlan.BoundaryRepairDecisions = append([]plan.BoundaryRepairDecision(nil), report.BoundaryRepairDecisions...)
	synthesisPlan.CVVCTiming = report.CVVCTiming
	synthesisPlan.CVVCTransitionGain = report.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = report.CVVCPreBoundaryFade
	for _, unitReport := range report.Units {
		if unitReport.Index < 0 || unitReport.Index >= len(synthesisPlan.Units) {
			continue
		}
		unit := &synthesisPlan.Units[unitReport.Index]
		unit.WorldRenderMode = unitReport.WorldRenderMode
		unit.WorldRenderReason = unitReport.WorldRenderReason
		unit.WorldGapRepairEligible = unitReport.WorldGapRepairEligible
		unit.WorldGapRepairReason = unitReport.WorldGapRepairReason
		unit.SpeechRetimeApplied = unitReport.SpeechRetimeApplied
		unit.StopBurstApplied = unitReport.StopBurstApplied
		unit.StopBurstGain = unitReport.StopBurstGain
		unit.StopBurstReason = unitReport.StopBurstReason
		unit.CVTimingApplied = unitReport.CVTimingApplied
		unit.CVTimingWarnings = append([]string(nil), unitReport.CVTimingWarnings...)
		unit.BoundaryEnvelope = unitReport.BoundaryEnvelope
		unit.SpeechJoinApplied = unitReport.SpeechJoinApplied
		unit.TimingScale = unitReport.TimingScale
		unit.EffectivePreutteranceMS = unitReport.EffectivePreutteranceMS
		unit.EffectiveConsonantMS = unitReport.EffectiveConsonantMS
		unit.EffectiveOverlapMS = unitReport.EffectiveOverlapMS
		unit.SourceF0Hz = unitReport.SourceF0Hz
		unit.TargetF0Hz = unitReport.TargetF0Hz
		unit.IntonationFactor = unitReport.IntonationFactor
	}
}
