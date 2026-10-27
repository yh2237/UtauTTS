package refspeech

import (
	"math"
	"testing"
)

func TestWarpStretchesOneSegment(t *testing.T) {
	warp := NewWarp([]float64{100, 100, 100}, []float64{100, 300, 100})
	if warp == nil {
		t.Fatal("warp is nil")
	}
	// 2番目の区間の半分（出力250ms）は、参照の150msに対応する。
	if got := warp.Map(250); math.Abs(got-150) > 1e-6 {
		t.Fatalf("Map(250) = %v, want 150", got)
	}
	// 区間の後の無音は、長さを変えずにずれる。
	if got := warp.Map(500); math.Abs(got-300) > 1e-6 {
		t.Fatalf("Map(500) = %v, want 300", got)
	}
	if got := warp.Map(-20); got != -20 {
		t.Fatalf("Map(-20) = %v, want -20", got)
	}
}

func TestWarpRefusesBadSegments(t *testing.T) {
	if NewWarp([]float64{100, 0}, []float64{100, 100}) != nil {
		t.Fatal("zero-length segment should be refused")
	}
	if NewWarp([]float64{100}, []float64{100, 100}) != nil {
		t.Fatal("different lengths should be refused")
	}
}

func TestWarpSequenceStretchesFrames(t *testing.T) {
	const dims = 4
	source := make([]float32, 15*dims)
	for i := range 15 {
		for d := range dims {
			source[i*dims+d] = float32(i)
		}
	}
	f0 := make([]float64, 60)
	levels := make([]float64, 60)
	for i := range f0 {
		f0[i] = 100
	}
	warp := NewWarp([]float64{100, 100, 100}, []float64{100, 300, 100})
	features, outF0, outLevels, frames := warpSequence(warp, 0, source, f0, levels, dims)
	// 参照は300ms（15フレーム）、出力は500ms（25フレーム）。
	if frames != 25 {
		t.Fatalf("frames = %d, want 25", frames)
	}
	if len(features) != frames*dims || len(outF0) != frames*4 || len(outLevels) != frames*4 {
		t.Fatalf("lengths: features %d, f0 %d, levels %d", len(features), len(outF0), len(outLevels))
	}
	// 最初の区間は変わらない。
	if features[2*dims] != 2 {
		t.Fatalf("first segment frame = %v, want 2", features[2*dims])
	}
	// 2番目の区間は3倍に伸びるので、フレームの進みが遅くなる。
	if features[10*dims] != float32(6) {
		t.Fatalf("second segment frame = %v, want 6", features[10*dims])
	}
}
