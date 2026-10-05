package worldline

import (
	"math"

	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
)

const (
	// singleCVLegatoMSは母音の連続を前の母音と重ねる長さ（先行発声も同じ長さに縮める）。
	singleCVLegatoMS = 60.0
	// singleCVLegatoSkipMSは単独の母音の立ち上がりを飛ばす長さ。
	singleCVLegatoSkipMS = 40.0
)

// 単独母音を立ち上がりから鳴らし直すと途切れるため、前の母音につなぐ。
func singleCVLegato(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	return synthesisPlan != nil && synthesisPlan.SingleCV && unit.Role == "mora" && !unit.Silent && !base.IsVCVUnit(unit) &&
		base.SingleCVOnset(synthesisPlan, unit) == "" && base.SingleCVMoraBoundaryEligible(synthesisPlan, unit.Position)
}

// 立ち上がりを飛ばし、母音の安定部から鳴らす。
func singleCVLegatoAnchors(sourceOnset, targetOnset, requiredLength, sourceDuration float64) []provider.SpeechAnchor {
	start := sourceOnset + singleCVLegatoSkipMS
	head := start + targetOnset*0.5
	end := sourceDuration - 15 // 解析はフレーム単位に丸め、oto.offsetの端数だけ後ろへずれる
	if targetOnset <= 0 || requiredLength <= targetOnset+10 || head+20 >= end || math.IsNaN(start+head+end) {
		return nil
	}
	return []provider.SpeechAnchor{
		{TargetMS: 0, SourceMS: start},
		{TargetMS: targetOnset, SourceMS: head},
		{TargetMS: requiredLength, SourceMS: end},
	}
}
