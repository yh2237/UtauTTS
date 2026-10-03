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

// singleCVLegatoは、単独音の音源で母音だけのモーラが有声のモーラに続く場合を表す。
// 母音どうしをつなぐ原音（「a い」など）が無い音源では、単独の母音を立ち上がりからやり直すため、
// 発音が一つずつ独立して聞こえ、立ち上がりの前に短い切れ目もできる。
// 母音の連続のエイリアスを持つ音源では、そちらが選ばれる（連続音のユニット）ので変わらない。
func singleCVLegato(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	return synthesisPlan != nil && synthesisPlan.SingleCV && unit.Role == "mora" && !unit.Silent && !base.IsVCVUnit(unit) &&
		base.SingleCVOnset(synthesisPlan, unit) == "" && base.SingleCVMoraBoundaryEligible(synthesisPlan, unit.Position)
}

// singleCVLegatoAnchorsは、ユニットの頭（先行発声の区間）を母音の安定した部分から始める対応点を返す。
// 頭は原音の進みを半分にして安定部を鳴らし、以降は残りの母音を要求長へ伸縮する。
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
