package jsut

import "testing"

func TestBuildPriorExcludesPausesAndAggregatesNaturalBoundaries(t *testing.T) {
	labels := "0 1000000 xx^xx-sil+k=a/A:0+1+1/F:1_1\n" +
		"1000000 3000000 xx^sil-k+a/A:1+1+1/F:1_1\n" +
		"3000000 5000000 sil^k-k+a/A:1+1+1/F:1_1\n" +
		"5000000 7000000 k-a+pau/A:1+1+1/F:1_1\n" +
		"7000000 9000000 a-pau+i/A:1+1+1/F:1_1\n" +
		"9000000 10000000 pau-i+sil/A:1+1+1/F:1_1\n" +
		"10000000 11000000 i-sil+xx/A:1+1+1/F:1_1"
	phones, boundaries, err := ParseHTSLabels(labels)
	if err != nil {
		t.Fatal(err)
	}
	phones[2].Frame = &Frame{Valid: true, RMSDB: -20, F0Hz: 220, SpectrumDB: []float64{-5, -4}}
	phones[3].Frame = &Frame{Valid: true, RMSDB: -19, F0Hz: 220, SpectrumDB: []float64{-5, -4}}
	boundaries[1].Features = &BoundaryFeatures{SpectrumDeltaDB: 1, RMSDeltaDB: 1, F0DeltaCents: 2, F0Comparable: true}
	boundaries[2].Features = &BoundaryFeatures{SpectrumDeltaDB: 3, RMSDeltaDB: 2, F0DeltaCents: 4, F0Comparable: true, VoicingMismatch: true}
	record := NewAlignment("test", "テスト", "test.wav", phones, boundaries)
	prior, err := BuildPrior([]Alignment{record}, false, "test-prior", "")
	if err != nil {
		t.Fatal(err)
	}
	if prior.PhoneCount != 4 || prior.Utterances != 1 || prior.BoundaryCount != 2 {
		t.Fatalf("prior counts=%+v", prior)
	}
	if _, ok := prior.Phones["sil"]; ok {
		t.Fatal("silence was included in phone prior")
	}
	if got := prior.Boundaries["k|a"].SpectrumDeltaDB.Mean; got != 3 {
		t.Fatalf("boundary prior mean=%v", got)
	}
	if got := prior.Boundaries["k|a"].VoicingMismatch.Mean; got != 1 {
		t.Fatalf("voicing mismatch mean=%v", got)
	}
}
