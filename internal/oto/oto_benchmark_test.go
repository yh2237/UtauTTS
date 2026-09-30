package oto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
)

var benchmarkIni *Ini
var benchmarkDecoded string

func benchmarkOtoData(b *testing.B, shiftJIS bool) []byte {
	b.Helper()
	var text strings.Builder
	for i := 0; i < 1024; i++ {
		fmt.Fprintf(&text, "音源%04d.wav=あ%02d,12.5,100.25,-200.5,30.75,10.5\r\n", i, i%32)
	}
	data := []byte(text.String())
	if shiftJIS {
		var err error
		data, err = japanese.ShiftJIS.NewEncoder().Bytes(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	return data
}

func BenchmarkOtoReadIni(b *testing.B) {
	for _, encoding := range []string{"UTF8", "ShiftJIS"} {
		b.Run(encoding, func(b *testing.B) {
			data := benchmarkOtoData(b, encoding == "ShiftJIS")
			path := filepath.Join(b.TempDir(), "oto.ini")
			if err := os.WriteFile(path, data, 0600); err != nil {
				b.Fatal(err)
			}
			ini, err := ReadIni(path)
			if err != nil || len(ini.Entries) != 32 || len(ini.Diagnostics) != 0 {
				b.Fatalf("fixture: %v, %v", ini, err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkIni, err = ReadIni(path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOtoDecode(b *testing.B) {
	for _, encoding := range []string{"UTF8", "ShiftJIS"} {
		b.Run(encoding, func(b *testing.B) {
			data := benchmarkOtoData(b, encoding == "ShiftJIS")
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkDecoded, _, err = Decode(data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
