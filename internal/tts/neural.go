package tts

import (
	"sync"

	"utautts/internal/engine"
	"utautts/internal/neural"

	// DiffSingerなどのprovider実装は低層パッケージ側で自身を登録する。
	_ "utautts/internal/diffsinger/adapter"
)

// NeuralSynthesizerはUTAU Unit Planではなくニューラル歌唱スコアから音声を構築するエンジンのprovider契約。Configはprovider固有処理前の共通入力。
type NeuralSynthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Config) (*Result, error)
}

// NeuralSynthesizerFactoryはprovider実装を組み立てる。登録側が実装型に依存せずに済む。
type NeuralSynthesizerFactory func() NeuralSynthesizer

// neuralSynthesizersはtts内部のprovider IDレジストリ。登録はinitや起動時にRegisterNeuralSynthesizerで行う。
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
	if found {
		return factory(), true
	}
	// 低層レジストリのproviderはConfig変換アダプタで包んで公開契約へ適合させる。
	if synthesizer, found := neural.ForProvider(id); found {
		return lowLevelNeuralSynthesizer{inner: synthesizer}, true
	}
	return nil, false
}

// lowLevelNeuralSynthesizerはConfigを低層DTOへ変換し、provider非依存の結果をResultへ戻す。
type lowLevelNeuralSynthesizer struct {
	inner neural.Synthesizer
}

func (s lowLevelNeuralSynthesizer) ProviderID() engine.ProviderID { return s.inner.ProviderID() }

func (s lowLevelNeuralSynthesizer) Synthesize(cfg Config) (*Result, error) {
	input, err := neuralInputFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	output, err := s.inner.Synthesize(input)
	if err != nil {
		return nil, err
	}
	return &Result{
		Plan:            output.Plan,
		Audio:           output.Audio,
		MoraDurationsMS: output.MoraDurationsMS,
		MoraPositionsMS: output.MoraPositionsMS,
		PitchPoints:     output.PitchPoints,
	}, nil
}
