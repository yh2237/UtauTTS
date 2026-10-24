//go:build !js

package openjtalk

import (
	"context"
	"fmt"
	"sync"

	"github.com/yh2237/UtauTTS/internal/runtimepath"
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
	close()
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
		if engine.library != nil {
			engine.library.close()
			engine.library = nil
		}
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
	return runtimepath.Find(explicit, runtimepath.LibraryName("utautts-openjtalk-engine"), false)
}

// ResolveDictionaryPathは配置だけを確認し、辞書の内容は検査しない。
func ResolveDictionaryPath(explicit string) (string, error) {
	return runtimepath.Find(explicit, "open_jtalk_dic_utf_8-1.11", true)
}
