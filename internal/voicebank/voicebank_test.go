package voicebank

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestLoadMergesNestedOtoFiles(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "power")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "oto.ini"), "a.wav=あ,0,0,0,0,0\nshared.wav=共通,0,0,0,0,0\n")
	write(t, filepath.Join(sub, "OTO.INI"), "i.wav=い,0,0,0,0,0\nshared2.wav=共通,0,0,0,0,0\ninvalid\n")

	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bank.OtoFiles) != 2 {
		t.Fatalf("oto files = %v", bank.OtoFiles)
	}
	if bank.EntryCount() != 4 {
		t.Fatalf("entry count = %d", bank.EntryCount())
	}
	if len(bank.Entries["共通"]) != 2 {
		t.Fatalf("duplicate alias entries = %d", len(bank.Entries["共通"]))
	}
	if got, want := bank.Aliases(), []string{"あ", "い", "共通"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
	if len(bank.Diagnostics) != 1 || bank.Diagnostics[0].Line != 3 {
		t.Fatalf("diagnostics = %+v", bank.Diagnostics)
	}
}

func TestSourcePathWithinRejectsSymlinkOutsideVoicebank(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "bank")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.wav")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.wav")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if sourcePathWithin(root, link) {
		t.Fatal("source symlink outside voicebank was accepted")
	}
}

func TestSourcePathWithinAcceptsRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "inside.wav")
	if err := os.WriteFile(path, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !sourcePathWithin(root, path) {
		t.Fatal("regular voicebank source was rejected")
	}
}

func TestLoadAcceptsOtoPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "oto.ini")
	write(t, path, "a.wav=あ,0,0,0,0,0\n")
	bank, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if bank.Root != root || bank.EntryCount() != 1 {
		t.Fatalf("bank = %+v", bank)
	}
}

func TestLoadRejectsDirectoryWithoutOto(t *testing.T) {
	_, err := Load(t.TempDir())
	if !errors.Is(err, ErrNoOto) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsOtoSourceOutsideVoicebank(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bank")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "oto.ini"), "../outside.wav=alias,0,0,0,0,0\n")
	if _, err := Load(root); err == nil {
		t.Fatal("accepted oto source outside voicebank root")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
