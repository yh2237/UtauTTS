//go:build windows

package openjtalk

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type windowsEngine struct {
	dll                            *syscall.DLL
	initialize, analyze, lastError *syscall.Proc
	release                        *syscall.Proc
}

func openNativeEngine(path string) (nativeEngine, error) {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	engine := windowsEngine{dll: dll}
	for name, proc := range map[string]**syscall.Proc{
		"UtauTTSOpenJTalkInit":  &engine.initialize,
		"UtauTTSOpenJTalkRun":   &engine.analyze,
		"UtauTTSOpenJTalkError": &engine.lastError,
	} {
		if *proc, err = dll.FindProc(name); err != nil {
			_ = dll.Release()
			return nil, err
		}
	}
	// 古いライブラリにはない。なければ辞書を返さずに解放する。
	engine.release, _ = dll.FindProc("UtauTTSOpenJTalkClose")
	return &engine, nil
}

func (engine *windowsEngine) close() {
	if engine.release != nil {
		engine.release.Call()
	}
	_ = engine.dll.Release()
}

func (engine *windowsEngine) load(dictionary string) error {
	path, err := syscall.BytePtrFromString(mecabPath(dictionary))
	if err != nil {
		return err
	}
	result, _, _ := engine.initialize.Call(uintptr(unsafe.Pointer(path)))
	runtime.KeepAlive(path)
	if result == 0 {
		return engine.err()
	}
	return nil
}

func (engine *windowsEngine) run(text string) (string, error) {
	input, err := syscall.BytePtrFromString(text)
	if err != nil {
		return "", err
	}
	capacity := len(text)*64 + 4096
	for range 2 {
		output := make([]byte, capacity)
		result, _, _ := engine.analyze.Call(uintptr(unsafe.Pointer(input)), uintptr(unsafe.Pointer(&output[0])), uintptr(capacity))
		runtime.KeepAlive(input)
		written := int(int32(result))
		if written > 0 {
			return string(output[:written]), nil
		}
		if written == 0 {
			return "", engine.err()
		}
		capacity = -written
	}
	return "", errors.New("Open JTalk output buffer is too small")
}

func (engine *windowsEngine) err() error {
	result, _, _ := engine.lastError.Call()
	message := *(**byte)(unsafe.Pointer(&result))
	if message == nil {
		return errors.New("Open JTalk failed")
	}
	length := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(message), length)) != 0 {
		length++
	}
	return fmt.Errorf("Open JTalk: %s", unsafe.String(message, length))
}

// MeCabは辞書をANSIのAPIで開くので、パスをANSIのコードページへ直す。直せない文字があれば短い名前を試す。
func mecabPath(path string) string {
	ascii := true
	for _, character := range path {
		if character > 0x7f {
			ascii = false
			break
		}
	}
	if ascii {
		return path
	}
	if converted, ok := ansiString(path); ok {
		return converted
	}
	if converted, ok := ansiString(shortPath(path)); ok {
		return converted
	}
	return path
}

var wideCharToMultiByte = syscall.NewLazyDLL("kernel32.dll").NewProc("WideCharToMultiByte")

func ansiString(text string) (string, bool) {
	const codePageANSI, noBestFitChars = 0, 0x400
	wide, err := syscall.UTF16FromString(text)
	if err != nil {
		return "", false
	}
	var usedDefault int32
	size, _, _ := wideCharToMultiByte.Call(codePageANSI, noBestFitChars, uintptr(unsafe.Pointer(&wide[0])), ^uintptr(0), 0, 0, 0, uintptr(unsafe.Pointer(&usedDefault)))
	if size == 0 || usedDefault != 0 {
		return "", false
	}
	buffer := make([]byte, size)
	written, _, _ := wideCharToMultiByte.Call(codePageANSI, noBestFitChars, uintptr(unsafe.Pointer(&wide[0])), ^uintptr(0), uintptr(unsafe.Pointer(&buffer[0])), size, 0, 0)
	if written == 0 {
		return "", false
	}
	return string(buffer[:written-1]), true
}

func shortPath(path string) string {
	long, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buffer := make([]uint16, syscall.MAX_LONG_PATH)
	length, err := syscall.GetShortPathName(long, &buffer[0], uint32(len(buffer)))
	if err != nil || length == 0 || int(length) > len(buffer) {
		return path
	}
	return syscall.UTF16ToString(buffer[:length])
}
