//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRunReplacesContentsWhenInstallDirectoryIsInUse(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	writeTestFile(t, filepath.Join(target, "voice", "bank", "oto.ini"), "user-voice")
	writeTestFile(t, filepath.Join(target, "obsolete.txt"), "obsolete")
	zipPath := makeZip(t, map[string]string{
		"app/utautts-gui.exe":   "new-gui",
		"voice/bundled/oto.ini": "new-voice",
	})

	name, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	const fileFlagBackupSemantics = 0x02000000
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, fileFlagBackupSemantics, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	if err := os.Rename(target, target+".probe"); err == nil {
		_ = os.Rename(target+".probe", target)
		t.Fatal("test directory was not held against renaming")
	}

	if err := run(target, "", zipPath, 0, "test", []string{"voice"}, false); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"app/utautts-gui.exe":   "new-gui",
		"voice/bank/oto.ini":    "user-voice",
		"voice/bundled/oto.ini": "new-voice",
	} {
		data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Errorf("%s = %q, %v; want %q", path, data, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "obsolete.txt")); !os.IsNotExist(err) {
		t.Errorf("obsolete file remained: %v", err)
	}
}

func TestRunKeepsOldInstallWhenAChildDirectoryIsInUse(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	zipPath := makeZip(t, map[string]string{"app/utautts-gui.exe": "new-gui"})

	name, err := syscall.UTF16PtrFromString(filepath.Join(target, "app"))
	if err != nil {
		t.Fatal(err)
	}
	const fileFlagBackupSemantics = 0x02000000
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, fileFlagBackupSemantics, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)

	if err := run(target, "", zipPath, 0, "test", nil, true); err == nil {
		t.Fatal("expected update to fail while app directory is in use")
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("downloaded archive was removed after a failed update: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "app", "utautts-gui.exe"))
	if err != nil || string(data) != "old-gui" {
		t.Fatalf("old install was not restored: %q, %v", data, err)
	}
}

func TestReplaceContentsRollsBackAfterPartialInstall(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	stage := target + ".stage"
	old := target + ".old"
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	writeTestFile(t, filepath.Join(target, "docs", "README.md"), "old-docs")
	writeTestFile(t, filepath.Join(stage, "app", "utautts-gui.exe"), "new-gui")
	writeTestFile(t, filepath.Join(stage, "docs", "README.md"), "new-docs")

	name, err := syscall.UTF16PtrFromString(filepath.Join(stage, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	const fileFlagBackupSemantics = 0x02000000
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, fileFlagBackupSemantics, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)

	if err := replaceInstallContents(target, stage, old); err == nil {
		t.Fatal("expected staged docs to block the update")
	}
	for path, want := range map[string]string{
		"app/utautts-gui.exe": "old-gui",
		"docs/README.md":      "old-docs",
	} {
		data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Errorf("restored %s = %q, %v; want %q", path, data, err, want)
		}
	}
}
