package refmatch

import (
	"math"
	"testing"
)

func TestQuietCutFollowsReferenceSilence(t *testing.T) {
	const rate, rows = 24000, 200
	samples := make([]float64, rows*rate*int(worldFrameMS)/1000)
	for i := range samples[:len(samples)/2] {
		samples[i] = 0.3 * math.Sin(float64(i)*0.05)
	}
	cut := quietCut(ReferenceLevels(samples, rate, rows), rows)
	if cut[20] > 0.01 {
		t.Fatalf("loud row cut %.2f dB", cut[20])
	}
	if math.Abs(cut[180]-maxQuietCutDB) > 0.01 {
		t.Fatalf("silent row cut %.2f dB, want %.0f", cut[180], maxQuietCutDB)
	}
	if got := quietCut(nil, rows); got[180] != 0 {
		t.Fatal("cut without reference levels")
	}
}

func TestNearestUsesOnlyCandidates(t *testing.T) {
	index := &Index{
		Header:   Header{Dims: 2, Frames: 4},
		Features: []float32{1, 0, 1, 0, 0, 1, 0, 1},
		File:     []int32{0, 0, 1, 1},
		F0:       make([]float32, 4*subFrames),
	}
	features := []float32{1, 0}
	voiced := []bool{false}
	all := index.nearest(features, 1, voiced, nil)
	if len(all[0]) == 0 || index.File[all[0][0]] != 0 {
		t.Fatalf("unconstrained pick = %v", all[0])
	}
	only := index.nearest(features, 1, voiced, [][]int32{{2, 3}})
	if len(only[0]) == 0 || index.File[only[0][0]] != 1 {
		t.Fatalf("constrained pick = %v", only[0])
	}
}
