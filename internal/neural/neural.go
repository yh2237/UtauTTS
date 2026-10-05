package neural

import (
	"context"
	"errors"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// providerはこの入力だけを参照し、tts.Configに依存しない。
type Input struct {
	Context         context.Context
	VoicebankPath   string
	Text            string
	Tone            string
	Language        string
	Phonemizer      string
	Reading         string
	Morae           []frontend.Mora
	Features        []prosody.FeatureFrame
	MoraDurationsMS []float64
	PitchPoints     []float64
	// PitchCurveはprovider向けのピッチ曲線。自動生成か手動かをAutomaticPitchで示す。
	PitchCurve      *render.PitchCurve
	AutomaticPitch  bool
	PhoneWeights    [][]float64
	ProviderOptions render.ProviderOptions
	Engine          engine.ResolvedEngine
}

type Output struct {
	Plan            *plan.Plan
	Audio           *audio.PCM
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
}

type Synthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Input) (*Output, error)
}

type Factory func() Synthesizer

var (
	mu           sync.RWMutex
	synthesizers = map[engine.ProviderID]Factory{}
)

// 同じIDを登録し直すと上書きする。
func Register(id engine.ProviderID, factory Factory) {
	if id == "" || factory == nil {
		panic("neural: synthesizer registration requires a provider id and factory")
	}
	mu.Lock()
	defer mu.Unlock()
	synthesizers[id] = factory
}

func ForProvider(id engine.ProviderID) (Synthesizer, bool) {
	mu.RLock()
	factory, found := synthesizers[id]
	mu.RUnlock()
	if !found {
		return nil, false
	}
	return factory(), true
}

var (
	closerMu sync.Mutex
	closers  []func() error
)

func RegisterCloser(fn func() error) {
	if fn == nil {
		return
	}
	closerMu.Lock()
	closers = append(closers, fn)
	closerMu.Unlock()
}

func CloseSessions() error {
	closerMu.Lock()
	registered := append([]func() error(nil), closers...)
	closerMu.Unlock()
	var err error
	for index := len(registered) - 1; index >= 0; index-- {
		err = errors.Join(err, registered[index]())
	}
	return err
}
