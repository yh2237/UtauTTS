package native

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSynthesisSourcePathsDoesNotLoadWaveforms(t *testing.T) {
	root := t.TempDir()
	bank := filepath.Join(root, "bank")
	if err := os.Mkdir(bank, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bank, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\nka.wav=か,0,0,0,0,0\nunused.wav=さ,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := newTestEngine(t, Config{VoiceDir: root, Renderer: "waveform", RendererDirectories: []string{waveformTestRendererDirectory(t)}})
	paths, err := e.SynthesisSourcePaths([]byte(`{"reading":"あか","method":"waveform-synthesis","voicebank_id":"bank","renderer":"waveform","model_id":"none"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{filepath.Join(bank, "a.wav"), filepath.Join(bank, "ka.wav")}) {
		t.Fatalf("paths=%v", paths)
	}
}
