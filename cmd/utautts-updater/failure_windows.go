//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func notifyUpdateFailure(err error) {
	message := "更新を適用できませんでした。旧バージョンを起動します。\n\nUtauTTS のほかの画面や関連プロセスを閉じてから再試行してください。インストール先の変更権限も確認してください。"
	if err != nil {
		message += fmt.Sprintf("\n\n詳細: %v", err)
	}
	title, _ := syscall.UTF16PtrFromString("UtauTTS 更新エラー")
	body, _ := syscall.UTF16PtrFromString(message)
	messageBox := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	messageBox.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), 0x10)
}
