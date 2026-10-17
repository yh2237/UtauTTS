package kokoro

import (
	"fmt"
	"math"

	"github.com/yh2237/gograd/tensor"
)

func (m *Model) albert(ids []int) []float32 {
	const emb = 128
	const width = 768
	n := len(ids)
	word := m.value("bert.embeddings.word_embeddings.weight")
	pos := m.value("bert.embeddings.position_embeddings.weight")
	typ := m.value("bert.embeddings.token_type_embeddings.weight")
	x := make([]float32, n*emb)
	for t, id := range ids {
		for c := 0; c < emb; c++ {
			x[t*emb+c] = (word[id*emb+c] + typ[c]) + pos[t*emb+c]
		}
	}
	norm(x, m.value("bert.embeddings.LayerNorm.weight"), m.value("bert.embeddings.LayerNorm.bias"), n, emb)
	x = m.linear(x, n, emb, width, "bert.encoder.embedding_hidden_mapping_in")
	base := "bert.encoder.albert_layer_groups.0.albert_layers.0"
	for layer := 0; layer < 12; layer++ {
		q := m.linear(x, n, width, width, base+".attention.query")
		k := m.linear(x, n, width, width, base+".attention.key")
		v := m.linear(x, n, width, width, base+".attention.value")
		context := make([]float32, len(x))
		for head := 0; head < 12; head++ {
			for t := 0; t < n; t++ {
				scores := make([]float32, n)
				peak := float32(math.Inf(-1))
				for s := 0; s < n; s++ {
					var dot float32
					for j := 0; j < 64; j++ {
						idx := head*64 + j
						dot += q[t*width+idx] * k[s*width+idx]
					}
					scores[s] = dot * .125
					if scores[s] > peak {
						peak = scores[s]
					}
				}
				var sum float32
				for s := range scores {
					scores[s] = float32(math.Exp(float64(scores[s] - peak)))
					sum += scores[s]
				}
				for s, p := range scores {
					p /= sum
					for j := 0; j < 64; j++ {
						idx := head*64 + j
						context[t*width+idx] += p * v[s*width+idx]
					}
				}
			}
		}
		attn := m.linear(context, n, width, width, base+".attention.dense")
		for i := range x {
			attn[i] += x[i]
		}
		norm(attn, m.value(base+".attention.LayerNorm.weight"), m.value(base+".attention.LayerNorm.bias"), n, width)
		ff := m.linear(attn, n, width, 2048, base+".ffn")
		for i, z := range ff {
			ff[i] = geluNew(z)
		}
		ff = m.linear(ff, n, 2048, width, base+".ffn_output")
		for i := range ff {
			ff[i] += attn[i]
		}
		norm(ff, m.value(base+".full_layer_layer_norm.weight"), m.value(base+".full_layer_layer_norm.bias"), n, width)
		x = ff
	}
	return x
}
func geluNew(x float32) float32 {
	cube := float32(math.Pow(float64(x), 3))
	inner := x + float32(.044715)*cube
	argument := float32(math.Sqrt(2/math.Pi)) * inner
	return (.5 * x) * (1 + float32(math.Tanh(float64(argument))))
}

func (m *Model) biLSTM(x []float32, n, in, hidden int, name string) []float32 {
	result := make([]float32, n*hidden*2)
	parallelRows(2, 1, func(start, end int) {
		for dir := start; dir < end; dir++ {
			suffix := ""
			if dir == 1 {
				suffix = "_reverse"
			}
			wi := m.value(name + ".weight_ih_l0" + suffix)
			wh := m.value(name + ".weight_hh_l0" + suffix)
			bi := m.value(name + ".bias_ih_l0" + suffix)
			bh := m.value(name + ".bias_hh_l0" + suffix)
			gx := make([]float32, n*hidden*4)
			tensor.SGEMMOpWorkers(gx, x, wi, n, hidden*4, in, false, true, 4)
			h := make([]float32, hidden)
			c := make([]float32, hidden)
			gates := make([]float32, hidden*4)
			for step := 0; step < n; step++ {
				t := step
				if dir == 1 {
					t = n - 1 - step
				}
				copy(gates, gx[t*hidden*4:(t+1)*hidden*4])
				for j := 0; j < hidden*4; j++ {
					gates[j] += bi[j] + bh[j]
				}
				for j := 0; j < hidden*4; j++ {
					row := wh[j*hidden : (j+1)*hidden]
					var sum float32
					for k, z := range h {
						sum += row[k] * z
					}
					gates[j] += sum
				}
				for j := 0; j < hidden; j++ {
					i := sigmoid(gates[j])
					f := sigmoid(gates[hidden+j])
					g := float32(math.Tanh(float64(gates[2*hidden+j])))
					o := sigmoid(gates[3*hidden+j])
					c[j] = f*c[j] + i*g
					h[j] = o * float32(math.Tanh(float64(c[j])))
					result[t*hidden*2+dir*hidden+j] = h[j]
				}
			}
		}
	})
	return result
}

func (m *Model) durationEncoder(x []float32, style []float32) []float32 {
	n := len(x) / 512
	z := concatStyle(x, n, 512, style)
	for layer := 0; layer < 3; layer++ {
		z = m.biLSTM(z, n, 640, 256, fmt.Sprintf("predictor.text_encoder.lstms.%d", 2*layer))
		base := fmt.Sprintf("predictor.text_encoder.lstms.%d.fc", 2*layer+1)
		aff := m.linear(style, 1, 128, 1024, base)
		norm(z, ones(512), zeros(512), n, 512)
		for t := 0; t < n; t++ {
			for c := 0; c < 512; c++ {
				z[t*512+c] = (1+aff[c])*z[t*512+c] + aff[512+c]
			}
		}
		z = concatStyle(z, n, 512, style)
	}
	return z
}
func ones(n int) []float32 {
	x := make([]float32, n)
	for i := range x {
		x[i] = 1
	}
	return x
}
func zeros(n int) []float32 { return make([]float32, n) }
func concatStyle(x []float32, n, ch int, style []float32) []float32 {
	y := make([]float32, n*(ch+len(style)))
	for t := 0; t < n; t++ {
		copy(y[t*(ch+len(style)):], x[t*ch:(t+1)*ch])
		copy(y[t*(ch+len(style))+ch:], style)
	}
	return y
}

func (m *Model) textEncoder(ids []int) []float32 {
	n := len(ids)
	emb := m.value("text_encoder.embedding.weight")
	x := make([]float32, n*512)
	for t, id := range ids {
		copy(x[t*512:(t+1)*512], emb[id*512:(id+1)*512])
	}
	for i := 0; i < 3; i++ {
		base := fmt.Sprintf("text_encoder.cnn.%d", i)
		x, _ = m.conv(x, n, 512, 512, 5, 1, 2, 1, 1, base+".0")
		norm(x, m.value(base+".1.gamma"), m.value(base+".1.beta"), n, 512)
		for j, v := range x {
			x[j] = leaky(v, .2)
		}
	}
	return m.biLSTM(x, n, 512, 256, "text_encoder.lstm")
}
