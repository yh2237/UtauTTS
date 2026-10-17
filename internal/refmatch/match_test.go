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
