package render

import "math"

// ConstrainPitchCurveは入力を保ったまま平滑化と変化量制限を適用する。外部編集した輪郭にも学習済み輪郭と同じ制限をかける。
func ConstrainPitchCurve(curve *PitchCurve, sigmaMS, maxCentsPer10MS float64) *PitchCurve {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return curve
	}
	result := &PitchCurve{FrameMS: curve.FrameMS, Cents: append([]float64(nil), curve.Cents...)}
	if sigmaMS > 0 {
		sigma := sigmaMS / curve.FrameMS
		radius := max(1, int(math.Ceil(3*sigma)))
		weights := make([]float64, 2*radius+1)
		for offset := -radius; offset <= radius; offset++ {
			weights[offset+radius] = math.Exp(-0.5 * math.Pow(float64(offset)/sigma, 2))
		}
		smoothed := make([]float64, len(result.Cents))
		for position := range result.Cents {
			sum, weightSum := 0.0, 0.0
			for offset := -radius; offset <= radius; offset++ {
				source := max(0, min(len(result.Cents)-1, position+offset))
				weight := weights[offset+radius]
				sum += result.Cents[source] * weight
				weightSum += weight
			}
			smoothed[position] = sum / weightSum
		}
		result.Cents = smoothed
	}
	maximumStep := maxCentsPer10MS * curve.FrameMS / 10
	if maximumStep > 0 {
		for position := 1; position < len(result.Cents); position++ {
			result.Cents[position] = math.Max(result.Cents[position-1]-maximumStep, math.Min(result.Cents[position-1]+maximumStep, result.Cents[position]))
		}
		for position := len(result.Cents) - 2; position >= 0; position-- {
			result.Cents[position] = math.Max(result.Cents[position+1]-maximumStep, math.Min(result.Cents[position+1]+maximumStep, result.Cents[position]))
		}
	}
	return result
}
