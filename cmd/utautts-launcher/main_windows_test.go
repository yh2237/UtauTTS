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
