package main

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
)

type tcn struct {
	Module        autograd.Module
	Input, Output *autograd.LinearLayer
	Layers        []*autograd.Conv1dLayer
	Device        tensor.Device
}

var dilations = []int{1, 2, 4, 8}

func newTCN(features, hidden int, device tensor.Device, seed int64) (*tcn, error) {
	rng := rand.New(rand.NewSource(seed))
	input, e := autograd.NewLinearLayer(features, hidden, device, rng)
	if e != nil {
		return nil, e
	}
	output, e := autograd.NewLinearLayer(hidden, 1, device, rng)
	if e != nil {
		return nil, e
	}
	m := &tcn{Input: input, Output: output, Device: device}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "input", Module: input.StateModule()})
	for i, d := range dilations {
		layer, e := autograd.NewConv1dLayer(hidden, hidden, 3, d, device, rng)
		if e != nil {
			return nil, e
		}
		m.Layers = append(m.Layers, layer)
		m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: fmt.Sprintf("layers.%d", i), Module: layer.StateModule()})
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "output", Module: output.StateModule()})
	return m, nil
}
func (m *tcn) forward(x *autograd.Tensor) *autograd.Tensor {
	h := autograd.Tanh(m.Input.Forward(x))
	for _, layer := range m.Layers {
		h = autograd.Tanh(autograd.Add(h, layer.Forward(h)))
	}
	y := m.Output.Forward(h)
	return autograd.Reshape(y, y.Shape[0], y.Shape[1])
}
func batch(rows []phrase, names []string, scale float64, device tensor.Device) (*autograd.Tensor, *autograd.Tensor, []bool, int, error) {
	length := 0
	for _, r := range rows {
		length = max(length, len(r.Y))
	}
	features := len(names)
	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	x := make([]float32, len(rows)*length*features)
	y := make([]float32, len(rows)*length)
	mask := make([]bool, len(rows)*length)
	for i, r := range rows {
		for j, f := range r.X {
			for k, v := range f {
				if v != 0 {
					x[(i*length+j)*features+index[k]] = float32(v)
				}
			}
		}
		for j, value := range r.Y {
			y[i*length+j] = float32(value / scale)
			mask[i*length+j] = true
		}
	}
	xt, e := autograd.New(x, []int{len(rows), length, features}, device, false)
	if e != nil {
		return nil, nil, nil, 0, e
	}
	yt, e := autograd.New(y, []int{len(rows), length}, device, false)
	if e != nil {
		xt.Close()
		return nil, nil, nil, 0, e
	}
	return xt, yt, mask, length, nil
}
func huber(diff *autograd.Tensor, mask []bool, device tensor.Device, count int, owned *[]*autograd.Tensor) *autograd.Tensor {
	a := autograd.Abs(diff)
	over := autograd.ReLU(autograd.SubScalar(a, 1))
	sq := autograd.Sub(autograd.Mul(a, a), autograd.Mul(over, over))
	weights := make([]float32, len(mask))
	for i, v := range mask {
		if v {
			weights[i] = 1
		}
	}
	w, e := autograd.New(weights, diff.Shape, device, false)
	if e != nil {
		panic(e)
	}
	*owned = append(*owned, w)
	return autograd.MulScalar(autograd.Sum(autograd.Mul(sq, w), 0, 1), .5/float32(max(1, count)))
}
func residualLoss(pred, target *autograd.Tensor, mask []bool, length int, deltaWeight, zeroPrior float64, device tensor.Device, owned *[]*autograd.Tensor) *autograd.Tensor {
	count := 0
	for _, v := range mask {
		if v {
			count++
		}
	}
	absolute := huber(autograd.Sub(pred, target), mask, device, count, owned)
	weights := make([]float32, len(mask))
	for i, v := range mask {
		if v {
			weights[i] = 1
		}
	}
	w, e := autograd.New(weights, pred.Shape, device, false)
	if e != nil {
		panic(e)
	}
	*owned = append(*owned, w)
	square := autograd.MulScalar(autograd.Sum(autograd.Mul(autograd.Mul(pred, pred), w), 0, 1), float32(zeroPrior)/float32(max(1, count)))
	total := autograd.Add(absolute, square)
	if length < 2 {
		return total
	}
	pd := autograd.Sub(autograd.Slice(pred, 1, 1, length), autograd.Slice(pred, 1, 0, length-1))
	td := autograd.Sub(autograd.Slice(target, 1, 1, length), autograd.Slice(target, 1, 0, length-1))
	b := pred.Shape[0]
	pairs := make([]bool, b*(length-1))
	n := 0
	for row := 0; row < b; row++ {
		for i := 1; i < length; i++ {
			v := mask[row*length+i] && mask[row*length+i-1]
			pairs[row*(length-1)+i-1] = v
			if v {
				n++
			}
		}
	}
	if n == 0 {
		return total
	}
	delta := huber(autograd.Sub(pd, td), pairs, device, n, owned)
	return autograd.Add(total, autograd.MulScalar(delta, float32(deltaWeight)))
}
func score(m *tcn, rows []phrase, names []string, scale float64) (float64, error) {
	sum := 0.0
	count := 0
	var failure error
	autograd.NoGrad(func() {
		for _, r := range rows {
			x, y, _, _, e := batch([]phrase{r}, names, scale, m.Device)
			if e != nil {
				failure = e
				return
			}
			z := m.forward(x)
			pred, e := z.ToHost()
			if e != nil {
				failure = e
				return
			}
			actual, e := y.ToHost()
			if e != nil {
				failure = e
				return
			}
			for i := range r.Y {
				sum += math.Abs(float64(pred[i]-actual[i]) * scale)
				count++
			}
			z.ReleaseGraph()
			x.Close()
			y.Close()
		}
	})
	if failure != nil {
		return 0, failure
	}
	if count == 0 {
		return 0, nil
	}
	return sum / float64(count), nil
}
func matrix(x []float32, rows, cols int) [][]float64 {
	out := make([][]float64, rows)
	for i := range out {
		out[i] = make([]float64, cols)
		for j := range out[i] {
			out[i][j] = float64(x[i*cols+j])
		}
	}
	return out
}
func vector(x []float32) []float64 {
	out := make([]float64, len(x))
	for i, v := range x {
		out[i] = float64(v)
	}
	return out
}
func exportResidual(m *tcn, names []string, hidden int, scale float64) map[string]any {
	state := m.Module.StateDict()
	layers := make([]any, len(dilations))
	for i, d := range dilations {
		w := state[fmt.Sprintf("layers.%d.weight", i)]
		cube := make([][][]float64, hidden)
		for o := range cube {
			cube[o] = matrix(w[o*hidden*3:(o+1)*hidden*3], hidden, 3)
		}
		layers[i] = map[string]any{"dilation": d, "weights": cube, "bias": vector(state[fmt.Sprintf("layers.%d.bias", i)])}
	}
	output := vector(state["output.weight"])
	for i := range output {
		output[i] *= scale
	}
	return map[string]any{"feature_names": names, "input_weights": matrix(state["input.weight"], hidden, len(names)), "input_bias": vector(state["input.bias"]), "layers": layers, "output_weight": output, "output_bias": float64(state["output.bias"][0]) * scale}
}
