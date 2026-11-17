package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyF0Cache(t *testing.T) {
	dir := t.TempDir()
	r := record{ID: "sample", Tokens: []token{{Start: 0, End: 100}, {Start: 100, End: 200, Pause: true}}}
	path := cachePathTag(dir, r, 10, "utautts_world_harvest")
	if got := filepath.Base(path); got != "20d4c32b3904508b1e562a6ca0a9abbd87954aa6.npy" {
		t.Fatal(got)
	}
	if got := filepath.Base(cachePathTag(dir, r, 2.5, "internal_autocorrelation")); got != "0afa7f8efaf0e46503f721189197c3c243deda9f.npy" {
		t.Fatal(got)
	}
	legacy, err := hex.DecodeString("934e554d5059010076007b276465736372273a20273c6638272c2027666f727472616e5f6f72646572273a2046616c73652c20277368617065273a2028322c292c207d2020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020202020200a0000000000e05e400000000000000080")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	values, err := loadF0Source(r, 10, dir, []float64{5, 15}, "world", "missing-world-engine")
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != 123.5 || math.Float64bits(values[1]) != 0x8000000000000000 {
		t.Fatalf("cached F0 = %v", values)
	}
	output := filepath.Join(dir, "training.npy")
	if err = writeNPY(output, values); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 96 || binary.LittleEndian.Uint16(got[8:10]) != 70 || !bytes.Equal(got[80:], legacy[128:]) {
		t.Fatalf("training NPY = %x", got)
	}
	if err = writeNPY(output, []float64{999}); err == nil {
		t.Fatal("overwrote existing NPY")
	}
	after, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(after, got) {
		t.Fatalf("existing NPY changed: %x, %v", after, err)
	}
}
