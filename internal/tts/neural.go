package tts

import (
	"sync"

	"utautts/internal/engine"
	"utautts/internal/neural"

	// DiffSingerなどのprovider実装は低層パッケージ側で自身を登録する。
	_ "utautts/internal/diffsinger/adapter"
)

// ニューラル合成の契約。Configは実装固有処理を適用する前の共通入力。
type NeuralSynthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Config) (*Result, error)
}

type NeuralSynthesizerFactory func() NeuralSynthesizer

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
