package aviutl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"utautts/internal/audio"
)

func writeWAV(t *testing.T, path string, sampleRate int, samples int) {
	t.Helper()
	data := make([]int16, samples)
	for index := range data {
		data[index] = 1000
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteExoMatchesAviUtlAudioLayout(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "001_あ.wav")
	second := filepath.Join(dir, "002_い.wav")
	writeWAV(t, first, 1000, 1000)
	writeWAV(t, second, 1000, 2000)

	output := filepath.Join(dir, "utautts.exo")
	if err := WriteExo(output, []string{first, second}, 60); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	if err != nil {
		t.Fatalf("exo is not Shift_JIS: %v", err)
	}
	content := string(decoded)

	want := "[exedit]\nwidth=1920\nheight=1080\nrate=60\nscale=1\nlength=180\naudio_rate=48000\naudio_ch=2\n" +
		"[0]\nstart=1\nend=60\nlayer=1\noverlay=1\naudio=1\n" +
		"[0.0]\n_name=音声ファイル\n再生位置=0.00\n再生速度=100.0\nループ再生=0\n動画ファイルと連携=0\nfile=" + toExoPath(first) + "\n" +
		"[0.1]\n_name=標準再生\n音量=100.0\n左右=0.0\n" +
		"[1]\nstart=61\nend=180\nlayer=1\noverlay=1\naudio=1\n" +
		"[1.0]\n_name=音声ファイル\n再生位置=0.00\n再生速度=100.0\nループ再生=0\n動画ファイルと連携=0\nfile=" + toExoPath(second) + "\n" +
		"[1.1]\n_name=標準再生\n音量=100.0\n左右=0.0\n"
	if content != want {
		t.Fatalf("exo layout mismatch:\n--- got ---\n%s\n--- want ---\n%s", content, want)
	}
}

func TestWriteExoHonorsFrameRate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.wav")
	writeWAV(t, path, 1000, 1000)

	output := filepath.Join(dir, "utautts.exo")
	if err := WriteExo(output, []string{path}, 30); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	content := string(decoded)
	if !strings.Contains(content, "rate=30\n") || !strings.Contains(content, "[0]\nstart=1\nend=30\n") {
		t.Fatalf("frame rate not applied:\n%s", content)
	}
}

func TestWriteExoRejectsEmptyListAndBadRate(t *testing.T) {
	output := filepath.Join(t.TempDir(), "utautts.exo")
	if err := WriteExo(output, nil, 60); err == nil {
		t.Fatal("empty clip list was accepted")
	}
	if err := WriteExo(output, []string{"a.wav"}, 0); err == nil {
		t.Fatal("zero frame rate was accepted")
	}
	if err := WriteExo(output, []string{"a.wav"}, 300); err == nil {
		t.Fatal("excessive frame rate was accepted")
	}
}
