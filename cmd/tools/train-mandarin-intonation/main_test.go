package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectionMath(t *testing.T) {
	if got := sampledF0([]float64{0, 100, 200, 800}, 0, .04, .5); got != 150 {
		t.Fatalf("sampled F0 = %g", got)
	}
	x := toneFeatures([]int{1, 2, 3}, 1, []bool{true, false, false}, []bool{false, false, true})
	if len(x) != 20 || x[6] != 1 || x[10] != 1 || x[17] != 1 {
		t.Fatalf("tone feature indices differ: %v", x)
	}
}

func TestTextgridWordsTier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.TextGrid")
	raw := "name = \"words\"\nintervals [1]:\n xmin = 0\n xmax = 0.1\n text = \"\"\nintervals [2]:\n xmin = 0.1\n xmax = 0.3\n text = \"ma1\"\nitem [2]:\nintervals [1]:\n xmin = 0\n xmax = 1\n text = \"ignore\"\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := textgridSyllables(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].label != "ma1" || got[0].start != .1 || got[0].end != .3 {
		t.Fatalf("words tier: %+v", got)
	}
}
