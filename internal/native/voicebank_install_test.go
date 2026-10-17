package native

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeBankZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(out)
	for name, body := range files {
		w, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func installRequest(t *testing.T, zipPath string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]string{"zip_path": zipPath, "name": "bank"})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInstallVoicebankReplacesExistingBank(t *testing.T) {
	voiceDir := t.TempDir()
	old := filepath.Join(voiceDir, "bank")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"oto.ini": "o.wav=お,0,0,0,0,0\n", "old.txt": "old"} {
		if err := os.WriteFile(filepath.Join(old, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipPath := filepath.Join(t.TempDir(), "bank.zip")
	writeBankZip(t, zipPath, map[string]string{"oto.ini": "a.wav=あ,0,0,0,0,0\n"})
	engine := newTestEngine(t, Config{VoiceDir: voiceDir})
	result, err := engine.installVoicebank(installRequest(t, zipPath))
	if err != nil {
		t.Fatal(err)
	}
	if listed := result.(map[string]any)["voicebanks"].([]map[string]any); len(listed) != 1 {
		t.Fatalf("listed %d voicebanks after install, want 1", len(listed))
	}
	if _, err := os.Stat(filepath.Join(old, "oto.ini")); err != nil {
		t.Fatalf("new bank not installed: %v", err)
	}
	if installed, err := os.ReadDir(old); err != nil || len(installed) != 1 {
		t.Fatalf("installed bank has %d entries (%v), want only oto.ini", len(installed), err)
	}
	entries, err := os.ReadDir(voiceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("voice directory has %d entries, want only the bank", len(entries))
	}
}

func TestInstallVoicebankKeepsOldBankWhenItCannotBeMoved(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("an open file blocks moving its folder only on Windows")
	}
	voiceDir := t.TempDir()
	old := filepath.Join(voiceDir, "bank")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(old, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	held, err := os.Open(filepath.Join(old, "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	zipPath := filepath.Join(t.TempDir(), "bank.zip")
	writeBankZip(t, zipPath, map[string]string{"oto.ini": "a.wav=あ,0,0,0,0,0\n"})
	engine := newTestEngine(t, Config{VoiceDir: voiceDir})
	if _, err := engine.installVoicebank(installRequest(t, zipPath)); err == nil {
		t.Fatal("install succeeded while the old bank was in use")
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(old, name)); err != nil {
			t.Fatalf("old bank lost %s: %v", name, err)
		}
	}
}
