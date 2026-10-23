//go:build !js

package openjtalk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ライブラリは辞書を1つだけ持つので、呼び出しを直列にする。
var engine struct {
	sync.Mutex
	library    nativeEngine
	path       string
	dictionary string
}

type nativeEngine interface {
	load(dictionary string) error
	run(text string) (string, error)
}

func runFrontend(ctx context.Context, text string, cfg Config) (*Analysis, error) {
	path, err := ResolveEnginePath(cfg.EnginePath)
	if err != nil {
		return nil, err
	}
	dictionary, err := ResolveDictionaryPath(cfg.DictionaryPath)
	if err != nil {
		return nil, err
	}
	engine.Lock()
	defer engine.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if engine.library == nil || engine.path != path {
		library, err := openNativeEngine(path)
		if err != nil {
			return nil, fmt.Errorf("load Open JTalk engine %q: %w", path, err)
		}
		engine.library, engine.path, engine.dictionary = library, path, ""
	}
	if engine.dictionary != dictionary {
		if err := engine.library.load(dictionary); err != nil {
			return nil, fmt.Errorf("load Open JTalk dictionary %q: %w", dictionary, err)
		}
		engine.dictionary = dictionary
	}
	tsv, err := engine.library.run(text)
	if err != nil {
		return nil, err
	}
	nodes, err := parseNJD(tsv)
	if err != nil {
		return nil, err
	}
	return buildAnalysis(nodes), nil
}

// ResolveEnginePathはAnalyzeと同じライブラリを、読み込まずに探す。
func ResolveEnginePath(explicit string) (string, error) {
	name := "utautts-openjtalk-engine.so"
	switch runtime.GOOS {
	case "windows":
		name = "utautts-openjtalk-engine.dll"
	case "darwin":
		name = "utautts-openjtalk-engine.dylib"
	}
	return resolveFile(explicit, name, []string{filepath.Join("runtime", name)})
}

// ResolveDictionaryPathは配置だけを確認し、辞書の内容は検査しない。
func ResolveDictionaryPath(explicit string) (string, error) {
	const name = "open_jtalk_dic_utf_8-1.11"
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && info.IsDir() {
			return filepath.Abs(explicit)
		}
		return "", fmt.Errorf("Open JTalk dictionary not found: %s", explicit)
	}
	for _, root := range searchRoots() {
		candidate := filepath.Join(root, "runtime", name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("Open JTalk dictionary not found; expected runtime/%s", name)
}

func resolveFile(explicit, description string, relatives []string) (string, error) {
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return filepath.Abs(explicit)
		}
		return "", fmt.Errorf("%s not found: %s", description, explicit)
	}
	for _, root := range searchRoots() {
		for _, relative := range relatives {
			candidate := filepath.Join(root, relative)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return filepath.Abs(candidate)
			}
		}
	}
	return "", fmt.Errorf("%s not found", description)
}

func searchRoots() []string {
	var roots []string
	if executable, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(executable), filepath.Dir(filepath.Dir(executable)))
	}
	if current, err := os.Getwd(); err == nil {
		roots = append(roots, current)
	}
	return roots
}
