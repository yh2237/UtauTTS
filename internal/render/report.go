package render

import (
	"fmt"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
)

// UnitRenderer is the contract used by providers that consume a selected UTAU
// Unit Plan. Implementations receive an isolated copy and report diagnostics
// separately from the selection plan.
type UnitRenderer interface {
	ProviderID() engine.ProviderID
	Render(*plan.Plan, Config) (*UnitRenderResult, error)
}

// UnitRenderResult is the immutable-plan rendering result.
type UnitRenderResult struct {
	Audio  *audio.PCM
	Report RenderReport
}

// RenderReport contains values previously written back to plan.Plan by a
// renderer. It can be applied to an export copy for backward compatibility.
type RenderReport struct {
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

// RenderDiagnostic is a provider message retained separately from the
// selection Plan. It is useful for UI logs without making provider-specific
// fields part of the core Plan contract.
type RenderDiagnostic struct {
	Severity string `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

// UnitRenderReport contains renderer-derived diagnostics for one input unit.
type UnitRenderReport struct {
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
	pcm, err := renderMutable(workingPlan, cfg)
	if err != nil {
		return nil, err
	}
	return &UnitRenderResult{
		Audio:  pcm,
		Report: reportFromPlan(renderer.provider, workingPlan),
	}, nil
}

// UnitRendererForBackend returns the current built-in adapter for a legacy
// backend name. renderer.json v1 still supplies this value through its
// compatibility adapter.
func UnitRendererForBackend(backend string) (UnitRenderer, error) {
	if backend == "" {
		backend = "waveform"
	}
	if _, found := rendererImplementations[backend]; !found {
		return nil, fmt.Errorf("unknown unit renderer provider %q", backend)
	}
	return builtinUnitRenderer{provider: engine.ProviderID(backend)}, nil
}

// RenderWithReport renders an isolated copy of the input Plan. Unlike Render,
// it never writes renderer diagnostics into the caller's Plan.
func RenderWithReport(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	renderer, err := UnitRendererForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return renderer.Render(synthesisPlan, cfg)
}

// UnitRendererForConfig selects a built-in adapter or a manifest-declared
// external Provider. Keeping this decision at the UnitRenderer boundary lets
// the rest of the TTS pipeline remain unaware of process transport details.
func UnitRendererForConfig(cfg Config) (UnitRenderer, error) {
	if cfg.EngineDefinition != nil && cfg.EngineDefinition.Protocol == "utautts-provider" {
		if cfg.EngineDefinition.Contract != engine.ContractUnitRenderer {
			return nil, fmt.Errorf("external provider contract %q is not a unit-renderer", cfg.EngineDefinition.Contract)
		}
		return newExternalUnitRenderer(*cfg.EngineDefinition), nil
	}
	return UnitRendererForBackend(cfg.Backend)
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

// ApplyTo restores renderer diagnostics to an export copy of a Plan. It must
// not be used on the canonical selection plan held by tts.Result.
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
		unit.TimingScale = unitReport.TimingScale
		unit.EffectivePreutteranceMS = unitReport.EffectivePreutteranceMS
		unit.EffectiveConsonantMS = unitReport.EffectiveConsonantMS
		unit.EffectiveOverlapMS = unitReport.EffectiveOverlapMS
		unit.SourceF0Hz = unitReport.SourceF0Hz
		unit.TargetF0Hz = unitReport.TargetF0Hz
		unit.IntonationFactor = unitReport.IntonationFactor
	}
}
