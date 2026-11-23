package wavfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/audiodsp/wav"
)

func TestWriteRejectsInvalidPCMWithoutReplacingFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		pcm  *wav.PCM
	}{
		{"nil", nil},
		{"empty", &wav.PCM{SampleRate: 16000, Channels: 1}},
		{"misaligned", &wav.PCM{SampleRate: 16000, Channels: 2, Data: make([]int16, 3)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output.wav")
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Write(path, tc.pcm); err == nil {
				t.Fatal("invalid PCM accepted")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "existing" {
				t.Fatalf("failed write replaced the file: %q, %v", data, err)
			}
		})
	}
}

func TestWriteRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.wav")
	pcm := &wav.PCM{SampleRate: 16000, Channels: 2, Data: []int16{1, -2, 3, -4}}
	if err := Write(path, pcm); err != nil {
		t.Fatal(err)
	}
	read, err := wav.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if read.SampleRate != pcm.SampleRate || read.Channels != pcm.Channels || len(read.Data) != len(pcm.Data) {
		t.Fatalf("read %+v", read)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(Bytes(pcm)) != string(data) {
		t.Fatalf("Bytes differs from the written file: %v", err)
	}
}

func TestBytesRejectsMisalignedPCM(t *testing.T) {
	if data := Bytes(&wav.PCM{SampleRate: 16000, Channels: 2, Data: make([]int16, 3)}); data != nil {
		t.Fatalf("encoded %d bytes of misaligned PCM", len(data))
	}
}
