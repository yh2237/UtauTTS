package main

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestNumPyHeaderAndCacheKey(t *testing.T) {
	r := record{ID: "sample", Tokens: []token{{Start: 0, End: 100}, {Start: 100, End: 200, Pause: true}}}
	if got := filepath.Base(cachePath("out", r)); got == "" || filepath.Ext(got) != ".npy" {
		t.Fatal(got)
	}
	var b bytes.Buffer
	if err := writeNPY(&b, []float64{123.5, 0}); err != nil {
		t.Fatal(err)
	}
	x := b.Bytes()
	if len(x) != 128+16 {
		t.Fatalf("NPY bytes=%d", len(x))
	}
	if string(x[:6]) != "\x93NUMPY" || binary.LittleEndian.Uint16(x[8:10]) != 118 {
		t.Fatalf("NPY header=%v", x[:10])
	}
}
