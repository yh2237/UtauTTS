//go:build windows

package hubert

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/cuda"
	"github.com/yh2237/gograd/tensor"
)

// gograd's CUDA device and stream are local to an OS thread. The index builder
// may call different models concurrently, so the lock covers all GPU work.
var hubertGPUMu sync.Mutex

type hubertCUDAState struct {
	weights map[string]*autograd.Tensor
}

func (s *hubertCUDAState) close() {
	for _, w := range s.weights {
		w.Close()
	}
}

func (m *Model) cudaWeight(s *hubertCUDAState, name string, shape ...int) (*autograd.Tensor, error) {
	if w := s.weights[name]; w != nil {
		return w, nil
	}
	values, err := m.get(name)
	if err != nil {
		return nil, err
	}
	w, err := autograd.New(values, shape, tensor.CUDA, false)
	if err != nil {
		return nil, fmt.Errorf("hubert: upload %s: %w", name, err)
	}
	s.weights[name] = w
	return w, nil
}

func (m *Model) cudaLinear(s *hubertCUDAState, x *autograd.Tensor, prefix string, in, out int) (*autograd.Tensor, error) {
	w, err := m.cudaWeight(s, prefix+".weight", out, in)
	if err != nil {
		return nil, err
	}
	b, err := m.cudaWeight(s, prefix+".bias", out)
	if err != nil {
		return nil, err
	}
	return autograd.Add(autograd.MatMul(x, autograd.Transpose(w, 0, 1)), b), nil
}

func (m *Model) cudaNorm(s *hubertCUDAState, x *autograd.Tensor, prefix string, width int) (*autograd.Tensor, error) {
	w, err := m.cudaWeight(s, prefix+".weight", width)
	if err != nil {
		return nil, err
	}
	b, err := m.cudaWeight(s, prefix+".bias", width)
	if err != nil {
		return nil, err
	}
	return autograd.LayerNorm(x, w, b, 1e-5), nil
}

func (m *Model) cudaPos(s *hubertCUDAState, x *autograd.Tensor, frames int) (*autograd.Tensor, error) {
	v, err := m.get("encoder.pos_conv_embed.conv.weight_v")
	if err != nil {
		v, err = m.get("encoder.pos_conv_embed.conv.parametrizations.weight.original1")
		if err != nil {
			return nil, err
		}
	}
	g, err := m.get("encoder.pos_conv_embed.conv.weight_g")
	if err != nil {
		g, err = m.get("encoder.pos_conv_embed.conv.parametrizations.weight.original0")
		if err != nil {
			return nil, err
		}
	}
	values, err := m.positionalWeights(v, g)
	if err != nil {
		return nil, err
	}
	w := s.weights["pos.weight"]
	if w == nil {
		w, err = autograd.New(values, []int{768, 48, 128}, tensor.CUDA, false)
		if err != nil {
			return nil, err
		}
		s.weights["pos.weight"] = w
	}
	b, err := m.cudaWeight(s, "encoder.pos_conv_embed.conv.bias", 768)
	if err != nil {
		return nil, err
	}
	parts := make([]*autograd.Tensor, 16)
	for group := range parts {
		input := autograd.Slice(x, 2, group*48, (group+1)*48)
		weight := autograd.Slice(w, 0, group*48, (group+1)*48)
		bias := autograd.Slice(b, 0, group*48, (group+1)*48)
		// Padding 64 yields one extra frame for the even kernel.
		conv := autograd.Conv1dGEMMStride(input, weight, bias, 1, 64, 1)
		parts[group] = autograd.Slice(conv, 1, 0, frames)
	}
	return autograd.GELU(autograd.Concat(2, parts...), false), nil
}

func (m *Model) cudaEncoder(s *hubertCUDAState, x *autograd.Tensor, frames, index int) (*autograd.Tensor, error) {
	base := fmt.Sprintf("encoder.layers.%d", index)
	q, err := m.cudaLinear(s, x, base+".attention.q_proj", 768, 768)
	if err != nil {
		return nil, err
	}
	k, err := m.cudaLinear(s, x, base+".attention.k_proj", 768, 768)
	if err != nil {
		return nil, err
	}
	v, err := m.cudaLinear(s, x, base+".attention.v_proj", 768, 768)
	if err != nil {
		return nil, err
	}
	heads := func(z *autograd.Tensor) *autograd.Tensor {
		return autograd.Permute(autograd.Reshape(z, 1, frames, 12, 64), 0, 2, 1, 3)
	}
	scores := autograd.MulScalar(autograd.MatMul(heads(q), autograd.Transpose(heads(k), 2, 3)), 0.125)
	if m.config.RelativeBias {
		bias, err := m.cudaRelativeBias(s, x, frames, index)
		if err != nil {
			return nil, err
		}
		scores = autograd.Add(scores, bias)
	}
	context := autograd.MatMul(autograd.Softmax(scores, 3), heads(v))
	context = autograd.Reshape(autograd.Permute(context, 0, 2, 1, 3), 1, frames, 768)
	attention, err := m.cudaLinear(s, context, base+".attention.out_proj", 768, 768)
	if err != nil {
		return nil, err
	}
	x = autograd.Add(x, attention)
	x, err = m.cudaNorm(s, x, base+".layer_norm", 768)
	if err != nil {
		return nil, err
	}
	f, err := m.cudaLinear(s, x, base+".feed_forward.intermediate_dense", 768, 3072)
	if err != nil {
		return nil, err
	}
	f = autograd.GELU(f, false)
	f, err = m.cudaLinear(s, f, base+".feed_forward.output_dense", 3072, 768)
	if err != nil {
		return nil, err
	}
	x = autograd.Add(x, f)
	return m.cudaNorm(s, x, base+".final_layer_norm", 768)
}

// cudaRelativeBiasは、WavLMの相対位置バイアスにゲートを掛けたものを返す。
// バイアス表はフレーム数ごとに使い回し、ゲートは層の入力から計算する。
func (m *Model) cudaRelativeBias(s *hubertCUDAState, x *autograd.Tensor, frames, index int) (*autograd.Tensor, error) {
	config := m.config
	heads, width := config.Heads, config.HiddenSize
	headDim := width / heads
	key := fmt.Sprintf("rel.bias.%d", frames)
	table := s.weights[key]
	if table == nil {
		values, err := m.relativeBias(frames)
		if err != nil {
			return nil, err
		}
		ordered := make([]float32, len(values))
		for t := 0; t < frames; t++ {
			for u := 0; u < frames; u++ {
				for h := 0; h < heads; h++ {
					ordered[h*frames*frames+t*frames+u] = values[(t*frames+u)*heads+h]
				}
			}
		}
		table, err = autograd.New(ordered, []int{1, heads, frames, frames}, tensor.CUDA, false)
		if err != nil {
			return nil, err
		}
		s.weights[key] = table
	}
	base := fmt.Sprintf("encoder.layers.%d", index)
	gateWeight, err := m.get(base + ".attention.gru_rel_pos_linear.weight")
	if err != nil {
		return nil, err
	}
	gateBias, err := m.get(base + ".attention.gru_rel_pos_linear.bias")
	if err != nil {
		return nil, err
	}
	constWeight, err := m.get(base + ".attention.gru_rel_pos_const")
	if err != nil {
		return nil, err
	}
	host, err := x.ToHost()
	if err != nil {
		return nil, err
	}
	gate := make([]float32, heads*frames)
	for t := 0; t < frames; t++ {
		for h := 0; h < heads; h++ {
			var first, second float32
			for o := 0; o < 8; o++ {
				sum := gateBias[o]
				for d := 0; d < headDim; d++ {
					sum += host[t*width+h*headDim+d] * gateWeight[o*headDim+d]
				}
				if o < 4 {
					first += sum
				} else {
					second += sum
				}
			}
			gateA := float32(1 / (1 + math.Exp(-float64(first))))
			gateB := float32(1 / (1 + math.Exp(-float64(second))))
			gate[h*frames+t] = gateA*(gateB*constWeight[h]-1) + 2
		}
	}
	gateTensor, err := autograd.New(gate, []int{1, heads, frames, 1}, tensor.CUDA, false)
	if err != nil {
		return nil, err
	}
	expanded := autograd.Expand(gateTensor, 1, heads, frames, frames)
	return autograd.Mul(table, expanded), nil
}

// FeaturesCUDA returns hidden_states[layer] as [frames][768] rows on the host.
func (m *Model) FeaturesCUDA(wave []float32, layer int) ([][]float32, error) {
	if layer < 0 || layer > 12 {
		return nil, fmt.Errorf("hubert: layer must be 0..12")
	}
	if len(wave) < 400 {
		return nil, fmt.Errorf("hubert: waveform too short")
	}
	hubertGPUMu.Lock()
	defer hubertGPUMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !cuda.Available() {
		return nil, cuda.ErrUnavailable
	}
	if err := cuda.SetDevice(0); err != nil {
		return nil, err
	}
	m.cudaMu.Lock()
	s, _ := m.cudaState.(*hubertCUDAState)
	if s == nil {
		s = &hubertCUDAState{weights: make(map[string]*autograd.Tensor)}
		runtime.SetFinalizer(s, (*hubertCUDAState).close)
		m.cudaState = s
	}
	m.cudaMu.Unlock()

	x, err := autograd.New(wave, []int{1, len(wave), 1}, tensor.CUDA, false)
	if err != nil {
		return nil, err
	}
	defer x.Close()
	defer func() { x.ReleaseGraph() }()
	kernels := [7]int{10, 3, 3, 3, 3, 2, 2}
	strides := [7]int{5, 2, 2, 2, 2, 2, 2}
	frames, channels := len(wave), 1
	zero := s.weights["conv.zero"]
	if zero == nil {
		zero, err = autograd.New(make([]float32, 512), []int{512}, tensor.CUDA, false)
		if err != nil {
			return nil, err
		}
		s.weights["conv.zero"] = zero
	}
	for i := 0; i < 7; i++ {
		base := fmt.Sprintf("feature_extractor.conv_layers.%d", i)
		w, e := m.cudaWeight(s, base+".conv.weight", 512, channels, kernels[i])
		if e != nil {
			return nil, e
		}
		x = autograd.Conv1dGEMMStride(x, w, zero, strides[i], 0, 1)
		frames = (frames-kernels[i])/strides[i] + 1
		channels = 512
		if i == 0 {
			w, e := m.cudaWeight(s, base+".layer_norm.weight", 512)
			if e != nil {
				return nil, e
			}
			b, e := m.cudaWeight(s, base+".layer_norm.bias", 512)
			if e != nil {
				return nil, e
			}
			x = autograd.GroupNorm(x, w, b, 512, 1e-5)
		}
		x = autograd.GELU(x, false)
	}
	x, err = m.cudaNorm(s, x, "feature_projection.layer_norm", 512)
	if err != nil {
		return nil, err
	}
	x, err = m.cudaLinear(s, x, "feature_projection.projection", 512, 768)
	if err != nil {
		return nil, err
	}
	pos, err := m.cudaPos(s, x, frames)
	if err != nil {
		return nil, err
	}
	x = autograd.Add(x, pos)
	x, err = m.cudaNorm(s, x, "encoder.layer_norm", 768)
	if err != nil {
		return nil, err
	}
	for i := 0; i < layer; i++ {
		x, err = m.cudaEncoder(s, x, frames, i)
		if err != nil {
			return nil, err
		}
	}
	values, err := x.ToHost()
	if err != nil {
		return nil, err
	}
	rows := make([][]float32, frames)
	for i := range rows {
		rows[i] = values[i*768 : (i+1)*768]
	}
	return rows, nil
}
