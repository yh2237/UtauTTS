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
	levels := ReferenceLevels(samples, rate, rows)
	cut := quietCut(levels, rows)
	if cut[20] > 0.01 {
		t.Fatalf("loud row cut %.2f dB", cut[20])
	}
	// 無音の行は、参照のレベルとの差のぶんだけ下げる（上限まで）。
	want := math.Min(-quietLevelDB-levels[180], maxQuietCutDB)
	if math.Abs(cut[180]-want) > 0.01 {
		t.Fatalf("silent row cut %.2f dB, want %.2f", cut[180], want)
	}
	if cut[180] < 40 {
		t.Fatalf("silent row cut %.2f dB, want at least 40", cut[180])
	}
	if got := quietCut(nil, rows); got[180] != 0 {
		t.Fatal("cut without reference levels")
	}
}
