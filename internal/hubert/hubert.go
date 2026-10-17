package hubert

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
)

type Model struct {
	file        *autograd.SafeTensorFile
	weights     map[string][]float32
	convWeights [7][]float32
	posWeights  []float32
	packed      map[string]*tensor.PackedB
	// muはCloseとFeaturesを、cacheは読み込んだ重みと前処理した重みを守る。
	mu    sync.RWMutex
	cache sync.Mutex
	// cudaMu protects cudaState; the state is defined in the Windows-only implementation.
	cudaMu    sync.Mutex
	cudaState any
}

func Load(path string) (*Model, error) {
	f, e := autograd.OpenSafeTensorFile(path)
	if e != nil {
		return nil, e
	}
	return &Model{file: f, weights: map[string][]float32{}, packed: map[string]*tensor.PackedB{}}, nil
}
func (m *Model) Close() error { m.mu.Lock(); defer m.mu.Unlock(); return m.file.Close() }
func (m *Model) get(name string) ([]float32, error) {
	m.cache.Lock()
	defer m.cache.Unlock()
	if x, ok := m.weights[name]; ok {
		return x, nil
	}
	x, _, e := m.file.ReadF32(name)
	if e != nil {
		return nil, e
	}
	m.weights[name] = x
	return x, nil
}
func (m *Model) linear(x []float32, rows, in, out int, prefix string) ([]float32, error) {
	w, e := m.get(prefix + ".weight")
	if e != nil {
		return nil, e
	}
	if len(w) != in*out {
		return nil, fmt.Errorf("hubert: %s weight shape", prefix)
	}
	y := make([]float32, rows*out)
	m.cache.Lock()
	packed := m.packed[prefix]
	if packed == nil {
		packed = tensor.PackB(w, out, in, true)
		m.packed[prefix] = packed
	}
	m.cache.Unlock()
	tensor.SGEMMPacked(y, x, packed, rows, 8)
	b, e := m.get(prefix + ".bias")
	if e != nil {
		return nil, e
	}
	for i := 0; i < rows; i++ {
		for j := 0; j < out; j++ {
			y[i*out+j] += b[j]
		}
	}
	return y, nil
}
func norm(x, w, b []float32, rows, cols int, eps float32) {
	for r := 0; r < rows; r++ {
		p := x[r*cols : (r+1)*cols]
		var mean, v float32
		for _, z := range p {
			mean += z
		}
		mean /= float32(cols)
		for _, z := range p {
			d := z - mean
			v += d * d
		}
		v /= float32(cols)
		scale := float32(1 / math.Sqrt(float64(v+eps)))
		for j := range p {
			p[j] = (p[j]-mean)*scale*w[j] + b[j]
		}
	}
}

const geluTableSteps = 8192

var geluTable = func() [16*geluTableSteps + 1]float32 {
	var table [16*geluTableSteps + 1]float32
	for i := range table {
		x := float64(i)/geluTableSteps - 8
		table[i] = float32(0.5 * x * (1 + math.Erf(x/math.Sqrt2)))
	}
	return table
}()

func gelu(x float32) float32 {
	if x <= -8 {
		return 0
	}
	if x >= 8 {
		return x
	}
	position := (float64(x) + 8) * geluTableSteps
	index := int(position)
	fraction := float32(position - float64(index))
	return geluTable[index] + fraction*(geluTable[index+1]-geluTable[index])
}

func applyGELU(values []float32) {
	workers := min(runtime.GOMAXPROCS(0), len(values)/8192)
	if workers < 2 {
		for index := range values {
			values[index] = gelu(values[index])
		}
		return
	}
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start, end := worker*len(values)/workers, (worker+1)*len(values)/workers
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := start; index < end; index++ {
				values[index] = gelu(values[index])
			}
		}()
	}
	wait.Wait()
}

// FeaturesAcceleratedは、GPUが使える環境ではGPUで、そうでなければCPUで特徴量を求める。
func (m *Model) FeaturesAccelerated(wave []float32, layer int) ([][]float32, error) {
	return m.featuresAccelerated(wave, layer)
}

// Features returns hidden_states[layer] as contiguous [frames][768] rows.
// Featuresは複数のgoroutineから同時に呼べる。
func (m *Model) Features(wave []float32, layer int) ([][]float32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if layer < 0 || layer > 12 {
		return nil, fmt.Errorf("hubert: layer must be 0..12")
	}
	if len(wave) < 400 {
		return nil, fmt.Errorf("hubert: waveform too short")
	}
	x := append([]float32(nil), wave...)
	frames, ch := len(x), 1
	kernels := [7]int{10, 3, 3, 3, 3, 2, 2}
	strides := [7]int{5, 2, 2, 2, 2, 2, 2}
	for i := 0; i < 7; i++ {
		base := fmt.Sprintf("feature_extractor.conv_layers.%d", i)
		w, e := m.get(base + ".conv.weight")
		if e != nil {
			return nil, e
		}
		m.cache.Lock()
		if m.convWeights[i] == nil {
			m.convWeights[i] = flattenConv(w, ch, 512, kernels[i])
		}
		convWeights := m.convWeights[i]
		m.cache.Unlock()
		x, frames = conv(x, frames, ch, convWeights, 512, kernels[i], strides[i])
		ch = 512
		if i == 0 {
			nw, e := m.get(base + ".layer_norm.weight")
			if e != nil {
				return nil, e
			}
			nb, e := m.get(base + ".layer_norm.bias")
			if e != nil {
				return nil, e
			}
			groupNorm(x, nw, nb, frames, ch)
		}
		applyGELU(x)
	}
	nw, e := m.get("feature_projection.layer_norm.weight")
	if e != nil {
		return nil, e
	}
	nb, e := m.get("feature_projection.layer_norm.bias")
	if e != nil {
		return nil, e
	}
	norm(x, nw, nb, frames, 512, 1e-5)
	x, e = m.linear(x, frames, 512, 768, "feature_projection.projection")
	if e != nil {
		return nil, e
	}
	pos, e := m.posConv(x, frames)
	if e != nil {
		return nil, e
	}
	for i := range x {
		x[i] += pos[i]
	}
	nw, e = m.get("encoder.layer_norm.weight")
	if e != nil {
		return nil, e
	}
	nb, e = m.get("encoder.layer_norm.bias")
	if e != nil {
		return nil, e
	}
	norm(x, nw, nb, frames, 768, 1e-5)
	for i := 0; i < layer; i++ {
		x, e = m.encoderLayer(x, frames, i)
		if e != nil {
			return nil, e
		}
	}
	rows := make([][]float32, frames)
	for i := range rows {
		rows[i] = x[i*768 : (i+1)*768]
	}
	return rows, nil
}

func groupNorm(x, w, b []float32, frames, ch int) {
	mean := make([]float32, ch)
	variance := make([]float32, ch)
	for frame := 0; frame < frames; frame++ {
		for channel, value := range x[frame*ch : (frame+1)*ch] {
			mean[channel] += value
		}
	}
	for channel := range mean {
		mean[channel] /= float32(frames)
	}
	for frame := 0; frame < frames; frame++ {
		for channel, value := range x[frame*ch : (frame+1)*ch] {
			difference := value - mean[channel]
			variance[channel] += difference * difference
		}
	}
	for channel := range variance {
		variance[channel] = float32(1 / math.Sqrt(float64(variance[channel]/float32(frames)+1e-5)))
	}
	workers := min(runtime.GOMAXPROCS(0), frames)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start, end := worker*frames/workers, (worker+1)*frames/workers
		wait.Add(1)
		go func() {
			defer wait.Done()
			for frame := start; frame < end; frame++ {
				row := x[frame*ch : (frame+1)*ch]
				for channel, value := range row {
					row[channel] = (value-mean[channel])*variance[channel]*w[channel] + b[channel]
				}
			}
		}()
	}
	wait.Wait()
}
func flattenConv(w []float32, in, out, kernel int) []float32 {
	depth := in * kernel
	flat := make([]float32, len(w))
	for output := 0; output < out; output++ {
		for channel := 0; channel < in; channel++ {
			for tap := 0; tap < kernel; tap++ {
				flat[output*depth+tap*in+channel] = w[(output*in+channel)*kernel+tap]
			}
		}
	}
	return flat
}

func conv(x []float32, frames, in int, flatW []float32, out, kernel, stride int) ([]float32, int) {
	n := (frames-kernel)/stride + 1
	y := make([]float32, n*out)
	depth := in * kernel
	const block = 512
	column := make([]float32, min(block, n)*depth)
	for start := 0; start < n; start += block {
		count := min(block, n-start)
		a := column[:count*depth]
		for r := 0; r < count; r++ {
			for k := 0; k < kernel; k++ {
				copy(a[r*depth+k*in:r*depth+(k+1)*in], x[((start+r)*stride+k)*in:((start+r)*stride+k+1)*in])
			}
		}
		tensor.SGEMMOp(y[start*out:(start+count)*out], a, flatW, count, out, depth, false, true)
	}
	return y, n
}

func (m *Model) posConv(x []float32, n int) ([]float32, error) {
	const width = 768
	const kernel = 128
	const groups = 16
	v, e := m.get("encoder.pos_conv_embed.conv.weight_v")
	if e != nil {
		v, e = m.get("encoder.pos_conv_embed.conv.parametrizations.weight.original1")
		if e != nil {
			return nil, e
		}
	}
	g, e := m.get("encoder.pos_conv_embed.conv.weight_g")
	if e != nil {
		g, e = m.get("encoder.pos_conv_embed.conv.parametrizations.weight.original0")
		if e != nil {
			return nil, e
		}
	}
	bias, e := m.get("encoder.pos_conv_embed.conv.bias")
	if e != nil {
		return nil, e
	}
	if len(v) != width*48*kernel {
		return nil, fmt.Errorf("hubert: positional convolution shape")
	}
	posWeights, e := m.positionalWeights(v, g)
	if e != nil {
		return nil, e
	}
	y := make([]float32, n*width)
	const depth = 48 * kernel
	const block = 64
	workers := min(runtime.GOMAXPROCS(0), groups)
	jobs := make(chan int, groups)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			column := make([]float32, min(block, n)*depth)
			z := make([]float32, min(block, n)*48)
			for group := range jobs {
				for start := 0; start < n; start += block {
					count := min(block, n-start)
					a := column[:count*depth]
					clear(a)
					for t := 0; t < count; t++ {
						for ic := 0; ic < 48; ic++ {
							for k := 0; k < kernel; k++ {
								src := start + t + k - 64
								if src >= 0 && src < n {
									a[t*depth+ic*kernel+k] = x[src*width+group*48+ic]
								}
							}
						}
					}
					blockOutput := z[:count*48]
					tensor.SGEMMOpWorkers(blockOutput, a, posWeights[group*48*depth:(group+1)*48*depth], count, 48, depth, false, true, 1)
					for t := 0; t < count; t++ {
						for c := 0; c < 48; c++ {
							channel := group*48 + c
							y[(start+t)*width+channel] = gelu(blockOutput[t*48+c] + bias[channel])
						}
					}
				}
			}
		}()
	}
	for group := 0; group < groups; group++ {
		jobs <- group
	}
	close(jobs)
	wait.Wait()
	return y, nil
}

func (m *Model) encoderLayer(x []float32, n, index int) ([]float32, error) {
	const width = 768
	const heads = 12
	const hd = 64
	base := fmt.Sprintf("encoder.layers.%d", index)
	q, e := m.linear(x, n, width, width, base+".attention.q_proj")
	if e != nil {
		return nil, e
	}
	k, e := m.linear(x, n, width, width, base+".attention.k_proj")
	if e != nil {
		return nil, e
	}
	v, e := m.linear(x, n, width, width, base+".attention.v_proj")
	if e != nil {
		return nil, e
	}
	context := make([]float32, len(x))
	queryHead := make([]float32, n*hd)
	keyHead := make([]float32, n*hd)
	valueHead := make([]float32, n*hd)
	scores := make([]float32, n*n)
	headContext := make([]float32, n*hd)
	for h := 0; h < heads; h++ {
		for t := 0; t < n; t++ {
			copy(queryHead[t*hd:(t+1)*hd], q[t*width+h*hd:t*width+(h+1)*hd])
			copy(keyHead[t*hd:(t+1)*hd], k[t*width+h*hd:t*width+(h+1)*hd])
			copy(valueHead[t*hd:(t+1)*hd], v[t*width+h*hd:t*width+(h+1)*hd])
		}
		tensor.SGEMMOp(scores, queryHead, keyHead, n, n, hd, false, true)
		for t := 0; t < n; t++ {
			score := scores[t*n : (t+1)*n]
			maxScore := float32(math.Inf(-1))
			for s := 0; s < n; s++ {
				score[s] *= 0.125
				if score[s] > maxScore {
					maxScore = score[s]
				}
			}
			var sum float32
			for s := range score {
				score[s] = float32(math.Exp(float64(score[s] - maxScore)))
				sum += score[s]
			}
			for s := range score {
				score[s] /= sum
			}
		}
		tensor.SGEMM(headContext, scores, valueHead, n, hd, n)
		for t := 0; t < n; t++ {
			copy(context[t*width+h*hd:t*width+(h+1)*hd], headContext[t*hd:(t+1)*hd])
		}
	}
	attn, e := m.linear(context, n, width, width, base+".attention.out_proj")
	if e != nil {
		return nil, e
	}
	for i := range x {
		x[i] += attn[i]
	}
	nw, e := m.get(base + ".layer_norm.weight")
	if e != nil {
		return nil, e
	}
	nb, e := m.get(base + ".layer_norm.bias")
	if e != nil {
		return nil, e
	}
	norm(x, nw, nb, n, width, 1e-5)
	f, e := m.linear(x, n, width, 3072, base+".feed_forward.intermediate_dense")
	if e != nil {
		return nil, e
	}
	applyGELU(f)
	f, e = m.linear(f, n, 3072, width, base+".feed_forward.output_dense")
	if e != nil {
		return nil, e
	}
	for i := range x {
		x[i] += f[i]
	}
	nw, e = m.get(base + ".final_layer_norm.weight")
	if e != nil {
		return nil, e
	}
	nb, e = m.get(base + ".final_layer_norm.bias")
	if e != nil {
		return nil, e
	}
	norm(x, nw, nb, n, width, 1e-5)
	return x, nil
}

func (m *Model) positionalWeights(v, g []float32) ([]float32, error) {
	const width = 768
	const kernel = 128
	m.cache.Lock()
	defer m.cache.Unlock()
	if m.posWeights == nil {
		w := make([]float32, len(v))
		if len(g) == kernel {
			for k := 0; k < kernel; k++ {
				var sq float64
				for i := k; i < len(v); i += kernel {
					sq += float64(v[i]) * float64(v[i])
				}
				scale := float32(float64(g[k]) / math.Sqrt(sq))
				for i := k; i < len(v); i += kernel {
					w[i] = v[i] * scale
				}
			}
		} else if len(g) == width {
			for c := 0; c < width; c++ {
				base := c * 48 * kernel
				var sq float64
				for _, z := range v[base : base+48*kernel] {
					sq += float64(z) * float64(z)
				}
				scale := float32(float64(g[c]) / math.Sqrt(sq))
				for j := 0; j < 48*kernel; j++ {
					w[base+j] = v[base+j] * scale
				}
			}
		} else {
			return nil, fmt.Errorf("hubert: positional weight norm shape")
		}
		m.posWeights = w
	}
	return m.posWeights, nil
}
