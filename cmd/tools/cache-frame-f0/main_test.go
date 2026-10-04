package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
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

func TestPythonWORLDCacheParity(t *testing.T) {
	root := filepath.Join("..", "..", "..", "out", "python-migration")
	py := filepath.Join(root, "py-f0")
	goCache := filepath.Join(root, "go-f0")
	entries, err := os.ReadDir(py)
	if err != nil {
		t.Skipf("optional Python cache: %v", err)
	}
	count := 0
	maxDiff := 0.0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".npy" {
			continue
		}
		want, err := readNPY(filepath.Join(py, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got, err := readNPY(filepath.Join(goCache, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("%s frames=%d want=%d", entry.Name(), len(got), len(want))
		}
		for i := range got {
			d := math.Abs(got[i] - want[i])
			maxDiff = math.Max(maxDiff, d)
			if d > 1e-9 {
				t.Fatalf("%s frame %d diff=%g", entry.Name(), i, d)
			}
		}
		count++
	}
	if count != 3 {
		t.Fatalf("matched %d caches, want 3", count)
	}
	t.Logf("%d real English records; maximum WORLD F0 difference %.12g Hz", count, maxDiff)
}

func readNPY(path string) ([]float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 10 {
		return nil, os.ErrInvalid
	}
	n := int(binary.LittleEndian.Uint16(b[8:10]))
	if len(b) < 10+n || (len(b)-10-n)%8 != 0 {
		return nil, os.ErrInvalid
	}
	data := b[10+n:]
	out := make([]float64, len(data)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return out, nil
}
