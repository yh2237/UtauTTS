package base

import (
	"math"

	"utautts/internal/plan"
)

// PitchCurveはフレームごとのセント単位の音高カーブ。
type PitchCurve struct {
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

// CentsAtは時刻(ms)のセント値を線形補間で返す。範囲外は端の値を使う。
func (curve *PitchCurve) CentsAt(timeMS float64) float64 {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return 0
	}
	position := math.Max(0, timeMS) / curve.FrameMS
	left := int(math.Floor(position))
	if left >= len(curve.Cents)-1 {
		return curve.Cents[len(curve.Cents)-1]
	}
	progress := position - float64(left)
	return curve.Cents[left]*(1-progress) + curve.Cents[left+1]*progress
}

// F0Trackは有声判定前の目標F0。StartMSはPlan基準で負値は文頭余白を表す。0は無声。
type F0Track struct {
	StartMS float64   `json:"start_ms"`
	FrameMS float64   `json:"frame_ms"`
	Hz      []float64 `json:"hz"`
}

// EffectiveTimingは補正後の実効タイミング。renderer非依存の共有型。
type EffectiveTiming struct {
	PreutteranceMS float64
	ConsonantMS    float64
	OverlapMS      float64
	Scale          float64
	CVApplied      bool
	CVWarnings     []string
	// StretchAdaptedはC3aの伸縮上限を適用したことを示す。
	StretchAdapted     bool
	StretchLimitReason string
}

// RenderedUnitは境界補正で参照する描画済みユニット。
type RenderedUnit struct {
	Index        int
	Unit         plan.Unit
	Timing       EffectiveTiming
	Wave         []float64
	StartFrame   int
	FadeInFrames int
}

// WorldlineEnvelopePointはWORLDのエンベロープ点。
type WorldlineEnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

// OpenUtauPhoneTimingはOpenUTAU互換の音素時間。
type OpenUtauPhoneTiming struct {
	Preutter    float64
	Overlap     float64
	TailIntrude float64
	TailOverlap float64
	Overlapped  bool
	CodaLimited bool
}
