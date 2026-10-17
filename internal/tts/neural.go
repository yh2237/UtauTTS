package tts

import (
	"sync"

	"github.com/yh2237/UtauTTS/internal/engine"
	"github.com/yh2237/UtauTTS/internal/neural"

	// DiffSingerなどのprovider実装は低層パッケージ側で自身を登録する。
	_ "github.com/yh2237/UtauTTS/internal/diffsinger/adapter"
)

type NeuralSynthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Config) (*Result, error)
}

type NeuralSynthesizerFactory func() NeuralSynthesizer

var (
	neuralSynthesizersMu sync.RWMutex
	neuralSynthesizers   = map[engine.ProviderID]NeuralSynthesizerFactory{}
)

func neuralSynthesizerForProvider(id engine.ProviderID) (NeuralSynthesizer, bool) {
	neuralSynthesizersMu.RLock()
	factory, found := neuralSynthesizers[id]
	neuralSynthesizersMu.RUnlock()
	if found {
		return factory(), true
	}
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
