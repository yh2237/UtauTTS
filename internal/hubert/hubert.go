package hubert

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
)

type Model struct {
	file        *autograd.SafeTensorFile
	weights     map[string][]float32
	convWeights [7][]float32
	posWeights  []float32
	config      Config
	packed      map[string]*tensor.PackedB
	// muはCloseとFeaturesを、cacheは読み込んだ重みと前処理した重みを守る。
	mu    sync.RWMutex
	cache sync.Mutex
	// stateMu protects deviceStates; the states are defined in the GPU implementation.
	stateMu      sync.Mutex
	deviceStates map[tensor.Device]*gpuState
}

func Load(path string) (*Model, error) {
	f, e := autograd.OpenSafeTensorFile(path)
	if e != nil {
		return nil, e
	}
	return &Model{file: f, weights: map[string][]float32{}, packed: map[string]*tensor.PackedB{}, config: loadConfig(path)}, nil
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

// lastDeviceは、直近の特徴量計算に使ったデバイス名。
var lastDevice atomic.Value

// LastDeviceは、直近の特徴量計算に使ったデバイスを返す。
func LastDevice() string {
	if value, ok := lastDevice.Load().(string); ok {
		return value
	}
	return ""
}

func setLastDevice(name string) {
	lastDevice.Store(name)
}

// gpuDeviceLabelは、画面に出すためのデバイス名を返す。
func gpuDeviceLabel(device tensor.Device) string {
	switch device {
	case tensor.CUDA:
		return "CUDA"
	case tensor.DirectML:
		return "DirectML"
	case tensor.Vulkan:
		return "Vulkan"
	}
	return string(device)
}

// FeaturesAcceleratedは、GPUが使える環境ではGPUで、そうでなければCPUで特徴量を求める。
func (m *Model) FeaturesAccelerated(wave []float32, layer int) ([][]float32, error) {
	return m.featuresAccelerated(wave, layer)
}

// Features returns hidden_states[layer] as contiguous [frames][HiddenSize] rows.
// Featuresは複数のgoroutineから同時に呼べる。
func (m *Model) Features(wave []float32, layer int) ([][]float32, error) {
	setLastDevice("CPU")
	m.mu.RLock()
	defer m.mu.RUnlock()
	config := m.config
	if layer < 0 || layer > config.Layers {
		return nil, fmt.Errorf("hubert: layer must be 0..%d", config.Layers)
	}
	if len(wave) < 400 {
		return nil, fmt.Errorf("hubert: waveform too short")
	}
	x := append([]float32(nil), wave...)
	frames, ch := len(x), 1
	for i := range config.ConvKernel {
		base := fmt.Sprintf("feature_extractor.conv_layers.%d", i)
		w, e := m.get(base + ".conv.weight")
		if e != nil {
			return nil, e
		}
		m.cache.Lock()
		if m.convWeights[i] == nil {
			m.convWeights[i] = flattenConv(w, ch, config.ConvDim[i], config.ConvKernel[i])
		}
		convWeights := m.convWeights[i]
		m.cache.Unlock()
		x, frames = conv(x, frames, ch, convWeights, config.ConvDim[i], config.ConvKernel[i], config.ConvStride[i])
		ch = config.ConvDim[i]
		if config.ConvBias {
			bias, e := m.get(base + ".conv.bias")
			if e != nil {
				return nil, e
			}
			for frame := 0; frame < frames; frame++ {
				for channel := 0; channel < ch; channel++ {
					x[frame*ch+channel] += bias[channel]
				}
			}
		}
		if config.FeatNorm == "layer" {
			nw, e := m.get(base + ".layer_norm.weight")
			if e != nil {
				return nil, e
			}
			nb, e := m.get(base + ".layer_norm.bias")
			if e != nil {
				return nil, e
			}
			norm(x, nw, nb, frames, ch, 1e-5)
		} else if i == 0 {
			// グループ正規化は、重みがある先頭の層だけに掛ける（HuBERTとWavLM）。
			nw, e := m.get(base + ".layer_norm.weight")
			if e != nil {
				return nil, e
			}
			nb, e := m.get(base + ".layer_norm.bias")
			if e != nil {
				return nil, e
			}
			if config.GroupNormGroups > 0 {
				groupNormHF(x, nw, nb, frames, ch, config.GroupNormGroups)
			} else {
				groupNorm(x, nw, nb, frames, ch)
			}
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
	norm(x, nw, nb, frames, ch, 1e-5)
	x, e = m.linear(x, frames, ch, config.HiddenSize, "feature_projection.projection")
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
	var relativeBias []float32
	if config.RelativeBias {
		relativeBias, e = m.relativeBias(frames)
		if e != nil {
			return nil, e
		}
	}
	if config.Stable {
		// 事前正規化のエンコーダ（XLS-R、WavLM）は、最後にまとめて正規化する。
		for i := 0; i < layer; i++ {
			x, e = m.encoderLayer(x, frames, i, relativeBias)
			if e != nil {
				return nil, e
			}
		}
		if layer == config.Layers {
			nw, e = m.get("encoder.layer_norm.weight")
			if e != nil {
				return nil, e
			}
			nb, e = m.get("encoder.layer_norm.bias")
			if e != nil {
				return nil, e
			}
			norm(x, nw, nb, frames, config.HiddenSize, 1e-5)
		}
	} else {
		nw, e = m.get("encoder.layer_norm.weight")
		if e != nil {
			return nil, e
		}
		nb, e = m.get("encoder.layer_norm.bias")
		if e != nil {
			return nil, e
		}
		norm(x, nw, nb, frames, config.HiddenSize, 1e-5)
		for i := 0; i < layer; i++ {
			x, e = m.encoderLayer(x, frames, i, relativeBias)
			if e != nil {
				return nil, e
			}
		}
	}
	rows := make([][]float32, frames)
	for i := range rows {
		rows[i] = x[i*config.HiddenSize : (i+1)*config.HiddenSize]
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

// groupNormHFは、チャンネルをいくつかのグループに分けて正規化する（transformersのGroupNorm）。
func groupNormHF(x, w, b []float32, frames, ch, groups int) {
	per := ch / groups
	mean := make([]float32, groups)
	scale := make([]float32, groups)
	for g := 0; g < groups; g++ {
		var sum float64
		for frame := 0; frame < frames; frame++ {
			for c := 0; c < per; c++ {
				sum += float64(x[frame*ch+g*per+c])
			}
		}
		mean[g] = float32(sum / float64(frames*per))
		var variance float64
		for frame := 0; frame < frames; frame++ {
			for c := 0; c < per; c++ {
				difference := float64(x[frame*ch+g*per+c]) - float64(mean[g])
				variance += difference * difference
			}
		}
		scale[g] = float32(1 / math.Sqrt(variance/float64(frames*per)+1e-5))
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
					group := channel / per
					row[channel] = (value-mean[group])*scale[group]*w[channel] + b[channel]
				}
			}
		}()
	}
	wait.Wait()
}

// relativeBucketは、WavLMの相対位置をバケットに割り当てる。
func relativeBucket(relative int) int {
	const numBuckets = 320
	const maxDistance = 800
	half := numBuckets / 2
	bucket := 0
	if relative > 0 {
		bucket += half
	} else {
		relative = -relative
	}
	maxExact := half / 2
	if relative < maxExact {
		return bucket + relative
	}
	value := maxExact + int(math.Log(float64(relative)/float64(maxExact))/math.Log(float64(maxDistance)/float64(maxExact))*float64(half-maxExact))
	if value > half-1 {
		value = half - 1
	}
	return bucket + value
}

// relativeBiasは、WavLMの相対位置のバイアスをフレーム数分作る（フレーム×フレーム×ヘッド）。
func (m *Model) relativeBias(frames int) ([]float32, error) {
	config := m.config
	embed, err := m.get("encoder.layers.0.attention.rel_attn_embed.weight")
	if err != nil {
		return nil, err
	}
	if len(embed) != 320*config.Heads {
		return nil, fmt.Errorf("hubert: relative position embedding shape")
	}
	bias := make([]float32, frames*frames*config.Heads)
	for i := 0; i < frames; i++ {
		for j := 0; j < frames; j++ {
			bucket := relativeBucket(j - i)
			for h := 0; h < config.Heads; h++ {
				bias[(i*frames+j)*config.Heads+h] = embed[bucket*config.Heads+h]
			}
		}
	}
	return bias, nil
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
	config := m.config
	width := config.HiddenSize
	kernel := config.PosKernel
	groups := config.PosGroups
	channels := width / groups
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
	if len(v) != width*channels*kernel {
		return nil, fmt.Errorf("hubert: positional convolution shape")
	}
	posWeights, e := m.positionalWeights(v, g)
	if e != nil {
		return nil, e
	}
	y := make([]float32, n*width)
	depth := channels * kernel
	const block = 64
	padding := kernel / 2
	workers := min(runtime.GOMAXPROCS(0), groups)
	jobs := make(chan int, groups)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			column := make([]float32, min(block, n)*depth)
			z := make([]float32, min(block, n)*channels)
			for group := range jobs {
				for start := 0; start < n; start += block {
					count := min(block, n-start)
					a := column[:count*depth]
					clear(a)
					for t := 0; t < count; t++ {
						for ic := 0; ic < channels; ic++ {
							for k := 0; k < kernel; k++ {
								src := start + t + k - padding
								if src >= 0 && src < n {
									a[t*depth+ic*kernel+k] = x[src*width+group*channels+ic]
								}
							}
						}
					}
					blockOutput := z[:count*channels]
					tensor.SGEMMOpWorkers(blockOutput, a, posWeights[group*channels*depth:(group+1)*channels*depth], count, channels, depth, false, true, 1)
					for t := 0; t < count; t++ {
						for c := 0; c < channels; c++ {
							channel := group*channels + c
							y[(start+t)*width+channel] = gelu(blockOutput[t*channels+c] + bias[channel])
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

// attentionは自己注意を計算し、出力の射影まで返す。biasを渡すと相対位置のバイアスを足す（WavLM）。
func (m *Model) attention(x []float32, n, index int, bias []float32) ([]float32, error) {
	config := m.config
	width, heads := config.HiddenSize, config.Heads
	headDim := width / heads
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
	scale := float32(1 / math.Sqrt(float64(headDim)))
	context := make([]float32, len(x))
	queryHead := make([]float32, n*headDim)
	keyHead := make([]float32, n*headDim)
	valueHead := make([]float32, n*headDim)
	scores := make([]float32, n*n)
	headContext := make([]float32, n*headDim)
	var gateWeight, gateBias, gateConst []float32
	if bias != nil {
		if gateWeight, e = m.get(base + ".attention.gru_rel_pos_linear.weight"); e != nil {
			return nil, e
		}
		if gateBias, e = m.get(base + ".attention.gru_rel_pos_linear.bias"); e != nil {
			return nil, e
		}
		if gateConst, e = m.get(base + ".attention.gru_rel_pos_const"); e != nil {
			return nil, e
		}
	}
	for h := 0; h < heads; h++ {
		for t := 0; t < n; t++ {
			copy(queryHead[t*headDim:(t+1)*headDim], q[t*width+h*headDim:t*width+(h+1)*headDim])
			copy(keyHead[t*headDim:(t+1)*headDim], k[t*width+h*headDim:t*width+(h+1)*headDim])
			copy(valueHead[t*headDim:(t+1)*headDim], v[t*width+h*headDim:t*width+(h+1)*headDim])
		}
		tensor.SGEMMOp(scores, queryHead, keyHead, n, n, headDim, false, true)
		if bias != nil {
			for i := range scores {
				scores[i] *= scale
			}
			for t := 0; t < n; t++ {
				var first, second float32
				for o := 0; o < 8; o++ {
					sum := gateBias[o]
					for d := 0; d < headDim; d++ {
						sum += x[t*width+h*headDim+d] * gateWeight[o*headDim+d]
					}
					if o < 4 {
						first += sum
					} else {
						second += sum
					}
				}
				gateA := float32(1 / (1 + math.Exp(-float64(first))))
				gateB := float32(1 / (1 + math.Exp(-float64(second))))
				gate := gateA*(gateB*gateConst[h]-1) + 2
				for s := 0; s < n; s++ {
					scores[t*n+s] += gate * bias[(t*n+s)*heads+h]
				}
			}
		}
		for t := 0; t < n; t++ {
			score := scores[t*n : (t+1)*n]
			maxScore := float32(math.Inf(-1))
			for s := 0; s < n; s++ {
				if bias == nil {
					score[s] *= scale
				}
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
		tensor.SGEMM(headContext, scores, valueHead, n, headDim, n)
		for t := 0; t < n; t++ {
			copy(context[t*width+h*headDim:t*width+(h+1)*headDim], headContext[t*headDim:(t+1)*headDim])
		}
	}
	return m.linear(context, n, width, width, base+".attention.out_proj")
}

// encoderLayerは1層分を計算する。Stableなら事前正規化、そうでなければ事後正規化。
func (m *Model) encoderLayer(x []float32, n, index int, bias []float32) ([]float32, error) {
	config := m.config
	width := config.HiddenSize
	base := fmt.Sprintf("encoder.layers.%d", index)
	if config.Stable {
		residual := append([]float32(nil), x...)
		nw, e := m.get(base + ".layer_norm.weight")
		if e != nil {
			return nil, e
		}
		nb, e := m.get(base + ".layer_norm.bias")
		if e != nil {
			return nil, e
		}
		norm(x, nw, nb, n, width, 1e-5)
		attn, e := m.attention(x, n, index, bias)
		if e != nil {
			return nil, e
		}
		for i := range residual {
			residual[i] += attn[i]
		}
		x = residual
		residual = append([]float32(nil), x...)
		nw, e = m.get(base + ".final_layer_norm.weight")
		if e != nil {
			return nil, e
		}
		nb, e = m.get(base + ".final_layer_norm.bias")
		if e != nil {
			return nil, e
		}
		norm(x, nw, nb, n, width, 1e-5)
		f, e := m.linear(x, n, width, config.Intermediate, base+".feed_forward.intermediate_dense")
		if e != nil {
			return nil, e
		}
		applyGELU(f)
		f, e = m.linear(f, n, config.Intermediate, width, base+".feed_forward.output_dense")
		if e != nil {
			return nil, e
		}
		for i := range residual {
			residual[i] += f[i]
		}
		return residual, nil
	}
	attn, e := m.attention(x, n, index, bias)
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
	f, e := m.linear(x, n, width, config.Intermediate, base+".feed_forward.intermediate_dense")
	if e != nil {
		return nil, e
	}
	applyGELU(f)
	f, e = m.linear(f, n, config.Intermediate, width, base+".feed_forward.output_dense")
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
	config := m.config
	width := config.HiddenSize
	kernel := config.PosKernel
	channels := width / config.PosGroups
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
				base := c * channels * kernel
				var sq float64
				for _, z := range v[base : base+channels*kernel] {
					sq += float64(z) * float64(z)
				}
				scale := float32(float64(g[c]) / math.Sqrt(sq))
				for j := 0; j < channels*kernel; j++ {
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
