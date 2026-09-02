//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"utautts/internal/updatelock"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		showError(err)
		return
	}
	root := filepath.Dir(executable)
	if os.Getenv("UTAUTTS_UPDATE_RELAUNCH") != "1" && updateBlocked(root) {
		return
	}
	target := filepath.Join(root, "app", "utautts-gui.exe")
	command := exec.Command(target, os.Args[1:]...)
	command.Dir = root
	if err := command.Start(); err != nil {
		showError(fmt.Errorf("Qt GUIを起動できませんでした。\n%s\n\n%w", target, err))
	}
}

func updateBlocked(root string) bool {
	paths, err := updatelock.Paths(root)
	if err != nil {
		return false
	}
	now := time.Now()
	active := false
	found := false
	for _, path := range paths {
		state, readErr := updatelock.ReadPath(path)
		if readErr == nil {
			found = true
			if lockStateActive(state, now, processAlive) {
				active = true
				break
			}
			continue
		}
		if !os.IsNotExist(readErr) {
			if info, statErr := os.Stat(path); statErr == nil {
				found = true
				if time.Since(info.ModTime()) < time.Minute {
					active = true
					break
				}
			}
		}
	}
	if !found {
		return false
	}
	if active {
		showMessage("UtauTTS 更新中", "UtauTTSを更新しています。\n完了すると自動的に再起動します。", 0x40)
		return true
	}
	if !confirmStaleUpdateRecovery() {
		return true
	}
	if err := updatelock.Remove(root); err != nil {
		showError(fmt.Errorf("更新ロックを解除できませんでした。\n\n%w", err))
		return true
	}
	return false
}

func lockStateActive(state updatelock.State, now time.Time, alive func(int) bool) bool {
	if state.UpdaterPID > 0 {
		return alive(state.UpdaterPID)
	}
	return !state.StartedAt.IsZero() && now.Sub(state.StartedAt) < time.Minute
}

func processAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").CombinedOutput()
	if err != nil {
		return false
	}
	pidText := strconv.Itoa(pid)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == pidText {
			return true
		}
	}
	return false
}

func confirmStaleUpdateRecovery() bool {
	const yes = 6
	result := showMessage("UtauTTS 更新の復旧",
		"前回の更新が中断された可能性があります。\n更新ロックを解除して通常起動しますか？",
		0x04|0x20)
	return result == yes
}

func showError(err error) {
	showMessage("UtauTTS 起動エラー", err.Error(), 0x10)
}

func showMessage(titleText, body string, flags uintptr) uintptr {
	text, _ := syscall.UTF16PtrFromString(body)
	title, _ := syscall.UTF16PtrFromString(titleText)
	messageBox := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	result, _, _ := messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), flags)
	return result
}
