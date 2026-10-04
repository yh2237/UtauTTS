package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
)

type tcn struct {
	Module        autograd.Module
	Input, Output *autograd.LinearLayer
	Layers        []*autograd.Conv1dLayer
	Hidden        int
	Dilations     []int
	Device        tensor.Device
}

func newTCN(features, hidden int, dilations []int, device tensor.Device, seed int64) (*tcn, error) {
	rng := rand.New(rand.NewSource(seed))
	input, e := autograd.NewLinearLayer(features, hidden, device, rng)
	if e != nil {
		return nil, e
	}
	output, e := autograd.NewLinearLayer(hidden, 1, device, rng)
	if e != nil {
		return nil, e
	}
	m := &tcn{Input: input, Output: output, Hidden: hidden, Dilations: dilations, Device: device}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "input", Module: input.StateModule()})
	for i, d := range dilations {
		l, e := autograd.NewConv1dLayer(hidden, hidden, 3, d, device, rng)
		if e != nil {
			return nil, e
		}
		m.Layers = append(m.Layers, l)
		m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: fmt.Sprintf("layers.%d", i), Module: l.StateModule()})
	}
	m.Module.Children = append(m.Module.Children, autograd.NamedModule{Name: "output", Module: output.StateModule()})
	return m, nil
}
func (m *tcn) forward(x *autograd.Tensor) *autograd.Tensor {
	h := autograd.Tanh(m.Input.Forward(x))
	for _, l := range m.Layers {
		h = autograd.Tanh(autograd.Add(h, l.Forward(h)))
	}
	y := m.Output.Forward(h)
	return autograd.Reshape(y, y.Shape[0], y.Shape[1])
}
func batchExamples(items []example, indices []int, features int, device tensor.Device) (*autograd.Tensor, *autograd.Tensor, []bool, int, error) {
	length := 0
	for _, id := range indices {
		length = max(length, items[id].Frames)
	}
	b := len(indices)
	x := make([]float32, b*length*features)
	target := make([]float32, b*length)
	mask := make([]bool, b*length)
	for row, id := range indices {
		it := items[id]
		for _, item := range it.Sparse {
			x[(row*length+item.Frame)*features+item.Column] = item.Value
		}
		copy(target[row*length:], it.Targets)
		copy(mask[row*length:], it.Mask)
	}
	xt, e := autograd.New(x, []int{b, length, features}, device, false)
	if e != nil {
		return nil, nil, nil, 0, e
	}
	yt, e := autograd.New(target, []int{b, length}, device, false)
	if e != nil {
		xt.Close()
		return nil, nil, nil, 0, e
	}
	return xt, yt, mask, length, nil
}
func lossHuber(diff *autograd.Tensor, mask []bool, device tensor.Device, denom float32, owned *[]*autograd.Tensor) *autograd.Tensor {
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
	return autograd.MulScalar(autograd.Sum(autograd.Mul(sq, w), 0, 1), .5/denom)
}
func lowerMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[(len(sorted)-1)/2]
}
func sequenceLoss(pred, target *autograd.Tensor, mask []bool, deltaWeight float64, low, high float32, device tensor.Device, owned *[]*autograd.Tensor) *autograd.Tensor {
	b, length := pred.Shape[0], pred.Shape[1]
	host, e := pred.ToHost()
	if e != nil {
		panic(e)
	}
	var centers []*autograd.Tensor
	for row := 0; row < b; row++ {
		type pair struct {
			v float32
			i int
		}
		var selected []pair
		for i := 0; i < length; i++ {
			if mask[row*length+i] {
				selected = append(selected, pair{host[row*length+i], i})
			}
		}
		if len(selected) == 0 {
			centers = append(centers, autograd.MulScalar(autograd.Slice(autograd.Slice(pred, 0, row, row+1), 1, 0, 1), 0))
			continue
		}
		sort.Slice(selected, func(i, j int) bool { return selected[i].v < selected[j].v })
		center := autograd.Slice(autograd.Slice(pred, 0, row, row+1), 1, selected[(len(selected)-1)/2].i, selected[(len(selected)-1)/2].i+1)
		centers = append(centers, center)
	}
	center := autograd.Concat(0, centers...)
	pc := autograd.Sub(pred, center)
	targetHost, e := target.ToHost()
	if e != nil {
		panic(e)
	}
	centeredData := make([]float32, b*length)
	for row := 0; row < b; row++ {
		var vals []float64
		for i := 0; i < length; i++ {
			if mask[row*length+i] {
				vals = append(vals, float64(targetHost[row*length+i]))
			}
		}
		med := float32(lowerMedian(vals))
		for i := 0; i < length; i++ {
			value := targetHost[row*length+i] - med
			centeredData[row*length+i] = max(low, min(high, value))
		}
	}
	centeredTarget, e := autograd.New(centeredData, []int{b, length}, device, false)
	if e != nil {
		panic(e)
	}
	*owned = append(*owned, centeredTarget)
	diff := autograd.Sub(pc, centeredTarget)
	count := 0
	for _, v := range mask {
		if v {
			count++
		}
	}
	absolute := lossHuber(diff, mask, device, float32(max(1, count)), owned)
	if length < 2 {
		return absolute
	}
	pd := autograd.Sub(autograd.Slice(pc, 1, 1, length), autograd.Slice(pc, 1, 0, length-1))
	td := autograd.Sub(autograd.Slice(centeredTarget, 1, 1, length), autograd.Slice(centeredTarget, 1, 0, length-1))
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
		return absolute
	}
	delta := lossHuber(autograd.Sub(pd, td), pairs, device, float32(n), owned)
	return autograd.Add(absolute, autograd.MulScalar(delta, float32(deltaWeight)))
}
func evaluate(m *tcn, items []example, features, batch int, c config) (raw, rendered float64, error error) {
	low, high := c.Low, c.High
	scale := math.Max(1, math.Max(math.Abs(low), math.Abs(high)))
	var rawSum, renderSum float64
	var count int
	autograd.NoGrad(func() {
		for offset := 0; offset < len(items); offset += batch {
			end := min(len(items), offset+batch)
			ids := make([]int, end-offset)
			for i := range ids {
				ids[i] = offset + i
			}
			x, y, mask, length, e := batchExamples(items, ids, features, m.Device)
			if e != nil {
				error = e
				return
			}
			p := m.forward(x)
			pred, e := p.ToHost()
			if e != nil {
				error = e
				return
			}
			target, e := y.ToHost()
			if e != nil {
				error = e
				return
			}
			for row := range ids {
				n := items[ids[row]].Frames
				pv := make([]float64, n)
				tv := make([]float64, n)
				valid := mask[row*length : row*length+n]
				var centered []float64
				for i := 0; i < n; i++ {
					pv[i] = float64(pred[row*length+i]) * scale
					tv[i] = float64(target[row*length+i]) * scale
					if valid[i] {
						centered = append(centered, pv[i])
					}
				}
				med := lowerMedian(centered)
				for i := 0; i < n; i++ {
					if valid[i] {
						v := math.Max(low, math.Min(high, pv[i]-med))
						rawSum += math.Abs(v - tv[i])
						count++
					}
				}
				a := renderContour(pv, valid, c.Frame, c.RenderStrength, c.RenderSmoothing, c.RenderP99, c.RenderMax, low, high)
				b := renderContour(tv, valid, c.Frame, c.RenderStrength, c.RenderSmoothing, c.RenderP99, c.RenderMax, low, high)
				for i := 0; i < n; i++ {
					if valid[i] {
						renderSum += math.Abs(a[i] - b[i])
					}
				}
			}
			p.ReleaseGraph()
			x.Close()
			y.Close()
		}
	})
	if count > 0 {
		raw = rawSum / float64(count)
		rendered = renderSum / float64(count)
	}
	return
}
func renderContour(x []float64, mask []bool, step, strength, smooth, p99, maximum, low, high float64) []float64 {
	out := append([]float64(nil), x...)
	var selected []float64
	for i, v := range out {
		if mask[i] {
			selected = append(selected, v)
		}
	}
	if len(selected) == 0 {
		return make([]float64, len(x))
	}
	center := median(selected)
	for i := range out {
		if mask[i] {
			out[i] -= center
		} else {
			out[i] = 0
		}
	}
	sigma := smooth / step
	if sigma > 0 {
		radius := max(1, int(math.Ceil(3*sigma)))
		kernel := make([]float64, 2*radius+1)
		sum := 0.0
		for i := range kernel {
			z := float64(i-radius) / sigma
			kernel[i] = math.Exp(-.5 * z * z)
			sum += kernel[i]
		}
		for i := range kernel {
			kernel[i] /= sum
		}
		for a := 0; a < len(out); {
			if !mask[a] {
				a++
				continue
			}
			b := a + 1
			for b < len(out) && mask[b] {
				b++
			}
			old := append([]float64(nil), out[a:b]...)
			for i := a; i < b; i++ {
				v := 0.0
				for j, k := range kernel {
					ix := max(0, min(len(old)-1, i-a+j-radius))
					v += old[ix] * k
				}
				out[i] = v
			}
			a = b
		}
	}
	var magnitudes []float64
	for i := range out {
		out[i] *= strength
		if mask[i] {
			magnitudes = append(magnitudes, math.Abs(out[i]))
		}
	}
	sort.Float64s(magnitudes)
	observed := magnitudes[max(0, int(math.Ceil(.99*float64(len(magnitudes))))-1)]
	ratio := 1.0
	if observed > p99 {
		ratio = p99 / observed
	}
	for i := range out {
		out[i] = math.Max(low, math.Min(high, math.Max(-maximum, math.Min(maximum, out[i]*ratio))))
	}
	return out
}
