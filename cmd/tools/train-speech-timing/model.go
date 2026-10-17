package main

import (
	"fmt"
	"math/rand"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
)

var SpeechTimingDilations = []int{1, 2, 4, 8, 1, 2, 4, 8}

// SpeechTiming composes the UtauTTS Target model. Activations use [batch,time,channel].
// Its Module emits the exact PyTorch state_dict names expected by LoadTCN.
type SpeechTiming struct {
	Module        autograd.Module
	Phone         *autograd.EmbeddingLayer
	Input, Output *autograd.Conv1dLayer
	Blocks        []*autograd.Conv1dLayer
	Norms         []*autograd.LayerNormLayer
	Dropout       autograd.DropoutLayer
	GELU          autograd.GELULayer
	Continuous    int
}

// NewSpeechTiming builds the UtauTTS Target model with the original 40-phone vocabulary.
func NewSpeechTiming(continuous int, device tensor.Device, seed int64) (*SpeechTiming, error) {
	return NewSpeechTimingWithPhones(40, continuous, device, seed)
}

// NewSpeechTimingWithPhones builds the model for a language-specific phone vocabulary.
// phones must match the embedding size expected by the runtime model metadata.
func NewSpeechTimingWithPhones(phones, continuous int, device tensor.Device, seed int64) (*SpeechTiming, error) {
	if continuous < 1 {
		return nil, fmt.Errorf("autograd: invalid continuous width")
	}
	if phones < 1 {
		return nil, fmt.Errorf("autograd: invalid phone count")
	}
	rng := rand.New(rand.NewSource(seed))
	m := &SpeechTiming{Continuous: continuous, Dropout: autograd.DropoutLayer{Probability: .15, Training: true}}
	var e error
	if m.Phone, e = autograd.NewEmbeddingLayer(phones, 32, -1, device, rng); e != nil {
		return nil, e
	}
	if m.Input, e = autograd.NewConv1dLayer(96+continuous, 128, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "phone", Module: m.Phone.StateModule()}, autograd.NamedModule{Name: "inp", Module: m.Input.StateModule()})
	for i, d := range SpeechTimingDilations {
		block, e := autograd.NewConv1dLayer(128, 128, 5, d, device, rng)
		if e != nil {
			return nil, e
		}
		norm, e := autograd.NewLayerNormLayer(128, 1e-5, device)
		if e != nil {
			return nil, e
		}
		m.Blocks = append(m.Blocks, block)
		m.Norms = append(m.Norms, norm)
		m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: fmt.Sprintf("blocks.%d", i), Module: block.StateModule()}, autograd.NamedModule{Name: fmt.Sprintf("norms.%d", i), Module: norm.StateModule()})
	}
	if m.Output, e = autograd.NewConv1dLayer(128, 80, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "out", Module: m.Output.StateModule()})
	m.Module.OnTrainingChange = m.Dropout.Train
	m.Module.Train(true)
	return m, nil
}
func (m *SpeechTiming) Parameters() []autograd.Parameter { return m.Module.NamedParameters() }
func (m *SpeechTiming) Train(training bool)              { m.Module.Train(training) }
func (m *SpeechTiming) Forward(ids []int, cont *autograd.Tensor, seed uint32) *autograd.Tensor {
	s := cont.Shape
	if len(s) != 3 || s[2] != m.Continuous || len(ids) != s[0]*s[1]*3 {
		panic("autograd: speech timing input shape")
	}
	phone := autograd.Reshape(m.Phone.Forward(ids, []int{s[0], s[1], 3}), s[0], s[1], 96)
	h := m.Input.Forward(autograd.Concat(2, phone, cont))
	for i := range m.Blocks {
		y := m.Blocks[i].Forward(h)
		y = m.Norms[i].Forward(y)
		y = m.GELU.Forward(y)
		y = m.Dropout.Forward(y, seed+uint32(i))
		h = autograd.Add(h, y)
	}
	return m.Output.Forward(h)
}

// SpeechTimingMultiHead adds a context-only F0 branch to SpeechTiming.
// The F0 branch never receives F0 inputs, so it cannot copy the answer.
type SpeechTimingMultiHead struct {
	Module        autograd.Module
	Phone         *autograd.EmbeddingLayer
	Input, Output *autograd.Conv1dLayer
	Blocks        []*autograd.Conv1dLayer
	Norms         []*autograd.LayerNormLayer
	Dropout       autograd.DropoutLayer
	GELU          autograd.GELULayer
	Continuous    int

	F0Input   *autograd.Conv1dLayer
	F0Blocks  []*autograd.Conv1dLayer
	F0Norms   []*autograd.LayerNormLayer
	F0Output  *autograd.Conv1dLayer
	F0Context int

	EnergyOutput *autograd.Conv1dLayer
}

// NewSpeechTimingMultiHead builds the mel trunk plus a context-only F0 trunk.
// f0Context is the width of the F0-branch continuous input (position and duration).
func NewSpeechTimingMultiHead(phones, continuous, f0Context int, device tensor.Device, seed int64) (*SpeechTimingMultiHead, error) {
	return NewSpeechTimingMultiHeadWithF0Dilations(phones, continuous, f0Context, SpeechTimingDilations, device, seed)
}

// NewSpeechTimingMultiHeadWithF0Dilations is NewSpeechTimingMultiHead with a custom
// dilation schedule for the F0 trunk, which sets how much sentence context the F0
// and energy heads see. The mel trunk keeps SpeechTimingDilations.
func NewSpeechTimingMultiHeadWithF0Dilations(phones, continuous, f0Context int, f0Dilations []int, device tensor.Device, seed int64) (*SpeechTimingMultiHead, error) {
	if len(f0Dilations) == 0 {
		return nil, fmt.Errorf("autograd: empty f0 dilation schedule")
	}
	for _, d := range f0Dilations {
		if d < 1 {
			return nil, fmt.Errorf("autograd: invalid f0 dilation %d", d)
		}
	}
	if continuous < 1 {
		return nil, fmt.Errorf("autograd: invalid continuous width")
	}
	if phones < 1 {
		return nil, fmt.Errorf("autograd: invalid phone count")
	}
	if f0Context < 1 {
		return nil, fmt.Errorf("autograd: invalid f0 context width")
	}
	rng := rand.New(rand.NewSource(seed))
	m := &SpeechTimingMultiHead{Continuous: continuous, F0Context: f0Context, Dropout: autograd.DropoutLayer{Probability: .15, Training: true}}
	var e error
	if m.Phone, e = autograd.NewEmbeddingLayer(phones, 32, -1, device, rng); e != nil {
		return nil, e
	}
	if m.Input, e = autograd.NewConv1dLayer(96+continuous, 128, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "phone", Module: m.Phone.StateModule()}, autograd.NamedModule{Name: "inp", Module: m.Input.StateModule()})
	for i, d := range SpeechTimingDilations {
		block, e := autograd.NewConv1dLayer(128, 128, 5, d, device, rng)
		if e != nil {
			return nil, e
		}
		norm, e := autograd.NewLayerNormLayer(128, 1e-5, device)
		if e != nil {
			return nil, e
		}
		m.Blocks = append(m.Blocks, block)
		m.Norms = append(m.Norms, norm)
		m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: fmt.Sprintf("blocks.%d", i), Module: block.StateModule()}, autograd.NamedModule{Name: fmt.Sprintf("norms.%d", i), Module: norm.StateModule()})
	}
	if m.Output, e = autograd.NewConv1dLayer(128, 80, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "out", Module: m.Output.StateModule()})
	if m.F0Input, e = autograd.NewConv1dLayer(96+f0Context, 128, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "f0_inp", Module: m.F0Input.StateModule()})
	for i, d := range f0Dilations {
		block, e := autograd.NewConv1dLayer(128, 128, 5, d, device, rng)
		if e != nil {
			return nil, e
		}
		norm, e := autograd.NewLayerNormLayer(128, 1e-5, device)
		if e != nil {
			return nil, e
		}
		m.F0Blocks = append(m.F0Blocks, block)
		m.F0Norms = append(m.F0Norms, norm)
		m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: fmt.Sprintf("f0_blocks.%d", i), Module: block.StateModule()}, autograd.NamedModule{Name: fmt.Sprintf("f0_norms.%d", i), Module: norm.StateModule()})
	}
	if m.F0Output, e = autograd.NewConv1dLayer(128, 1, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "f0_out", Module: m.F0Output.StateModule()})
	if m.EnergyOutput, e = autograd.NewConv1dLayer(128, 1, 1, 1, device, rng); e != nil {
		return nil, e
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "energy_out", Module: m.EnergyOutput.StateModule()})
	m.Module.OnTrainingChange = m.Dropout.Train
	m.Module.Train(true)
	return m, nil
}

func (m *SpeechTimingMultiHead) Parameters() []autograd.Parameter { return m.Module.NamedParameters() }
func (m *SpeechTimingMultiHead) Train(training bool)              { m.Module.Train(training) }

// Forward returns the mel prediction, the context-only F0 prediction and the
// context-only energy prediction (centered dB/10).
func (m *SpeechTimingMultiHead) Forward(ids []int, cont, f0Cont *autograd.Tensor, seed uint32) (*autograd.Tensor, *autograd.Tensor, *autograd.Tensor) {
	s := cont.Shape
	if len(s) != 3 || s[2] != m.Continuous || len(ids) != s[0]*s[1]*3 {
		panic("autograd: speech timing input shape")
	}
	phone := autograd.Reshape(m.Phone.Forward(ids, []int{s[0], s[1], 3}), s[0], s[1], 96)
	h := m.Input.Forward(autograd.Concat(2, phone, cont))
	for i := range m.Blocks {
		y := m.Blocks[i].Forward(h)
		y = m.Norms[i].Forward(y)
		y = m.GELU.Forward(y)
		y = m.Dropout.Forward(y, seed+uint32(i))
		h = autograd.Add(h, y)
	}
	mel := m.Output.Forward(h)

	f := f0Cont.Shape
	if len(f) != 3 || f[2] != m.F0Context || f[0] != s[0] || f[1] != s[1] {
		panic("autograd: speech timing f0 input shape")
	}
	g := m.F0Input.Forward(autograd.Concat(2, phone, f0Cont))
	for i := range m.F0Blocks {
		y := m.F0Blocks[i].Forward(g)
		y = m.F0Norms[i].Forward(y)
		y = m.GELU.Forward(y)
		y = m.Dropout.Forward(y, seed+uint32(64+i))
		g = autograd.Add(g, y)
	}
	return mel, m.F0Output.Forward(g), m.EnergyOutput.Forward(g)
}
