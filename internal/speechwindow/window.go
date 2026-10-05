// Package speechwindowは、検出した有声・破裂の過渡区間を保護するときの広がりを共有する。
package speechwindow

import "math"

// TransientLeadMSは過渡区間の開始を基準位置より前へ広げる量。
const TransientLeadMS = 4.0

// TransientTailMSは過渡区間の終端を基準位置から後ろへ広げる量。長さが無効ならfallbackMSを返す。
func TransientTailMS(durationMS, fallbackMS float64) float64 {
	if durationMS <= 0 || math.IsNaN(durationMS) || math.IsInf(durationMS, 0) {
		return fallbackMS
	}
	return math.Max(10, math.Min(24, durationMS+6))
}
