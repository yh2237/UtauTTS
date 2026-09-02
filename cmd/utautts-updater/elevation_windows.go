//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"utautts/internal/updatelock"
)

func relaunchElevatedIfNeeded(target string, args []string) (bool, error) {
	if updatelock.PrimaryWritable(target) {
		return false, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return false, err
	}
	file, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return false, err
	}
	parameters, err := syscall.UTF16PtrFromString(joinWindowsArgs(args))
	if err != nil {
		return false, err
	}
	shellExecute := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	result, _, callErr := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		0,
		1,
	)
	if result <= 32 {
		if callErr != syscall.Errno(0) {
			return false, fmt.Errorf("ShellExecuteW(runas): %w", callErr)
		}
		return false, fmt.Errorf("ShellExecuteW(runas) failed with code %d", result)
	}
	return true, nil
}

func joinWindowsArgs(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = quoteWindowsArg(arg)
	}
	return strings.Join(quoted, " ")
}

func quoteWindowsArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}
	var builder strings.Builder
	builder.WriteByte('"')
	backslashes := 0
	for _, character := range arg {
		switch character {
		case '\\':
			backslashes++
		case '"':
			builder.WriteString(strings.Repeat("\\", backslashes*2+1))
			builder.WriteByte('"')
			backslashes = 0
		default:
			builder.WriteString(strings.Repeat("\\", backslashes))
			builder.WriteRune(character)
			backslashes = 0
		}
	}
	builder.WriteString(strings.Repeat("\\", backslashes*2))
	builder.WriteByte('"')
	return builder.String()
}
