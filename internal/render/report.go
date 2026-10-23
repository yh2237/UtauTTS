package render

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

type UnitRenderer interface {
	ProviderID() string
	Render(*plan.Plan, Config) (*UnitRenderResult, error)
}

type UnitRenderResult struct {
	Audio  *audio.PCM
	Report RenderReport
}

type RenderReport struct {
	TargetF0            *F0Track `json:"target_f0,omitempty"`
	Provider            string
	LeadingMarginMS     float64
	CVVCTiming          string
	CVVCTransitionGain  float64
	CVVCPreBoundaryFade bool
	Diagnostics         []RenderDiagnostic `json:"diagnostics,omitempty"`
	Units               []UnitRenderReport
}

type RenderDiagnostic struct {
	Severity string `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

type UnitRenderReport struct {
	SpeechMapping           string
	SpeechSourceAnchorsMS   []float64
	SpeechTargetAnchorsMS   []float64
	WorldRenderMode         plan.WorldRenderMode
	WorldRenderReason       string
	WorldGapRepairEligible  bool
	WorldGapRepairReason    string
	SpeechJoinApplied       bool
	SpeechTransitionApplied bool
	SpeechRetimeApplied     bool
	StopBurstApplied        bool
	StopBurstGain           float64
	StopBurstReason         string
	CVTimingApplied         bool
	CVTimingWarnings        []string
	StretchAdapted          bool
	StretchLimitReason      string
	CodaBoundaryLimited     bool
	CodaClosureMS           float64
	CodaReleaseMS           float64
	CodaReleaseSeparated    bool
	Index                   int
	TimingScale             float64
	EffectivePreutteranceMS float64
	EffectiveConsonantMS    float64
	EffectiveOverlapMS      float64
	SourceF0Hz              float64
	TargetF0Hz              float64
	IntonationFactor        float64
	ResamplerVolume         int
}

type builtinUnitRenderer struct {
	provider string
}

func (renderer builtinUnitRenderer) ProviderID() string {
	return renderer.provider
}

func (renderer builtinUnitRenderer) Render(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	workingPlan := plan.Clone(synthesisPlan)
	cfg.Backend = string(renderer.provider)
	targetF0 := &F0Track{}
	cfg.TargetF0 = targetF0
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

func UnitRendererForProvider(provider string) (UnitRenderer, error) {
	if provider == "" {
		provider = base.DefaultBackend
	}
	if _, found := base.RendererImplementation(provider); !found {
		return nil, fmt.Errorf("unknown unit renderer provider %q", provider)
	}
	return builtinUnitRenderer{provider: provider}, nil
}

// RenderWithReportはPlanのコピーを描画する。
func RenderWithReport(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	renderer, err := UnitRendererForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return renderer.Render(synthesisPlan, cfg)
}

func UnitRendererForConfig(cfg Config) (UnitRenderer, error) {
	return UnitRendererForProvider(cfg.Backend)
}

func reportFromPlan(provider string, synthesisPlan *plan.Plan) RenderReport {
	report := RenderReport{Provider: provider}
	if synthesisPlan == nil {
		return report
	}
	report.LeadingMarginMS = synthesisPlan.LeadingMarginMS
	report.CVVCTiming = synthesisPlan.CVVCTiming
	report.CVVCTransitionGain = synthesisPlan.CVVCTransitionGain
	report.CVVCPreBoundaryFade = synthesisPlan.CVVCPreBoundaryFade
	report.Units = make([]UnitRenderReport, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		report.Units[index] = UnitRenderReport{
			SpeechMapping:           unit.SpeechMapping,
			SpeechSourceAnchorsMS:   append([]float64(nil), unit.SpeechSourceAnchorsMS...),
			SpeechTargetAnchorsMS:   append([]float64(nil), unit.SpeechTargetAnchorsMS...),
			WorldRenderMode:         unit.WorldRenderMode,
			WorldRenderReason:       unit.WorldRenderReason,
			WorldGapRepairEligible:  unit.WorldGapRepairEligible,
			WorldGapRepairReason:    unit.WorldGapRepairReason,
			SpeechJoinApplied:       unit.SpeechJoinApplied,
			SpeechTransitionApplied: unit.SpeechTransitionApplied,
			SpeechRetimeApplied:     unit.SpeechRetimeApplied,
			StopBurstApplied:        unit.StopBurstApplied,
			StopBurstGain:           unit.StopBurstGain,
			StopBurstReason:         unit.StopBurstReason,
			CVTimingApplied:         unit.CVTimingApplied,
			CVTimingWarnings:        append([]string(nil), unit.CVTimingWarnings...),
			StretchAdapted:          unit.StretchAdapted,
			StretchLimitReason:      unit.StretchLimitReason,
			CodaBoundaryLimited:     unit.CodaBoundaryLimited,
			CodaClosureMS:           unit.CodaClosureMS,
			CodaReleaseMS:           unit.CodaReleaseMS,
			CodaReleaseSeparated:    unit.CodaReleaseSeparated,
			Index:                   index,
			TimingScale:             unit.TimingScale,
			EffectivePreutteranceMS: unit.EffectivePreutteranceMS,
			EffectiveConsonantMS:    unit.EffectiveConsonantMS,
			EffectiveOverlapMS:      unit.EffectiveOverlapMS,
			SourceF0Hz:              unit.SourceF0Hz,
			TargetF0Hz:              unit.TargetF0Hz,
			IntonationFactor:        unit.IntonationFactor,
			ResamplerVolume:         unit.ResamplerVolume,
		}
	}
	return report
}

func (report RenderReport) ApplyTo(synthesisPlan *plan.Plan) {
	if synthesisPlan == nil {
		return
	}
	synthesisPlan.LeadingMarginMS = report.LeadingMarginMS
	synthesisPlan.CVVCTiming = report.CVVCTiming
	synthesisPlan.CVVCTransitionGain = report.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = report.CVVCPreBoundaryFade
	for _, unitReport := range report.Units {
		if unitReport.Index < 0 || unitReport.Index >= len(synthesisPlan.Units) {
			continue
		}
		unit := &synthesisPlan.Units[unitReport.Index]
		unit.SpeechMapping = unitReport.SpeechMapping
		unit.SpeechSourceAnchorsMS = append([]float64(nil), unitReport.SpeechSourceAnchorsMS...)
		unit.SpeechTargetAnchorsMS = append([]float64(nil), unitReport.SpeechTargetAnchorsMS...)
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
		unit.StretchAdapted = unitReport.StretchAdapted
		unit.StretchLimitReason = unitReport.StretchLimitReason
		unit.CodaBoundaryLimited = unitReport.CodaBoundaryLimited
		unit.CodaClosureMS = unitReport.CodaClosureMS
		unit.CodaReleaseMS = unitReport.CodaReleaseMS
		unit.CodaReleaseSeparated = unitReport.CodaReleaseSeparated
		unit.SpeechJoinApplied = unitReport.SpeechJoinApplied
		unit.SpeechTransitionApplied = unitReport.SpeechTransitionApplied
		unit.TimingScale = unitReport.TimingScale
		unit.EffectivePreutteranceMS = unitReport.EffectivePreutteranceMS
		unit.EffectiveConsonantMS = unitReport.EffectiveConsonantMS
		unit.EffectiveOverlapMS = unitReport.EffectiveOverlapMS
		unit.SourceF0Hz = unitReport.SourceF0Hz
		unit.TargetF0Hz = unitReport.TargetF0Hz
		unit.IntonationFactor = unitReport.IntonationFactor
		unit.ResamplerVolume = unitReport.ResamplerVolume
	}
}
