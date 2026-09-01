package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAndSampleWorldFRQ(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source_wav.frq")
	data := make([]byte, 40+4*16)
	copy(data, "FREQ0003")
	binary.LittleEndian.PutUint32(data[8:12], 256)
	binary.LittleEndian.PutUint32(data[36:40], 4)
	values := []float64{0, 200, 220, 0}
	for index, value := range values {
		binary.LittleEndian.PutUint64(data[40+index*16:48+index*16], math.Float64bits(value))
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	frq, err := readWorldFRQ(path)
	if err != nil {
		t.Fatal(err)
	}
	if frq.hopSize != 256 || len(frq.f0) != 4 || frq.f0[2] != 220 {
		t.Fatalf("unexpected FRQ: %+v", frq)
	}
	sampled := sampleWorldFRQ(frq, 2, 441, 71)
	if math.Abs(sampled[0]-210) > 1e-9 || math.Abs(sampled[1]-210) > 1e-9 {
		t.Fatalf("sampled F0 = %v, want [210 210]", sampled)
	}
}

func TestWorldAutoGainUsesWholeSourceForUnvoicedSegment(t *testing.T) {
	gain := worldAutoGain([]float64{0.1, -0.1}, []float64{0.8, -0.8}, []float64{0, 0})
	if gain >= 1 {
		t.Fatalf("unvoiced auto gain = %f, want less than 1", gain)
	}
}
