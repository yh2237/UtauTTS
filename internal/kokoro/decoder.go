package kokoro

import (
	"fmt"
	"math"
	"sync"

	"github.com/yh2237/gograd/torchrng"
)

const sineTableSize = 16384

var sineTable = func() [sineTableSize + 1]float32 {
	var table [sineTableSize + 1]float32
	for index := range table {
		table[index] = float32(math.Sin(2 * math.Pi * float64(index) / sineTableSize))
	}
	return table
}()

func fastSin(angle float32) float32 {
	position := angle * float32(sineTableSize/(2*math.Pi))
	if position > 1e7 || position < -1e7 {
		return float32(math.Sin(float64(angle)))
	}
	index := int(position)
	fraction := position - float32(index)
	if fraction < 0 {
		index--
		fraction++
	}
	index &= sineTableSize - 1
	return sineTable[index] + fraction*(sineTable[index+1]-sineTable[index])
}

var fft20Cos, fft20Sin [20][20]float64
var fft20Window [20]float64

func init() {
	for index := range fft20Window {
		fft20Window[index] = .5 - .5*math.Cos(2*math.Pi*float64(index)/20)
		for frequency := 0; frequency < 20; frequency++ {
			angle := 2 * math.Pi * float64(frequency*index) / 20
			fft20Cos[frequency][index] = math.Cos(angle)
			fft20Sin[frequency][index] = math.Sin(angle)
		}
	}
}

func concatChannels(t int, parts ...[]float32) []float32 {
	ch := 0
	for _, p := range parts {
		ch += len(p) / t
	}
	out := make([]float32, t*ch)
	for ti := 0; ti < t; ti++ {
		offset := 0
		for _, p := range parts {
			width := len(p) / t
			copy(out[ti*ch+offset:], p[ti*width:(ti+1)*width])
			offset += width
		}
	}
	return out
}
func (m *Model) decode(asr []float32, t int, f0, noise, style []float32, rng *torchrng.Generator) []float32 {
	f, _ := m.conv(f0, 2*t, 1, 1, 3, 2, 1, 1, 1, "decoder.F0_conv")
	n, _ := m.conv(noise, 2*t, 1, 1, 3, 2, 1, 1, 1, "decoder.N_conv")
	x, _ := m.adainBlock(concatChannels(t, asr, f, n), t, 514, 1024, false, style, "decoder.encode")
	res, _ := m.conv(asr, t, 512, 64, 1, 1, 0, 1, 1, "decoder.asr_res.0")
	for i := 0; i < 4; i++ {
		x = concatChannels(t, x, res, f, n)
		out := 1024
		up := i == 3
		if up {
			out = 512
		}
		x, t = m.adainBlock(x, t, 1090, out, up, style, fmt.Sprintf("decoder.decode.%d", i))
	}
	return m.generator(x, t, f0, style, rng)
}

func (m *Model) snake(x []float32, t, ch int, alpha []float32) {
	parallelRows(t, max(8, 8192/ch), func(start, end int) {
		for ti := start; ti < end; ti++ {
			for c := 0; c < ch; c++ {
				i := ti*ch + c
				a := alpha[c]
				s := fastSin(a * x[i])
				x[i] += s * s / a
			}
		}
	})
}
func (m *Model) resBlock(x []float32, t, ch, kernel int, dilations []int, style []float32, name string) []float32 {
	for j, dilation := range dilations {
		base := fmt.Sprintf("%s", name)
		y := m.adaIN(x, t, ch, style, fmt.Sprintf("%s.adain1.%d", base, j))
		m.snake(y, t, ch, m.value(fmt.Sprintf("%s.alpha1.%d", base, j)))
		y, _ = m.conv(y, t, ch, ch, kernel, 1, (kernel*dilation-dilation)/2, dilation, 1, fmt.Sprintf("%s.convs1.%d", base, j))
		y = m.adaIN(y, t, ch, style, fmt.Sprintf("%s.adain2.%d", base, j))
		m.snake(y, t, ch, m.value(fmt.Sprintf("%s.alpha2.%d", base, j)))
		y, _ = m.conv(y, t, ch, ch, kernel, 1, (kernel-1)/2, 1, 1, fmt.Sprintf("%s.convs2.%d", base, j))
		for i := range x {
			x[i] += y[i]
		}
	}
	return x
}
func (m *Model) generator(x []float32, t int, f0, style []float32, rng *torchrng.Generator) []float32 {
	harmonic := m.source(f0, rng)
	spec, phase, frames := stft20(harmonic)
	har := make([]float32, frames*22)
	for ti := 0; ti < frames; ti++ {
		for b := 0; b < 11; b++ {
			har[ti*22+b] = spec[b*frames+ti]
			har[ti*22+11+b] = phase[b*frames+ti]
		}
	}
	channels := 512
	ups := [2]int{10, 6}
	kernels := [2]int{20, 12}
	for i := 0; i < 2; i++ {
		for j, v := range x {
			x[j] = leaky(v, .1)
		}
		nconv := 1
		stride := 1
		pad := 0
		if i == 0 {
			nconv = 12
			stride = 6
			pad = 3
		}
		source, sourceT := m.conv(har, frames, 22, channels/2, nconv, stride, pad, 1, 1, fmt.Sprintf("decoder.generator.noise_convs.%d", i))
		var wait sync.WaitGroup
		wait.Add(1)
		go func() {
			defer wait.Done()
			source = m.resBlock(source, sourceT, channels/2, map[bool]int{true: 7, false: 11}[i == 0], []int{1, 3, 5}, style, fmt.Sprintf("decoder.generator.noise_res.%d", i))
		}()
		x, t = m.convT(x, t, channels, channels/2, kernels[i], ups[i], (kernels[i]-ups[i])/2, 0, 1, fmt.Sprintf("decoder.generator.ups.%d", i))
		wait.Wait()
		channels /= 2
		if i == 1 {
			padded := make([]float32, (t+1)*channels)
			copy(padded[0:channels], x[channels:2*channels])
			copy(padded[channels:], x)
			x = padded
			t++
		}
		if t != sourceT {
			panic(fmt.Sprintf("kokoro: source length mismatch %d %d", t, sourceT))
		}
		for j := range x {
			x[j] += source[j]
		}
		blocks := make([][]float32, 3)
		parallelRows(3, 1, func(start, end int) {
			for j := start; j < end; j++ {
				blocks[j] = m.resBlock(append([]float32(nil), x...), t, channels, []int{3, 7, 11}[j], []int{1, 3, 5}, style, fmt.Sprintf("decoder.generator.resblocks.%d", i*3+j))
			}
		})
		sum := blocks[0]
		for j := 1; j < 3; j++ {
			for index, value := range blocks[j] {
				sum[index] += value
			}
		}
		for j := range sum {
			sum[j] /= 3
		}
		x = sum
	}
	for j, v := range x {
		x[j] = leaky(v, .01)
	}
	x, t = m.conv(x, t, channels, 22, 7, 1, 3, 1, 1, "decoder.generator.conv_post")
	magnitude := make([]float32, 11*t)
	phase = make([]float32, 11*t)
	for ti := 0; ti < t; ti++ {
		for b := 0; b < 11; b++ {
			magnitude[b*t+ti] = float32(math.Exp(float64(x[ti*22+b])))
			phase[b*t+ti] = float32(math.Sin(float64(x[ti*22+11+b])))
		}
	}
	return istft20(magnitude, phase, t)
}

func (m *Model) source(f0 []float32, rng *torchrng.Generator) []float32 {
	n := len(f0) * 300
	initial := make([]float32, 9)
	rng.Uniform(initial, 0, 1)
	initial[0] = 0
	harmonics := make([]float32, n*9)
	for h := 0; h < 9; h++ {
		var phase float64
		low := make([]float32, len(f0))
		for i, v := range f0 {
			rad := v * float32(h+1) / 24000
			phase += float64(rad - float32(math.Floor(float64(rad))))
			low[i] = float32(phase) * float32(2*math.Pi) * 300
		}
		for ti := 0; ti < n; ti++ {
			src := (float32(ti)+.5)*float32(1.0/300) - .5
			lo := int(math.Floor(float64(src)))
			hi := lo + 1
			frac := src - float32(lo)
			lo = max(0, min(len(low)-1, lo))
			hi = max(0, min(len(low)-1, hi))
			p := low[lo]*(1-frac) + low[hi]*frac
			harmonics[ti*9+h] = float32(math.Sin(float64(p))) * .1
		}
	}
	random := make([]float32, n*9)
	for i := range random {
		rng.Normal(random[i:i+1], 0, 1)
	}
	weight := m.value("decoder.generator.m_source.l_linear.weight")
	bias := m.value("decoder.generator.m_source.l_linear.bias")[0]
	out := make([]float32, n)
	for ti := 0; ti < n; ti++ {
		voiced := f0[ti/300] > 10
		sum := bias
		for h := 0; h < 9; h++ {
			v := random[h*n+ti]
			if voiced {
				v = harmonics[ti*9+h] + v*.003
			} else {
				v *= .1 / 3
			}
			sum += weight[h] * v
		}
		out[ti] = float32(math.Tanh(float64(sum)))
	}
	unused := make([]float32, n)
	rng.Normal(unused, 0, 1)
	return out
}

func stft20(x []float32) ([]float32, []float32, int) {
	const fft = 20
	const hop = 5
	frames := len(x)/hop + 1
	spec := make([]float32, 11*frames)
	phase := make([]float32, 11*frames)
	for t := 0; t < frames; t++ {
		for k := 0; k < 11; k++ {
			var re, im float64
			for j := 0; j < fft; j++ {
				src := t*hop + j - 10
				if src < 0 {
					src = -src
				}
				if src >= len(x) {
					src = 2*len(x) - 2 - src
				}
				v := float64(x[src]) * fft20Window[j]
				re += v * fft20Cos[k][j]
				im -= v * fft20Sin[k][j]
			}
			if k == 0 || k == fft/2 {
				im = 0
			}
			at := k*frames + t
			spec[at] = float32(math.Hypot(re, im))
			phase[at] = float32(math.Atan2(im, re))
		}
	}
	return spec, phase, frames
}
func istft20(spec, phase []float32, frames int) []float32 {
	const fft = 20
	const hop = 5
	length := (frames - 1) * hop
	acc := make([]float64, length+fft)
	env := make([]float64, length+fft)
	for t := 0; t < frames; t++ {
		var re, im [20]float64
		for k := 0; k < 11; k++ {
			at := k*frames + t
			mag := float64(spec[at])
			angle := float64(phase[at])
			re[k] = mag * math.Cos(angle)
			im[k] = mag * math.Sin(angle)
		}
		for k := 1; k < 10; k++ {
			re[20-k] = re[k]
			im[20-k] = -im[k]
		}
		for j := 0; j < fft; j++ {
			var v float64
			for k := 0; k < fft; k++ {
				v += re[k]*fft20Cos[k][j] - im[k]*fft20Sin[k][j]
			}
			v /= fft
			at := t*hop + j
			acc[at] += v * fft20Window[j]
			env[at] += fft20Window[j] * fft20Window[j]
		}
	}
	out := make([]float32, length)
	for i := range out {
		at := i + 10
		if env[at] > 1e-11 {
			out[i] = float32(acc[at] / env[at])
		}
	}
	return out
}
