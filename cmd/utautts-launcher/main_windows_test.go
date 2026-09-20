//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"utautts/internal/updatelock"
)

func TestUpdateBlockedWithoutLock(t *testing.T) {
	root := filepath.Join(t.TempDir(), "UtauTTS")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if updateBlocked(root) {
		t.Fatal("launcher blocked startup without an update lock")
	}
}

func TestRemoveOldInstallBackup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "UtauTTS")
	old := root + ".old"
	if err := os.MkdirAll(filepath.Join(old, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "tools", "utautts-updater.exe"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeOldInstallBackup(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old install backup remains: %v", err)
	}
}

func TestLockStateActive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		state updatelock.State
		alive bool
		want  bool
	}{
		{name: "running updater", state: updatelock.State{UpdaterPID: 42}, alive: true, want: true},
		{name: "stopped updater", state: updatelock.State{UpdaterPID: 42}, alive: false, want: false},
		{name: "pending handoff", state: updatelock.State{StartedAt: now.Add(-10 * time.Second)}, want: true},
		{name: "stale pending handoff", state: updatelock.State{StartedAt: now.Add(-2 * time.Minute)}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := lockStateActive(test.state, now, func(int) bool { return test.alive })
			if got != test.want {
				t.Fatalf("active = %v, want %v", got, test.want)
			}
		})
	}
}
