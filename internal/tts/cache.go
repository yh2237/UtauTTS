package tts

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"utautts/internal/diffsinger"
	"utautts/internal/openjtalk"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

// GUIの反復合成で変わらない高コストな入力をプロセス内に保持する。

const maxAnalysisCacheEntries = 512

var synthesisCache = struct {
	sync.RWMutex
	banks         map[string]*voicebank.Bank
	models        map[string]modelCacheEntry
	analyses      map[analysisCacheKey]*openjtalk.Analysis
	analysisOrder []analysisCacheKey
}{
	banks:    make(map[string]*voicebank.Bank),
	models:   make(map[string]modelCacheEntry),
	analyses: make(map[analysisCacheKey]*openjtalk.Analysis),
}

type modelCacheEntry struct {
	size    int64
	modTime int64
	model   *prosody.Model
}

type analysisCacheKey struct {
	text       string
	helper     string
	dictionary string
}

func loadVoicebankCached(path string) (*voicebank.Bank, error) {
	key, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	synthesisCache.RLock()
	bank := synthesisCache.banks[key]
	synthesisCache.RUnlock()
	if bank != nil {
		return bank, nil
	}
	bank, err = voicebank.Load(key)
	if err != nil {
		return nil, err
	}
	synthesisCache.Lock()
	if existing := synthesisCache.banks[key]; existing != nil {
		bank = existing
	} else {
		synthesisCache.banks[key] = bank
	}
	synthesisCache.Unlock()
	return bank, nil
}

func loadProsodyModelCached(path string) (*prosody.Model, error) {
	key, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(key)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime().UnixNano()
	synthesisCache.RLock()
	entry, ok := synthesisCache.models[key]
	synthesisCache.RUnlock()
	if ok && entry.size == info.Size() && entry.modTime == modTime && entry.model != nil {
		return entry.model, nil
	}
	model, err := prosody.LoadModel(key)
	if err != nil {
		return nil, err
	}
	synthesisCache.Lock()
	synthesisCache.models[key] = modelCacheEntry{size: info.Size(), modTime: modTime, model: model}
	synthesisCache.Unlock()
	return model, nil
}

func analyzeOpenJTalkCached(ctx context.Context, text string, cfg openjtalk.Config) (*openjtalk.Analysis, error) {
	key := analysisCacheKey{text: text, helper: cfg.HelperPath, dictionary: cfg.DictionaryPath}
	synthesisCache.RLock()
	analysis := synthesisCache.analyses[key]
	synthesisCache.RUnlock()
	if analysis != nil {
		return analysis, nil
	}
	analysis, err := openjtalk.AnalyzeContext(ctx, text, cfg)
	if err != nil {
		return nil, err
	}
	synthesisCache.Lock()
	if existing := synthesisCache.analyses[key]; existing != nil {
		analysis = existing
	} else {
		synthesisCache.analyses[key] = analysis
		synthesisCache.analysisOrder = append(synthesisCache.analysisOrder, key)
		if len(synthesisCache.analysisOrder) > maxAnalysisCacheEntries {
			oldest := synthesisCache.analysisOrder[0]
			synthesisCache.analysisOrder = synthesisCache.analysisOrder[1:]
			delete(synthesisCache.analyses, oldest)
		}
	}
	synthesisCache.Unlock()
	return analysis, nil
}

// ClearCachesは音源やランタイム資源の更新後に合成入力を破棄する。
func ClearCaches() {
	// A DiffSinger bridge keeps ONNX sessions resident by model path. Closing
	// it here prevents a replaced model at the same path from remaining live
	// in the child process after a voicebank/model reload.
	_ = diffsinger.CloseProviderSessions()
	synthesisCache.Lock()
	synthesisCache.banks = make(map[string]*voicebank.Bank)
	synthesisCache.models = make(map[string]modelCacheEntry)
	synthesisCache.analyses = make(map[analysisCacheKey]*openjtalk.Analysis)
	synthesisCache.analysisOrder = nil
	synthesisCache.Unlock()
	render.ClearWAVCache()
}
