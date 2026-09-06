package tts

import "utautts/internal/engine"

// NeuralSynthesizer is the provider dispatch contract for engines that build
// audio from a neural singing score rather than a selected UTAU Unit Plan.
// Config remains the compatibility envelope until provider-specific synthesis
// configuration is moved out of tts entirely.
type NeuralSynthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Config) (*Result, error)
}

type diffSingerNeuralSynthesizer struct{}

func (diffSingerNeuralSynthesizer) ProviderID() engine.ProviderID {
	return "diffsinger"
}

func (diffSingerNeuralSynthesizer) Synthesize(cfg Config) (*Result, error) {
	return synthesizeDiffSinger(cfg)
}

var neuralSynthesizers = map[engine.ProviderID]NeuralSynthesizer{
	"diffsinger": diffSingerNeuralSynthesizer{},
}

func neuralSynthesizerForProvider(id engine.ProviderID) (NeuralSynthesizer, bool) {
	synthesizer, found := neuralSynthesizers[id]
	return synthesizer, found
}
