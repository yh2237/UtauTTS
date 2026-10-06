// speechtimingは学習した読み上げに合わせて時間配分を調整する。
// 予測した包絡は出力せず、原音の包絡を時間方向だけ伸縮する。
package speechtiming

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Predictor interface {
	// 音素IDはこの配列の添字。
	Phones() []string
	// idsは現在・前・次の音素。contは音素内位置・対数長・相対対数F0・有声フラグ。
	Predict(ids [][3]int, cont [][4]float32) ([][]float32, error)
	Mels() int
}

//go:embed speech-timing-target-v1.safetensors
var embeddedTarget []byte

//go:embed speech-timing-target-en-v1.safetensors
var embeddedEnglishTarget []byte

var (
	defaultOnce   sync.Once
	defaultTarget *TCN
	defaultErr    error
	englishOnce   sync.Once
	englishTarget *TCN
	englishErr    error
)

func DefaultTarget() (*TCN, error) {
	defaultOnce.Do(func() {
		defaultTarget, defaultErr = LoadTCN(embeddedTarget)
	})
	return defaultTarget, defaultErr
}

// TargetForLanguageは言語別の時間伸縮モデルを返す。未対応の言語は日本語モデル。
func TargetForLanguage(language string) (*TCN, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "en", "en-us", "en-gb":
		englishOnce.Do(func() {
			englishTarget, englishErr = LoadTCN(embeddedEnglishTarget)
		})
		return englishTarget, englishErr
	default:
		return DefaultTarget()
	}
}

type tensor struct {
	shape []int
	data  []float32
}

func readSafeTensors(data []byte) (map[string]tensor, map[string]string, error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("safetensors: short file")
	}
	size := binary.LittleEndian.Uint64(data[:8])
	if size > uint64(len(data)-8) {
		return nil, nil, fmt.Errorf("safetensors: header size %d", size)
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(data[8:8+size], &header); err != nil {
		return nil, nil, fmt.Errorf("safetensors: %w", err)
	}
	body := data[8+size:]
	tensors := make(map[string]tensor, len(header))
	metadata := map[string]string{}
	for name, raw := range header {
		if name == "__metadata__" {
			if err := json.Unmarshal(raw, &metadata); err != nil {
				return nil, nil, fmt.Errorf("safetensors metadata: %w", err)
			}
			continue
		}
		var entry struct {
			DType   string   `json:"dtype"`
			Shape   []int    `json:"shape"`
			Offsets [2]int64 `json:"data_offsets"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, nil, fmt.Errorf("safetensors %s: %w", name, err)
		}
		if entry.DType != "F32" {
			return nil, nil, fmt.Errorf("safetensors %s: dtype %s", name, entry.DType)
		}
		count := 1
		for _, dim := range entry.Shape {
			count *= dim
		}
		start, end := entry.Offsets[0], entry.Offsets[1]
		if start < 0 || end < start || end > int64(len(body)) || end-start != int64(count*4) {
			return nil, nil, fmt.Errorf("safetensors %s: bad offsets", name)
		}
		values := make([]float32, count)
		for i := range values {
			values[i] = math.Float32frombits(binary.LittleEndian.Uint32(body[start+int64(i)*4:]))
		}
		tensors[name] = tensor{shape: entry.Shape, data: values}
	}
	return tensors, metadata, nil
}

type TCN struct {
	phones    []string
	embedDim  int
	hidden    int
	mels      int
	kernel    int
	dilations []int
	embed     tensor
	inW, inB  tensor
	blockW    []tensor
	blockB    []tensor
	normG     []tensor
	normB     []tensor
	outW      tensor
	outB      tensor
}

// モデル構成はsafetensorsの__metadata__から読む。
func LoadTCN(data []byte) (*TCN, error) {
	tensors, metadata, err := readSafeTensors(data)
	if err != nil {
		return nil, err
	}
	get := func(name string, dims int) (tensor, error) {
		value, ok := tensors[name]
		if !ok || len(value.shape) != dims {
			return tensor{}, fmt.Errorf("speech timing model: missing %s", name)
		}
		return value, nil
	}
	model := &TCN{phones: strings.Split(metadata["phones"], " ")}
	kernel, err := strconv.Atoi(metadata["kernel"])
	if err != nil || kernel <= 0 || kernel%2 == 0 {
		return nil, fmt.Errorf("speech timing model: kernel %q", metadata["kernel"])
	}
	model.kernel = kernel
	for field := range strings.FieldsSeq(metadata["dilations"]) {
		value, err := strconv.Atoi(field)
		if err != nil || value <= 0 {
			return nil, fmt.Errorf("speech timing model: dilation %q", field)
		}
		model.dilations = append(model.dilations, value)
	}
	if model.embed, err = get("phone.weight", 2); err != nil {
		return nil, err
	}
	model.embedDim = model.embed.shape[1]
	if model.embed.shape[0] != len(model.phones) {
		return nil, fmt.Errorf("speech timing model: %d phones for %d embeddings", len(model.phones), model.embed.shape[0])
	}
	if model.inW, err = get("inp.weight", 3); err != nil {
		return nil, err
	}
	if model.inB, err = get("inp.bias", 1); err != nil {
		return nil, err
	}
	model.hidden = model.inW.shape[0]
	if model.inW.shape[1] != 3*model.embedDim+4 || model.inW.shape[2] != 1 {
		return nil, fmt.Errorf("speech timing model: input shape %v", model.inW.shape)
	}
	for index := range model.dilations {
		w, err := get(fmt.Sprintf("blocks.%d.weight", index), 3)
		if err != nil {
			return nil, err
		}
		if w.shape[0] != model.hidden || w.shape[1] != model.hidden || w.shape[2] != kernel {
			return nil, fmt.Errorf("speech timing model: block %d shape %v", index, w.shape)
		}
		b, err := get(fmt.Sprintf("blocks.%d.bias", index), 1)
		if err != nil {
			return nil, err
		}
		g, err := get(fmt.Sprintf("norms.%d.weight", index), 1)
		if err != nil {
			return nil, err
		}
		nb, err := get(fmt.Sprintf("norms.%d.bias", index), 1)
		if err != nil {
			return nil, err
		}
		model.blockW, model.blockB = append(model.blockW, kernelMajor(w)), append(model.blockB, b)
		model.normG, model.normB = append(model.normG, g), append(model.normB, nb)
	}
	if model.outW, err = get("out.weight", 3); err != nil {
		return nil, err
	}
	if model.outB, err = get("out.bias", 1); err != nil {
		return nil, err
	}
	if model.outW.shape[1] != model.hidden || model.outW.shape[2] != 1 {
		return nil, fmt.Errorf("speech timing model: output shape %v", model.outW.shape)
	}
	model.mels = model.outW.shape[0]
	return model, nil
}

func (m *TCN) Phones() []string { return append([]string(nil), m.phones...) }
func (m *TCN) Mels() int        { return m.mels }

func (m *TCN) Predict(ids [][3]int, cont [][4]float32) ([][]float32, error) {
	frames := len(ids)
	if len(cont) != frames {
		return nil, fmt.Errorf("speech timing model: %d ids for %d inputs", frames, len(cont))
	}
	if frames == 0 {
		return nil, nil
	}
	inputs := 3*m.embedDim + 4
	x := make([]float32, frames*inputs)
	for t := range frames {
		row := x[t*inputs : (t+1)*inputs]
		for slot, id := range ids[t] {
			if id < 0 || id >= len(m.phones) {
				return nil, fmt.Errorf("speech timing model: phone id %d", id)
			}
			copy(row[slot*m.embedDim:(slot+1)*m.embedDim], m.embed.data[id*m.embedDim:(id+1)*m.embedDim])
		}
		copy(row[3*m.embedDim:], cont[t][:])
	}
	h := pointwise(x, frames, inputs, m.inW.data, m.inB.data, m.hidden)
	y := make([]float32, frames*m.hidden)
	for index, dilation := range m.dilations {
		m.dilatedConv(h, y, frames, index, dilation)
		parallelFrames(frames, func(t int) {
			row := y[t*m.hidden : (t+1)*m.hidden]
			layerNorm(row, m.normG[index].data, m.normB[index].data)
			dst := h[t*m.hidden : (t+1)*m.hidden]
			for c, value := range row {
				dst[c] += gelu(value)
			}
		})
	}
	out := pointwise(h, frames, m.hidden, m.outW.data, m.outB.data, m.mels)
	result := make([][]float32, frames)
	for t := range result {
		result[t] = out[t*m.mels : (t+1)*m.mels]
	}
	return result, nil
}

func (m *TCN) dilatedConv(h, y []float32, frames, index, dilation int) {
	w, b := m.blockW[index].data, m.blockB[index].data
	hidden, kernel := m.hidden, m.kernel
	half := kernel / 2
	parallelFrames(frames, func(t int) {
		row := y[t*hidden : (t+1)*hidden]
		copy(row, b)
		for k := range kernel {
			source := t + (k-half)*dilation
			if source < 0 || source >= frames {
				continue
			}
			input := h[source*hidden : (source+1)*hidden]
			tap := w[k*hidden*hidden : (k+1)*hidden*hidden]
			for o := range hidden {
				weights := tap[o*hidden : (o+1)*hidden]
				var sum float32
				for i, value := range input {
					sum += weights[i] * value
				}
				row[o] += sum
			}
		}
	})
}

// 内積を連続メモリで計算するため、[out,in,k]を[k,out,in]へ並べ替える。
func kernelMajor(w tensor) tensor {
	out, in, kernel := w.shape[0], w.shape[1], w.shape[2]
	data := make([]float32, len(w.data))
	for o := range out {
		for i := range in {
			for k := range kernel {
				data[(k*out+o)*in+i] = w.data[(o*in+i)*kernel+k]
			}
		}
	}
	return tensor{shape: []int{kernel, out, in}, data: data}
}

func pointwise(x []float32, frames, inputs int, w, b []float32, outputs int) []float32 {
	y := make([]float32, frames*outputs)
	parallelFrames(frames, func(t int) {
		input := x[t*inputs : (t+1)*inputs]
		row := y[t*outputs : (t+1)*outputs]
		for o := range outputs {
			weights := w[o*inputs : (o+1)*inputs]
			sum := b[o]
			for i, value := range input {
				sum += weights[i] * value
			}
			row[o] = sum
		}
	})
	return y
}

func layerNorm(row, gamma, beta []float32) {
	var mean, variance float64
	for _, value := range row {
		mean += float64(value)
	}
	mean /= float64(len(row))
	for _, value := range row {
		d := float64(value) - mean
		variance += d * d
	}
	variance /= float64(len(row))
	scale := 1 / math.Sqrt(variance+1e-5)
	for c, value := range row {
		row[c] = float32((float64(value)-mean)*scale)*gamma[c] + beta[c]
	}
}

func gelu(value float32) float32 {
	x := float64(value)
	return float32(0.5 * x * (1 + math.Erf(x/math.Sqrt2)))
}

func parallelFrames(frames int, body func(int)) {
	workers := min(runtime.GOMAXPROCS(0), max(1, frames/16))
	if workers <= 1 {
		for t := range frames {
			body(t)
		}
		return
	}
	var wg sync.WaitGroup
	chunk := (frames + workers - 1) / workers
	for start := 0; start < frames; start += chunk {
		end := min(frames, start+chunk)
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for t := start; t < end; t++ {
				body(t)
			}
		}(start, end)
	}
	wg.Wait()
}
