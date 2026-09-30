package otofile

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanKeepsDuplicatesAndDiagnosticsWithoutFilesystem(t *testing.T) {
	base := filepath.Join("missing", "bank")
	text := "; comment\r\n\r\nsub\\a.wav=あ,12.5,100,-20,30,10,extra\r\n" +
		"b.wav=あ,0,0,0,0,0\r\nbroken\r\nc.wav=,,,,,\r\n"
	var entries []Entry
	diagnostics, err := Scan(strings.NewReader(text), base, func(entry Entry) { entries = append(entries, entry) })
	if err != nil || len(entries) != 3 || len(diagnostics) != 1 || diagnostics[0].Line != 5 {
		t.Fatalf("entries=%+v diagnostics=%+v err=%v", entries, diagnostics, err)
	}
	if entries[0].Filename != filepath.Join(base, "sub", "a.wav") || entries[0].Line != 3 || entries[0].Blank != -20 || entries[1].Alias != "あ" || entries[2].Alias != "c" {
		t.Fatalf("unexpected parsed values: %+v", entries)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestScanPropagatesReaderAndLineLimitErrors(t *testing.T) {
	want := errors.New("read failed")
	if _, err := Scan(io.MultiReader(strings.NewReader("a.wav=a,0,0,0,0,0\n"), failingReader{want}), "", func(Entry) {}); !errors.Is(err, want) {
		t.Fatalf("reader error = %v", err)
	}
	if _, err := Scan(strings.NewReader(strings.Repeat("x", 1024*1024)), "", func(Entry) {}); err == nil {
		t.Fatal("oversized line accepted")
	}
}
