package tts

import (
	"sync"

	"utautts/internal/engine"
)

// NeuralSynthesizerはUTAU Unit Planではなくニューラル歌唱スコアから音声を構築するエンジンのprovider契約。Configはprovider固有処理前の共通入力。
type NeuralSynthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Config) (*Result, error)
}

// NeuralSynthesizerFactoryはprovider実装を組み立てる。登録側が実装型に依存せずに済む。
type NeuralSynthesizerFactory func() NeuralSynthesizer

// neuralSynthesizersはprovider IDから実装を引くレジストリ。登録はinitや起動時にRegisterNeuralSynthesizerで行う。
var (
	neuralSynthesizersMu sync.RWMutex
	neuralSynthesizers   = map[engine.ProviderID]NeuralSynthesizerFactory{}
)

// RegisterNeuralSynthesizerはprovider実装をprovider IDで登録する。同一IDの再登録は上書きする。
func RegisterNeuralSynthesizer(id engine.ProviderID, factory NeuralSynthesizerFactory) {
	if id == "" || factory == nil {
		panic("tts: neural synthesizer registration requires a provider id and factory")
	}
	neuralSynthesizersMu.Lock()
	defer neuralSynthesizersMu.Unlock()
	neuralSynthesizers[id] = factory
}

// neuralSynthesizerForProviderは登録済みproviderを解決する。未登録はfound=falseを返す。
func neuralSynthesizerForProvider(id engine.ProviderID) (NeuralSynthesizer, bool) {
	neuralSynthesizersMu.RLock()
	factory, found := neuralSynthesizers[id]
	neuralSynthesizersMu.RUnlock()
	if !found {
		return nil, false
	}
	return factory(), true
}
