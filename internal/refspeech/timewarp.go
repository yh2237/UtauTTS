package refspeech

import (
	"math"

	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/vocoder"
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
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		warp.Source[i+1] = warp.Source[i] + v
		if math.IsInf(warp.Source[i+1], 0) {
			return nil
		}
	}
	for i, v := range target {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		warp.Target[i+1] = warp.Target[i] + v
		if math.IsInf(warp.Target[i+1], 0) {
			return nil
		}
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
	worldFrameMS := index.WorldFrameMS
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

// warpQuery maps the feature frames and WORLD rows to the same output timeline.
func warpQuery(warp *Warp, leadingMS float64, channels map[string][]float32, channelDims map[string]int, analysis *vocoder.ReferenceAnalysis) {
	if warp == nil || analysis.Frames == 0 {
		return
	}
	length := float64(analysis.Frames)*index.FrameMS + warp.Target[len(warp.Target)-1] - warp.Source[len(warp.Source)-1]
	frames := int(math.Round(length / index.FrameMS))
	if frames <= 0 {
		return
	}
	frameMap := warpRows(warp, leadingMS, index.FrameMS, analysis.Frames, frames)
	rowMap := warpRows(warp, leadingMS, index.WorldFrameMS, analysis.Frames*index.SubFrames, frames*index.SubFrames)
	for id, values := range channels {
		channels[id] = remap(values, frameMap, channelDims[id])
	}
	analysis.MouthStatic = remap(analysis.MouthStatic, frameMap, len(analysis.MouthStatic)/analysis.Frames)
	analysis.MouthDelta = remap(analysis.MouthDelta, frameMap, len(analysis.MouthDelta)/analysis.Frames)
	analysis.IsUV20 = remap(analysis.IsUV20, frameMap, 1)
	analysis.F0 = remap(analysis.F0, rowMap, 1)
	analysis.Levels = remap(analysis.Levels, rowMap, 1)
	analysis.SafeRefAP = remap(analysis.SafeRefAP, rowMap, 1)
	analysis.Frames = frames
}

func warpRows(warp *Warp, leadingMS, frameMS float64, sourceRows, targetRows int) []int {
	rows := make([]int, targetRows)
	for i := range rows {
		at := (float64(i)+0.5)*frameMS - leadingMS
		source := leadingMS + warp.Map(at)
		rows[i] = max(0, min(int(source/frameMS), sourceRows-1))
	}
	return rows
}

func remap[T any](values []T, rows []int, width int) []T {
	if len(values) == 0 {
		return nil
	}
	out := make([]T, len(rows)*width)
	for i, row := range rows {
		copy(out[i*width:(i+1)*width], values[row*width:(row+1)*width])
	}
	return out
}
