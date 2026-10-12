package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWavRejectsEmptyPCMWithoutReplacingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.wav")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteWav(path, &PCM{SampleRate: 16000, Channels: 1}); err == nil {
		t.Fatal("empty PCM accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing" {
		t.Fatalf("failed write replaced the file: %q, %v", data, err)
	}
}
