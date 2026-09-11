package render

import (
	"fmt"
	"math"
	"utautts/internal/plan"
)

// 原音範囲とF0を変えずフェード時間だけを調整する。
func wordBoundaryEnvelope(p *plan.Plan, u plan.Unit, points []worldlineEnvelopePoint) ([]worldlineEnvelopePoint, string) {
	if len(points) != 5 || u.Position < 0 || u.Position >= len(p.Morae) {
		return points, ""
	}
	boundary := func(left, right int) bool {
		if left < 0 || right >= len(p.Morae) {
			return false
		}
		a, b := p.Morae[left], p.Morae[right]
		return !a.Pause && !b.Pause && a.WordIndex != b.WordIndex
	}
	incoming, outgoing := boundary(u.Position-1, u.Position), boundary(u.Position, u.Position+1)
	// 遷移音は対象モーラの前、語末子音は後ろに置かれる。
	if u.Role == "transition" {
		outgoing = false
	}
	if u.Role != "transition" && u.Role != "mora" {
		incoming = false
	}
	if !incoming && !outgoing {
		return points, ""
	}
	r := append([]worldlineEnvelopePoint(nil), points...)
	beforeIn, beforeOut := r[1].XMS-r[0].XMS, r[4].XMS-r[3].XMS
	if incoming {
		r[1].XMS = r[0].XMS + math.Min(beforeIn, math.Max(5, beforeIn*.5))
	}
	if outgoing {
		r[3].XMS = r[4].XMS - math.Min(beforeOut, math.Max(5, beforeOut*.5))
	}
	if r[1].XMS == points[1].XMS && r[3].XMS == points[3].XMS {
		return points, ""
	}
	return r, fmt.Sprintf("fade-in %.3f->%.3f ms; fade-out %.3f->%.3f ms", beforeIn, r[1].XMS-r[0].XMS, beforeOut, r[4].XMS-r[3].XMS)
}
