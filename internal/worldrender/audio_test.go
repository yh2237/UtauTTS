package worldrender

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func pcm16TestFormat(channels uint16) []byte {
	format := make([]byte, 16)
	binary.LittleEndian.PutUint16(format, 1)
	binary.LittleEndian.PutUint16(format[2:], channels)
	binary.LittleEndian.PutUint32(format[4:], 44100)
	binary.LittleEndian.PutUint32(format[8:], 44100*uint32(channels)*2)
	binary.LittleEndian.PutUint16(format[12:], channels*2)
	binary.LittleEndian.PutUint16(format[14:], 16)
	return format
}

func pcm16TestChunk(id string, payload []byte) []byte {
	chunk := make([]byte, 8+len(payload)+len(payload)%2)
	copy(chunk, id)
	binary.LittleEndian.PutUint32(chunk[4:], uint32(len(payload)))
	copy(chunk[8:], payload)
	return chunk
}

func pcm16TestWAV(chunks ...[]byte) []byte {
	data := []byte("RIFF\x00\x00\x00\x00WAVE")
	for _, chunk := range chunks {
		data = append(data, chunk...)
	}
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	return data
}

func writePCM16TestFile(tb testing.TB, data []byte) string {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "source.wav")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		tb.Fatal(err)
	}
	return path
}

func TestReadPCM16Chunks(t *testing.T) {
	for _, channels := range []uint16{1, 2} {
		name := "mono"
		if channels == 2 {
			name = "stereo"
		}
		t.Run(name, func(t *testing.T) {
			format := pcm16TestChunk("fmt ", append(pcm16TestFormat(channels), 0, 0))
			pcm := []byte{0, 128, 255, 127}
			data := pcm16TestChunk("data", pcm)
			rate, samples, err := readPCM16(writePCM16TestFile(t, pcm16TestWAV(format, pcm16TestChunk("JUNK", []byte{1, 2, 3}), data)))
			if err != nil {
				t.Fatal(err)
			}
			want := []float64{-1, 32767.0 / 32768}
			if channels == 2 {
				want = []float64{-1.0 / 65536}
			}
			if rate != 44100 || len(samples) != len(want) {
				t.Fatalf("rate=%d samples=%v", rate, samples)
			}
			for i := range want {
				if samples[i] != want[i] {
					t.Fatalf("samples[%d]=%v, want %v", i, samples[i], want[i])
				}
			}
		})
	}
}

// 配布音源にある形。RIFFサイズが8 byte短く、奇数長のdataがpadなしでEOFに達する。
func TestReadPCM16AcceptsShortRIFFAndUnpaddedOddData(t *testing.T) {
	data := pcm16TestChunk("data", []byte{0, 128, 255, 127, 9})
	contents := pcm16TestWAV(pcm16TestChunk("fmt ", pcm16TestFormat(1)), data[:len(data)-1])
	binary.LittleEndian.PutUint32(contents[4:], uint32(len(contents)-16))
	rate, samples, err := readPCM16(writePCM16TestFile(t, contents))
	if err != nil {
		t.Fatal(err)
	}
	if rate != 44100 || len(samples) != 2 || samples[0] != -1 || samples[1] != 32767.0/32768 {
		t.Fatalf("rate=%d samples=%v", rate, samples)
	}
}

func TestReadPCM16RejectsMalformedChunks(t *testing.T) {
	format := pcm16TestChunk("fmt ", pcm16TestFormat(1))
	data := pcm16TestChunk("data", []byte{0, 0})
	missingPad := pcm16TestChunk("JUNK", []byte{1})
	missingPad = missingPad[:len(missingPad)-1]
	for name, contents := range map[string][]byte{
		"short fmt":            pcm16TestWAV(pcm16TestChunk("fmt ", []byte{1, 0}), data),
		"partial stereo frame": pcm16TestWAV(pcm16TestChunk("fmt ", pcm16TestFormat(2)), data),
		"missing padding":      pcm16TestWAV(format, data, missingPad),
		"partial chunk header": pcm16TestWAV(format, data, []byte("JUNK")),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := readPCM16(writePCM16TestFile(t, contents)); err == nil {
				t.Fatal("malformed WAV was accepted")
			}
		})
	}
}

func BenchmarkReadPCM16(b *testing.B) {
	for _, metadataBytes := range []int{0, 4 << 20} {
		name := "audio-only"
		if metadataBytes > 0 {
			name = "4MiB-metadata"
		}
		b.Run(name, func(b *testing.B) {
			chunks := [][]byte{pcm16TestChunk("fmt ", pcm16TestFormat(1))}
			if metadataBytes > 0 {
				chunks = append(chunks, pcm16TestChunk("JUNK", make([]byte, metadataBytes)))
			}
			chunks = append(chunks, pcm16TestChunk("data", make([]byte, 4410*2)))
			path := writePCM16TestFile(b, pcm16TestWAV(chunks...))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, _, err := readPCM16(path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestReadPCM16RejectsOversizedDeclarations(t *testing.T) {
	for _, id := range []string{"fmt ", "data", "JUNK"} {
		t.Run(id, func(t *testing.T) {
			chunk := pcm16TestChunk(id, nil)
			binary.LittleEndian.PutUint32(chunk[4:], math.MaxUint32)
			if _, _, err := readPCM16(writePCM16TestFile(t, pcm16TestWAV(chunk))); !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("oversized chunk error=%v, want unexpected EOF", err)
			}
		})
	}
}

func TestReadPCM16RejectsInvalidFormat(t *testing.T) {
	for name, mutate := range map[string]func([]byte){
		"not RIFF":      func(b []byte) { copy(b, "RIFX") },
		"not WAVE":      func(b []byte) { copy(b[8:], "wave") },
		"not PCM":       func(b []byte) { binary.LittleEndian.PutUint16(b[20:], 3) },
		"zero channels": func(b []byte) { binary.LittleEndian.PutUint16(b[22:], 0) },
		"zero rate":     func(b []byte) { binary.LittleEndian.PutUint32(b[24:], 0) },
	} {
		t.Run(name, func(t *testing.T) {
			contents := pcm16TestWAV(pcm16TestChunk("fmt ", pcm16TestFormat(1)), pcm16TestChunk("data", []byte{0, 0}))
			mutate(contents)
			if _, _, err := readPCM16(writePCM16TestFile(t, contents)); err == nil {
				t.Fatal("invalid format was accepted")
			}
		})
	}
}

func FuzzReadPCM16(f *testing.F) {
	f.Add(pcm16TestWAV(pcm16TestChunk("fmt ", pcm16TestFormat(1)), pcm16TestChunk("data", []byte{0, 0})))
	f.Add(pcm16TestWAV(pcm16TestChunk("JUNK", []byte{1})))
	f.Add([]byte("RIFF\xff\xff\xff\xffWAVEdata\xff\xff\xff\xff"))
	f.Fuzz(func(t *testing.T, contents []byte) {
		if len(contents) > 64<<10 {
			t.Skip()
		}
		rate, samples, err := readPCM16(writePCM16TestFile(t, contents))
		if err == nil && (rate <= 0 || len(samples) > len(contents)/2) {
			t.Fatalf("invalid decoded metadata: rate=%d frames=%d bytes=%d", rate, len(samples), len(contents))
		}
	})
}
