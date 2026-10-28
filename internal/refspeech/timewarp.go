package refspeech

import (
	"math"

	"github.com/yh2237/UtauIDX/refmatch"
)

// Warpは参照の音声の時間軸を、出力の時間軸へ引き伸ばす対応。
// SourceとTargetは、発話の始まりからの累積の長さ（ミリ秒）。区間ごとに直線で結ぶ。
type Warp struct {
	Source []float64
	Target []float64
}

// NewWarpは、参照の発話区間の長さと、出力したい区間の長さから対応を作る。
// 長さが0以下の区間があればnilを返す。
func NewWarp(source, target []float64) *Warp {
	if len(source) == 0 || len(source) != len(target) {
		return nil
	}
	warp := &Warp{
		Source: make([]float64, len(source)+1),
		Target: make([]float64, len(target)+1),
	}
	for i, v := range source {
		if v <= 0 {
			return nil
		}
		warp.Source[i+1] = warp.Source[i] + v
	}
	for i, v := range target {
		if v <= 0 {
			return nil
		}
		warp.Target[i+1] = warp.Target[i] + v
	}
	return warp
}

// Mapは、発話の始まりからの出力の時刻に対応する、参照の時刻を返す。
// 発話の区間の外は、前後の無音の長さを変えずに移す。
func (w *Warp) Map(targetMS float64) float64 {
	total := w.Target[len(w.Target)-1]
	if targetMS <= 0 {
		return targetMS
	}
	if targetMS >= total {
		return targetMS + w.Source[len(w.Source)-1] - total
	}
	low, high := 0, len(w.Target)-1
	for low+1 < high {
		mid := (low + high) / 2
		if w.Target[mid] <= targetMS {
			low = mid
		} else {
			high = mid
		}
	}
	span := w.Target[high] - w.Target[low]
	if span <= 0 {
		return w.Source[low]
	}
	ratio := (targetMS - w.Target[low]) / span
	return w.Source[low] + (w.Source[high]-w.Source[low])*ratio
}

// WarpF0は、5ミリ秒ごとのF0を出力の時間軸へ引き伸ばす。プレビューの抑揚に使う。
func WarpF0(warp *Warp, leadingMS float64, f0 []float64) []float64 {
	if warp == nil || len(f0) == 0 {
		return f0
	}
	worldFrameMS := refmatch.WorldFrameMS()
	length := float64(len(f0))*worldFrameMS + warp.Target[len(warp.Target)-1] - warp.Source[len(warp.Source)-1]
	rows := int(math.Round(length / worldFrameMS))
	if rows <= 0 {
		return f0
	}
	out := make([]float64, rows)
	for i := range rows {
		at := (float64(i)+0.5)*worldFrameMS - leadingMS
		source := leadingMS + warp.Map(at)
		row := max(0, min(int(source/worldFrameMS), len(f0)-1))
		out[i] = f0[row]
	}
	return out
}

// warpSequenceは、特徴量・F0・音量を出力の時間軸へ引き伸ばす。
func warpSequence(warp *Warp, leadingMS float64, features []float32, f0, levels []float64, dims int) ([]float32, []float64, []float64, int) {
	subFrames := int(refmatch.FrameMS / refmatch.WorldFrameMS())
	sourceFrames := len(features) / dims
	if warp == nil || sourceFrames == 0 || len(f0) < sourceFrames*subFrames {
		return features, f0, levels, sourceFrames
	}
	worldFrameMS := refmatch.WorldFrameMS()
	sourceLength := float64(len(f0)) / float64(subFrames) * refmatch.FrameMS
	outLength := sourceLength + warp.Target[len(warp.Target)-1] - warp.Source[len(warp.Source)-1]
	outFrames := int(math.Round(outLength / refmatch.FrameMS))
	if outFrames <= 0 {
		return features, f0, levels, sourceFrames
	}
	outFeatures := make([]float32, outFrames*dims)
	for i := range outFrames {
		at := (float64(i)+0.5)*refmatch.FrameMS - leadingMS
		source := leadingMS + warp.Map(at)
		frame := max(0, min(int(source/refmatch.FrameMS), sourceFrames-1))
		copy(outFeatures[i*dims:(i+1)*dims], features[frame*dims:(frame+1)*dims])
	}
	rows := outFrames * subFrames
	outF0 := make([]float64, rows)
	outLevels := make([]float64, rows)
	for i := range rows {
		at := (float64(i)+0.5)*worldFrameMS - leadingMS
		source := leadingMS + warp.Map(at)
		row := max(0, min(int(source/worldFrameMS), len(f0)-1))
		outF0[i] = f0[row]
		if len(levels) > row {
			outLevels[i] = levels[row]
		}
	}
	return outFeatures, outF0, outLevels, outFrames
}
