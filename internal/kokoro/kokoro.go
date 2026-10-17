package kokoro

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/yh2237/gograd/tensor"
	"github.com/yh2237/gograd/torchpth"
	"github.com/yh2237/gograd/torchrng"
)

type Config struct {
	Vocab      map[string]int `json:"vocab"`
	HiddenDim  int            `json:"hidden_dim"`
	StyleDim   int            `json:"style_dim"`
	MaxDur     int            `json:"max_dur"`
	NLayer     int            `json:"n_layer"`
	TextKernel int            `json:"text_encoder_kernel_size"`
	PLBERT     struct {
		MaxPosition int `json:"max_position_embeddings"`
	} `json:"plbert"`
}
type Model struct {
	config       Config
	weights      map[string]torchpth.Tensor
	convLayouts  map[string][]float32
	convTLayouts map[string][]float32
	convPacked   map[string][]*tensor.PackedB
	packedLinear sync.Map
	layoutMu     sync.Mutex
	mu           sync.Mutex
}

type Voice struct{ data []float32 }
type Result struct {
	Audio     []float32
	Durations []int
	F0        []float32
}

func Load(pthPath, configPath string) (*Model, error) {
	b, e := os.ReadFile(configPath)
	if e != nil {
		return nil, e
	}
	var cfg Config
	if e = json.Unmarshal(b, &cfg); e != nil {
		return nil, e
	}
	if cfg.HiddenDim != 512 || cfg.StyleDim != 128 || cfg.MaxDur != 50 {
		return nil, fmt.Errorf("kokoro: unsupported config dimensions")
	}
	w, e := torchpth.Read(pthPath)
	if e != nil {
		return nil, e
	}
	clean := make(map[string]torchpth.Tensor, len(w))
	for name, value := range w {
		name = strings.Replace(name, ".module.", ".", 1)
		clean[name] = value
	}
	model := &Model{config: cfg, weights: clean, convLayouts: make(map[string][]float32), convTLayouts: make(map[string][]float32), convPacked: make(map[string][]*tensor.PackedB)}
	for name := range clean {
		if strings.HasSuffix(name, ".weight_v") {
			model.folded(strings.TrimSuffix(name, ".weight_v"))
		}
	}
	for name, weight := range clean {
		if !strings.HasSuffix(name, ".weight") {
			continue
		}
		prefix := strings.TrimSuffix(name, ".weight")
		if len(weight.Shape) == 3 {
			channels, otherChannels, kernel := weight.Shape[0], weight.Shape[1], weight.Shape[2]
			flat := make([]float32, len(weight.Data))
			if strings.Contains(prefix, "decoder.generator.ups.") || strings.HasSuffix(prefix, ".pool") {
				for channel := 0; channel < channels; channel++ {
					for output := 0; output < otherChannels; output++ {
						for tap := 0; tap < kernel; tap++ {
							flat[channel*otherChannels*kernel+tap*otherChannels+output] = weight.Data[(channel*otherChannels+output)*kernel+tap]
						}
					}
				}
				model.convTLayouts[prefix] = flat
			} else {
				for output := 0; output < channels; output++ {
					for channel := 0; channel < otherChannels; channel++ {
						for tap := 0; tap < kernel; tap++ {
							flat[output*otherChannels*kernel+tap*otherChannels+channel] = weight.Data[(output*otherChannels+channel)*kernel+tap]
						}
					}
				}
				model.convLayouts[prefix] = flat
			}
			continue
		}
		if len(weight.Shape) != 2 {
			continue
		}
		bias, ok := clean[prefix+".bias"]
		if ok && len(bias.Data) == weight.Shape[0] {
			model.packedLinear.Store(prefix, tensor.PackB(weight.Data, weight.Shape[0], weight.Shape[1], true))
		}
	}
	return model, nil
}
func LoadVoice(ptPath string) (*Voice, error) {
	w, e := torchpth.Read(ptPath)
	if e != nil {
		return nil, e
	}
	v, ok := w[""]
	if !ok {
		return nil, fmt.Errorf("kokoro: voice tensor absent")
	}
	if len(v.Shape) != 3 || v.Shape[0] != 510 || v.Shape[1] != 1 || v.Shape[2] != 256 {
		return nil, fmt.Errorf("kokoro: voice shape %v", v.Shape)
	}
	return &Voice{v.Data}, nil
}
func (m *Model) Tokens(phonemes string) []int {
	ids := make([]int, 0, len(phonemes))
	for _, r := range phonemes {
		if id, ok := m.config.Vocab[string(r)]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}
func (m *Model) value(name string) []float32 {
	v, ok := m.weights[name]
	if !ok {
		panic("kokoro: missing weight " + name)
	}
	return v.Data
}
func (m *Model) linear(x []float32, rows, in, out int, name string) []float32 {
	w := m.value(name + ".weight")
	b := m.value(name + ".bias")
	y := make([]float32, rows*out)
	packed, ok := m.packedLinear.Load(name)
	if !ok {
		packed, _ = m.packedLinear.LoadOrStore(name, tensor.PackB(w, out, in, true))
	}
	tensor.SGEMMPacked(y, x, packed.(*tensor.PackedB), rows, runtime.GOMAXPROCS(0))
	for r := 0; r < rows; r++ {
		for c := 0; c < out; c++ {
			y[r*out+c] += b[c]
		}
	}
	return y
}
func norm(x, w, b []float32, rows, ch int) {
	for r := 0; r < rows; r++ {
		row := x[r*ch : (r+1)*ch]
		var mean, v float64
		for _, z := range row {
			mean += float64(z)
		}
		mean /= float64(ch)
		for _, z := range row {
			d := float64(z) - mean
			v += d * d
		}
		v /= float64(ch)
		scale := float32(1 / math.Sqrt(v+1e-5))
		for j := range row {
			row[j] = (row[j]-float32(mean))*scale*w[j] + b[j]
		}
	}
}
func sigmoid(x float32) float32 { return 1 / (1 + float32(math.Exp(float64(-x)))) }
func leaky(x, alpha float32) float32 {
	if x < 0 {
		return alpha * x
	}
	return x
}
func parallelRows(rows, minRows int, fn func(int, int)) {
	workers := min(runtime.GOMAXPROCS(0), rows/minRows)
	if workers < 2 {
		fn(0, rows)
		return
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		start := i * rows / workers
		end := (i + 1) * rows / workers
		wg.Add(1)
		go func() { defer wg.Done(); fn(start, end) }()
	}
	wg.Wait()
}
func (m *Model) folded(name string) []float32 {
	if w, ok := m.weights[name+".weight"]; ok {
		return w.Data
	}
	v := m.value(name + ".weight_v")
	g := m.value(name + ".weight_g")
	shape := m.weights[name+".weight_v"].Shape
	out := make([]float32, len(v))
	if len(g) == shape[0] {
		step := len(v) / len(g)
		for c := range g {
			var sum float64
			for _, z := range v[c*step : (c+1)*step] {
				sum += float64(z) * float64(z)
			}
			s := float32(float64(g[c]) / math.Sqrt(sum))
			for j := 0; j < step; j++ {
				out[c*step+j] = v[c*step+j] * s
			}
		}
	} else if len(g) == shape[1] {
		step := shape[1] * shape[2]
		for c := range g {
			var sum float64
			for i := c * shape[2]; i < len(v); i += step {
				for k := 0; k < shape[2]; k++ {
					z := v[i+k]
					sum += float64(z) * float64(z)
				}
			}
			s := float32(float64(g[c]) / math.Sqrt(sum))
			for i := c * shape[2]; i < len(v); i += step {
				for k := 0; k < shape[2]; k++ {
					out[i+k] = v[i+k] * s
				}
			}
		}
	} else {
		panic("kokoro: unsupported weight norm " + name)
	}
	m.weights[name+".weight"] = torchpth.Tensor{Shape: shape, Data: out}
	return out
}

func (m *Model) conv(x []float32, t, in, out, kernel, stride, pad, dilation, groups int, name string) ([]float32, int) {
	m.layoutMu.Lock()
	flatWeights, ok := m.convLayouts[name]
	if !ok {
		w := m.folded(name)
		depth := in / groups * kernel
		flatWeights = make([]float32, len(w))
		outGroup := out / groups
		for group := 0; group < groups; group++ {
			for output := 0; output < outGroup; output++ {
				for channel := 0; channel < in/groups; channel++ {
					for tap := 0; tap < kernel; tap++ {
						flatWeights[(group*outGroup+output)*depth+tap*(in/groups)+channel] = w[((group*outGroup+output)*(in/groups)+channel)*kernel+tap]
					}
				}
			}
		}
		m.convLayouts[name] = flatWeights
	}
	depth := in / groups * kernel
	outGroup := out / groups
	packed, ok := m.convPacked[name]
	if !ok {
		packed = make([]*tensor.PackedB, groups)
		for group := range packed {
			packed[group] = tensor.PackB(flatWeights[group*outGroup*depth:(group+1)*outGroup*depth], outGroup, depth, true)
		}
		m.convPacked[name] = packed
	}
	m.layoutMu.Unlock()
	n := (t+2*pad-dilation*(kernel-1)-1)/stride + 1
	y := make([]float32, n*out)
	const block = 256
	blocks := (n + block - 1) / block
	// ブロックごとに並べ直しから行列積までを分担し、1つのブロックしかないときは行列積を並列にする。
	workers := min(runtime.GOMAXPROCS(0), blocks)
	gemmWorkers := max(1, runtime.GOMAXPROCS(0)/workers)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Go(func() {
			column := make([]float32, min(block, n)*depth)
			blockOutput := make([]float32, min(block, n)*outGroup)
			for index := worker; index < blocks; index += workers {
				start := index * block
				rows := min(block, n-start)
				for group := 0; group < groups; group++ {
					a := column[:rows*depth]
					clear(a)
					for r := 0; r < rows; r++ {
						for k := 0; k < kernel; k++ {
							src := (start+r)*stride + k*dilation - pad
							if src >= 0 && src < t {
								copy(a[r*depth+k*(in/groups):r*depth+(k+1)*(in/groups)], x[src*in+group*(in/groups):src*in+(group+1)*(in/groups)])
							}
						}
					}
					z := blockOutput[:rows*outGroup]
					tensor.SGEMMPacked(z, a, packed[group], rows, gemmWorkers)
					for r := 0; r < rows; r++ {
						copy(y[(start+r)*out+group*outGroup:(start+r)*out+(group+1)*outGroup], z[r*outGroup:(r+1)*outGroup])
					}
				}
			}
		})
	}
	wait.Wait()
	if b, ok := m.weights[name+".bias"]; ok {
		for r := 0; r < n; r++ {
			for c := 0; c < out; c++ {
				y[r*out+c] += b.Data[c]
			}
		}
	}
	return y, n
}
func (m *Model) convT(x []float32, t, in, out, kernel, stride, pad, outPad, groups int, name string) ([]float32, int) {
	m.layoutMu.Lock()
	flatWeights, ok := m.convTLayouts[name]
	if !ok {
		w := m.folded(name)
		flatWeights = make([]float32, len(w))
		inGroup := in / groups
		outGroup := out / groups
		for group := 0; group < groups; group++ {
			for channel := 0; channel < inGroup; channel++ {
				for output := 0; output < outGroup; output++ {
					for tap := 0; tap < kernel; tap++ {
						flatWeights[(group*inGroup+channel)*outGroup*kernel+tap*outGroup+output] = w[((group*inGroup+channel)*outGroup+output)*kernel+tap]
					}
				}
			}
		}
		m.convTLayouts[name] = flatWeights
	}
	m.layoutMu.Unlock()
	n := (t-1)*stride - 2*pad + kernel + outPad
	y := make([]float32, n*out)
	inGroup := in / groups
	outGroup := out / groups
	for group := 0; group < groups; group++ {
		a := make([]float32, t*inGroup)
		for ti := 0; ti < t; ti++ {
			copy(a[ti*inGroup:(ti+1)*inGroup], x[ti*in+group*inGroup:ti*in+(group+1)*inGroup])
		}
		flat := flatWeights[group*inGroup*outGroup*kernel : (group+1)*inGroup*outGroup*kernel]
		z := make([]float32, t*outGroup*kernel)
		tensor.SGEMM(z, a, flat, t, outGroup*kernel, inGroup)
		for ti := 0; ti < t; ti++ {
			for k := 0; k < kernel; k++ {
				dst := ti*stride + k - pad
				if dst < 0 || dst >= n {
					continue
				}
				for oc := 0; oc < outGroup; oc++ {
					y[dst*out+group*outGroup+oc] += z[ti*outGroup*kernel+k*outGroup+oc]
				}
			}
		}
	}
	if b, ok := m.weights[name+".bias"]; ok {
		for ti := 0; ti < n; ti++ {
			for c := 0; c < out; c++ {
				y[ti*out+c] += b.Data[c]
			}
		}
	}
	return y, n
}

func (m *Model) Synthesize(phonemes string, voice *Voice, speed float32, rng *torchrng.Generator) (Result, error) {
	if speed <= 0 {
		return Result{}, fmt.Errorf("kokoro: invalid synthesis arguments")
	}
	return m.synthesizePhonemes(phonemes, voice, speed, nil, rng)
}

// SynthesizeWithDurationsは、入力記号ごとの長さ（先頭と末尾の0を含む、600サンプル単位）を指定して合成する。
func (m *Model) SynthesizeWithDurations(phonemes string, voice *Voice, durations []int, rng *torchrng.Generator) (Result, error) {
	for _, d := range durations {
		if d < 1 {
			return Result{}, fmt.Errorf("kokoro: durations must be positive")
		}
	}
	return m.synthesizePhonemes(phonemes, voice, 1, durations, rng)
}

// Prosodyは、波形を作る前までの結果（記号ごとの長さとF0）。Decodeで波形にする。
type Prosody struct {
	Durations []int
	F0        []float32
	frames    int
	asr       []float32
	noise     []float32
	style     []float32
}

// ReadProsodyは、波形を作らずに記号ごとの長さとF0を求める。durationsがnilならspeedで長さを予測する。
func (m *Model) ReadProsody(phonemes string, voice *Voice, speed float32, durations []int) (*Prosody, error) {
	if voice == nil || speed <= 0 {
		return nil, fmt.Errorf("kokoro: invalid synthesis arguments")
	}
	for _, d := range durations {
		if d < 1 {
			return nil, fmt.Errorf("kokoro: durations must be positive")
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.Tokens(phonemes)
	if len(ids)+2 > m.config.PLBERT.MaxPosition {
		return nil, fmt.Errorf("kokoro: too many tokens")
	}
	runes := []rune(phonemes)
	if len(runes) < 1 || len(runes) > 510 {
		return nil, fmt.Errorf("kokoro: voice length out of range")
	}
	ref := voice.data[(len(runes)-1)*256 : len(runes)*256]
	ids = append(append([]int{0}, ids...), 0)
	if durations != nil && len(durations) != len(ids) {
		return nil, fmt.Errorf("kokoro: %d durations for %d tokens", len(durations), len(ids))
	}
	return m.prosody(ids, ref, speed, durations), nil
}

// Decodeは、ReadProsodyの結果から24 kHzの波形を作る。
func (m *Model) Decode(p *Prosody, rng *torchrng.Generator) ([]float32, error) {
	if p == nil || rng == nil {
		return nil, fmt.Errorf("kokoro: invalid synthesis arguments")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.decode(p.asr, p.frames, p.F0, p.noise, p.style, rng), nil
}

func (m *Model) synthesizePhonemes(phonemes string, voice *Voice, speed float32, durations []int, rng *torchrng.Generator) (Result, error) {
	if rng == nil {
		return Result{}, fmt.Errorf("kokoro: invalid synthesis arguments")
	}
	p, err := m.ReadProsody(phonemes, voice, speed, durations)
	if err != nil {
		return Result{}, err
	}
	audio, err := m.Decode(p, rng)
	if err != nil {
		return Result{}, err
	}
	return Result{audio, p.Durations, p.F0}, nil
}

func (m *Model) prosody(ids []int, ref []float32, speed float32, durations []int) *Prosody {
	bert := m.albert(ids)
	dEn := m.linear(bert, len(ids), 768, 512, "bert_encoder")
	d := m.durationEncoder(dEn, ref[128:])
	dur := append([]int(nil), durations...)
	if dur == nil {
		x := m.biLSTM(d, len(ids), 640, 256, "predictor.lstm")
		durLogits := m.linear(x, len(ids), 512, 50, "predictor.duration_proj.linear_layer")
		dur = make([]int, len(ids))
		for i := range dur {
			var sum float32
			for _, v := range durLogits[i*50 : (i+1)*50] {
				sum += sigmoid(v)
			}
			dur[i] = max(1, int(math.RoundToEven(float64(sum/speed))))
		}
	}
	total := 0
	for _, n := range dur {
		total += n
	}
	aligned := align(d, 640, dur, total)
	var text []float32
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		text = m.textEncoder(ids)
	}()
	f0, noise := m.f0n(aligned, total, ref[128:])
	wait.Wait()
	return &Prosody{Durations: dur, F0: f0, frames: total, asr: align(text, 512, dur, total), noise: noise, style: ref[:128]}
}
func align(x []float32, ch int, dur []int, total int) []float32 {
	y := make([]float32, total*ch)
	at := 0
	for i, n := range dur {
		for j := 0; j < n; j++ {
			copy(y[at*ch:(at+1)*ch], x[i*ch:(i+1)*ch])
			at++
		}
	}
	return y
}
