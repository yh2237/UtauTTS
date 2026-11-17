package toolutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnderOut(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"out", true},
		{"out/model.json", true},
		{"out2/model.json", false},
		{"models/model.json", false},
		{"../out/model.json", false},
	}
	for _, test := range tests {
		if got := UnderOut(test.path); got != test.want {
			t.Errorf("UnderOut(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestUnderOutChild(t *testing.T) {
	if UnderOutChild("out") {
		t.Error("UnderOutChild(out) = true, want false")
	}
	if !UnderOutChild("out/model.json") {
		t.Error("UnderOutChild(out/model.json) = false, want true")
	}
}

func TestScanJSONLBytes(t *testing.T) {
	data := []byte("\ufeff{\"id\":\"a\"}\n\n  \n{\"id\":\"b\"}\n")
	path := filepath.Join(t.TempDir(), "data.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, scan := range []func(func([]byte) error) error{
		func(handle func([]byte) error) error { return ScanJSONLBytes(data, handle) },
		func(handle func([]byte) error) error { return ScanJSONL(path, handle) },
	} {
		var ids []string
		if err := scan(func(line []byte) error {
			ids = append(ids, string(line))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(ids) != 2 || ids[0] != "{\"id\":\"a\"}" || ids[1] != "{\"id\":\"b\"}" {
			t.Fatalf("lines = %q", ids)
		}
	}
}

func TestScanJSONLErrors(t *testing.T) {
	failure := errors.New("callback failed")
	calls := 0
	err := ScanJSONLBytes([]byte("first\nsecond\n"), func([]byte) error {
		calls++
		return failure
	})
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("callback error=%v calls=%d", err, calls)
	}
	if err := ScanJSONLBytes(bytes.Repeat([]byte("x"), maxJSONLLineBytes+1), func([]byte) error {
		t.Fatal("oversized line reached callback")
		return nil
	}); err == nil {
		t.Fatal("oversized line was accepted")
	}
	if err := ScanJSONL(filepath.Join(t.TempDir(), "missing"), func([]byte) error { return nil }); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error=%v", err)
	}
	err = scanJSONL(failingReader{failure}, func([]byte) error { return nil })
	if !errors.Is(err, failure) {
		t.Fatalf("read error=%v", err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestScanJSONLOnlyRemovesLeadingBOM(t *testing.T) {
	data := "\ufefffirst\n\ufeffsecond\n"
	var got []string
	if err := ScanJSONLBytes([]byte(data), func(line []byte) error {
		got = append(got, string(line))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != "first\n\ufeffsecond" {
		t.Fatalf("lines=%q", got)
	}
}
