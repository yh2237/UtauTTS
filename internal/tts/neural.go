package tts

import "utautts/internal/engine"

// NeuralSynthesizerはUTAU Unit Planではなくニューラル歌唱スコアから音声を構築するエンジンのprovider契約。Configはprovider固有処理前の共通入力。
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
