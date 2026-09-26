package tts

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"utautts/internal/frontend"
	"utautts/internal/neural"
	"utautts/internal/openjtalk"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

// GUIの反復合成で変わらない高コストな入力をプロセス内に保持する。

const maxAnalysisCacheEntries = 512
const maxProsodyCacheEntries = 128

var synthesisCache = struct {
	sync.RWMutex
	banks            map[string]*voicebank.Bank
	models           map[string]modelCacheEntry
	analyses         map[analysisCacheKey]*openjtalk.Analysis
	analysisOrder    []analysisCacheKey
	computations     map[prosodyComputationKey]prosodyComputation
	computationOrder []prosodyComputationKey
}{
	banks:        make(map[string]*voicebank.Bank),
	models:       make(map[string]modelCacheEntry),
	analyses:     make(map[analysisCacheKey]*openjtalk.Analysis),
	computations: make(map[prosodyComputationKey]prosodyComputation),
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

// prosodyComputationはプレビューと本合成で共有できるプロソディ入力。
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
	return fmt.Sprintf("it=%v sp=%v", caps.InternalTiming, caps.SpeechProsodyExperiment)
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
		"mora=%v pause=%v pitchonly=%v apply=%v strength=%v release=%v speech=%v cap=%v wbe=%v spx=%q "+
			"cd=%v cds=%v bt=%v bts=%v sa=%v sas=%v pc=%v pcs=%v ewf=%v tone=%v color=%v",
		cfg.MoraDurationMS, cfg.PauseDurationMS, cfg.ProsodyPitchOnly, cfg.ApplyPitch,
		cfg.IntonationStrength, cfg.ReleaseMS, cfg.SpeechTiming, rendererCapabilityKey(cfg.RendererCapabilities),
		cfg.WordBoundaryEnvelope, cfg.SpeechProsodyExperiment,
		cfg.ContextDuration, cfg.ContextDurationStrength, cfg.BoundaryTone, cfg.BoundaryToneStrength,
		cfg.StretchAdapt, cfg.StretchAdaptStrength, cfg.PauseContext, cfg.PauseContextStrength,
		optionalBoolKey(cfg.EnglishWeakForm), cfg.Tone, cfg.Color)
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

// resolveProsodyComputationは特徴量と予測をまとめて解決し、プレビューと本合成で再利用する。
func resolveProsodyComputation(cfg Config, profile languageProfile, model *prosody.Model, morae []frontend.Mora, reading string) ([]prosody.FeatureFrame, []prosody.Prediction, error) {
	if len(cfg.ProsodyFeatures) > 0 {
		features := cfg.ProsodyFeatures
		predictions, err := predictMorae(cfg, profile, model, morae, features)
		return features, predictions, err
	}
	key := prosodyComputationKeyFor(cfg)
	synthesisCache.RLock()
	entry, ok := synthesisCache.computations[key]
	synthesisCache.RUnlock()
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
	synthesisCache.Lock()
	synthesisCache.computations[key] = prosodyComputation{
		morae: append([]frontend.Mora(nil), morae...), features: features, predictions: predictions,
	}
	synthesisCache.computationOrder = append(synthesisCache.computationOrder, key)
	if len(synthesisCache.computationOrder) > maxProsodyCacheEntries {
		oldest := synthesisCache.computationOrder[0]
		synthesisCache.computationOrder = synthesisCache.computationOrder[1:]
		delete(synthesisCache.computations, oldest)
	}
	synthesisCache.Unlock()
	return features, predictions, nil
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
	// DiffSinger bridgeはモデルパス単位でONNXセッションを保持するため、音源/モデル再読込後に同一パスの旧モデルが子プロセスに残らないようここで閉じる。
	_ = neural.CloseSessions()
	synthesisCache.Lock()
	synthesisCache.banks = make(map[string]*voicebank.Bank)
	synthesisCache.models = make(map[string]modelCacheEntry)
	synthesisCache.analyses = make(map[analysisCacheKey]*openjtalk.Analysis)
	synthesisCache.analysisOrder = nil
	synthesisCache.computations = make(map[prosodyComputationKey]prosodyComputation)
	synthesisCache.computationOrder = nil
	synthesisCache.Unlock()
	render.ClearWAVCache()
}
