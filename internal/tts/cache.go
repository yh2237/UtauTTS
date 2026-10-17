package tts

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/neural"
	"github.com/yh2237/UtauTTS/internal/openjtalk"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// GUIの反復合成で変わらない高コストな入力をプロセス内に保持する。

const maxAnalysisCacheEntries = 512
const maxProsodyCacheEntries = 128

type synthesisCacheState struct {
	sync.RWMutex
	banks            map[string]*voicebank.Bank
	models           map[string]modelCacheEntry
	analyses         map[analysisCacheKey]*openjtalk.Analysis
	analysisOrder    []analysisCacheKey
	computations     map[prosodyComputationKey]prosodyComputation
	computationOrder []prosodyComputationKey
	unified          map[unifiedProsodyCacheKey]unifiedProsodyPrediction
	unifiedOrder     []unifiedProsodyCacheKey
	unifiedBytes     int
}

func newSynthesisCache() *synthesisCacheState {
	return &synthesisCacheState{
		banks: make(map[string]*voicebank.Bank), models: make(map[string]modelCacheEntry),
		analyses:     make(map[analysisCacheKey]*openjtalk.Analysis),
		computations: make(map[prosodyComputationKey]prosodyComputation),
		unified:      make(map[unifiedProsodyCacheKey]unifiedProsodyPrediction),
	}
}

var synthesisCache = newSynthesisCache()

func cacheFor(ctx context.Context) (*synthesisCacheState, error) {
	return lifecycle.Resource(ctx, "tts.cache", synthesisCache, newSynthesisCache,
		func(cache *synthesisCacheState) error { cache.clear(); return nil })
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

type prosodyComputation struct {
	morae       []frontend.Mora
	features    []prosody.FeatureFrame
	predictions []prosody.Prediction
}

type prosodyComputationKey struct {
	text, reading, language, phonemizer string
	modelPath, renderer, voicebankPath  string
	openJTalkPath, openJTalkDictionary  string
	settings                            string
	dictionaryHash, moraDurationsHash   string
}

func optionalBoolKey(value *bool) string {
	if value == nil {
		return "nil"
	}
	if *value {
		return "true"
	}
	return "false"
}

func rendererCapabilityKey(caps *plugin.Capabilities) string {
	if caps == nil {
		return "nil"
	}
	return fmt.Sprintf("it=%v", caps.InternalTiming)
}

func hashStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hasher := fnv.New64a()
	for _, key := range keys {
		_, _ = hasher.Write([]byte(key))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(values[key]))
		_, _ = hasher.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hasher.Sum64())
}

func hashFloatSlice(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	hasher := fnv.New64a()
	for _, value := range values {
		_, _ = fmt.Fprintf(hasher, "%v,", value)
	}
	return fmt.Sprintf("%x", hasher.Sum64())
}

func prosodyComputationKeyFor(cfg Config) prosodyComputationKey {
	settings := fmt.Sprintf(
		"mora=%v pause=%v/%v pitchonly=%v apply=%v strength=%v release=%v cap=%v "+
			"cd=%v cds=%v bt=%v bts=%v sa=%v sas=%v pc=%v pcs=%v tone=%v color=%v",
		cfg.MoraDurationMS, cfg.PauseDurationMS, cfg.PauseDurationSet, cfg.ProsodyPitchOnly, cfg.ApplyPitch,
		cfg.IntonationStrength, cfg.ReleaseMS, rendererCapabilityKey(cfg.RendererCapabilities),
		optionalBoolKey(cfg.ContextDuration), cfg.ContextDurationStrength, optionalBoolKey(cfg.BoundaryTone), cfg.BoundaryToneStrength,
		optionalBoolKey(cfg.StretchAdapt), cfg.StretchAdaptStrength, optionalBoolKey(cfg.PauseContext), cfg.PauseContextStrength,
		cfg.Tone, cfg.Color)
	return prosodyComputationKey{
		text: cfg.Text, reading: cfg.Reading, language: cfg.Language, phonemizer: cfg.Phonemizer,
		modelPath: cfg.ProsodyModelPath, renderer: cfg.Renderer, voicebankPath: cfg.VoicebankPath,
		openJTalkPath: cfg.OpenJTalkPath, openJTalkDictionary: cfg.OpenJTalkDictionaryPath,
		settings:          settings,
		dictionaryHash:    hashStringMap(cfg.Dictionary),
		moraDurationsHash: hashFloatSlice(cfg.MoraDurationsMS),
	}
}

func moraeEqual(a, b []frontend.Mora) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index].Text != b[index].Text || a[index].Pause != b[index].Pause ||
			a[index].Vowel != b[index].Vowel || a[index].DurationScale != b[index].DurationScale {
			return false
		}
	}
	return true
}

func resolveProsodyComputation(cfg Config, profile languageProfile, model *prosody.Model, morae []frontend.Mora, reading string) ([]prosody.FeatureFrame, []prosody.Prediction, error) {
	if len(cfg.ProsodyFeatures) > 0 {
		features := cfg.ProsodyFeatures
		predictions, err := predictMorae(cfg, profile, model, morae, features)
		return features, predictions, err
	}
	cache, err := cacheFor(cfg.Context)
	if err != nil {
		return nil, nil, err
	}
	key := prosodyComputationKeyFor(cfg)
	cache.RLock()
	entry, ok := cache.computations[key]
	cache.RUnlock()
	if ok && moraeEqual(entry.morae, morae) {
		return entry.features, entry.predictions, nil
	}
	features, err := resolveProsodyFeatures(cfg, model, morae, reading)
	if err != nil {
		return nil, nil, err
	}
	predictions, err := predictMorae(cfg, profile, model, morae, features)
	if err != nil {
		return nil, nil, err
	}
	cache.Lock()
	if _, exists := cache.computations[key]; !exists {
		cache.computationOrder = append(cache.computationOrder, key)
	}
	cache.computations[key] = prosodyComputation{
		morae: append([]frontend.Mora(nil), morae...), features: features, predictions: predictions,
	}
	if len(cache.computationOrder) > maxProsodyCacheEntries {
		oldest := cache.computationOrder[0]
		cache.computationOrder = cache.computationOrder[1:]
		delete(cache.computations, oldest)
	}
	cache.Unlock()
	return features, predictions, nil
}

func loadVoicebankCached(path string) (*voicebank.Bank, error) {
	return loadVoicebankCachedContext(nil, path)
}

func loadVoicebankCachedContext(ctx context.Context, path string) (*voicebank.Bank, error) {
	cache, err := cacheFor(ctx)
	if err != nil {
		return nil, err
	}
	key, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cache.RLock()
	bank := cache.banks[key]
	cache.RUnlock()
	if bank != nil {
		return bank, nil
	}
	bank, err = voicebank.Load(key)
	if err != nil {
		return nil, err
	}
	cache.Lock()
	if existing := cache.banks[key]; existing != nil {
		bank = existing
	} else {
		cache.banks[key] = bank
	}
	cache.Unlock()
	return bank, nil
}

func loadProsodyModelCached(ctx context.Context, path string) (*prosody.Model, error) {
	cache, err := cacheFor(ctx)
	if err != nil {
		return nil, err
	}
	key, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(key)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime().UnixNano()
	cache.RLock()
	entry, ok := cache.models[key]
	cache.RUnlock()
	if ok && entry.size == info.Size() && entry.modTime == modTime && entry.model != nil {
		return entry.model, nil
	}
	model, err := prosody.LoadModel(key)
	if err != nil {
		return nil, err
	}
	cache.Lock()
	if old := cache.models[key].model; old != nil && old.F0Head != nil && old.F0Head != model.F0Head {
		cache.forgetUnifiedModelLocked(old.F0Head)
	}
	cache.models[key] = modelCacheEntry{size: info.Size(), modTime: modTime, model: model}
	cache.Unlock()
	return model, nil
}

func analyzeOpenJTalkCached(ctx context.Context, text string, cfg openjtalk.Config) (*openjtalk.Analysis, error) {
	cache, err := cacheFor(ctx)
	if err != nil {
		return nil, err
	}
	key := analysisCacheKey{text: text, helper: cfg.HelperPath, dictionary: cfg.DictionaryPath}
	cache.RLock()
	analysis := cache.analyses[key]
	cache.RUnlock()
	if analysis != nil {
		return analysis, nil
	}
	analysis, err = openjtalk.AnalyzeContext(ctx, text, cfg)
	if err != nil {
		return nil, err
	}
	cache.Lock()
	if existing := cache.analyses[key]; existing != nil {
		analysis = existing
	} else {
		cache.analyses[key] = analysis
		cache.analysisOrder = append(cache.analysisOrder, key)
		if len(cache.analysisOrder) > maxAnalysisCacheEntries {
			oldest := cache.analysisOrder[0]
			cache.analysisOrder = cache.analysisOrder[1:]
			delete(cache.analyses, oldest)
		}
	}
	cache.Unlock()
	return analysis, nil
}

func ClearCaches() {
	// 同じパスの旧モデルが子プロセスに残らないよう、ONNXセッションも閉じる。
	_ = neural.CloseSessions()
	synthesisCache.clear()
	render.ClearWAVCache()
}

func (cache *synthesisCacheState) clear() {
	cache.Lock()
	defer cache.Unlock()
	cache.banks = make(map[string]*voicebank.Bank)
	cache.models = make(map[string]modelCacheEntry)
	cache.analyses = make(map[analysisCacheKey]*openjtalk.Analysis)
	cache.analysisOrder = nil
	cache.computations = make(map[prosodyComputationKey]prosodyComputation)
	cache.computationOrder = nil
	cache.unified = make(map[unifiedProsodyCacheKey]unifiedProsodyPrediction)
	cache.unifiedOrder = nil
	cache.unifiedBytes = 0
}
