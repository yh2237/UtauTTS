package kokoro

import (
	"fmt"
	"math"
	"sync"
)

func (m *Model) adaIN(x []float32, t, ch int, style []float32, name string) []float32 {
	aff := m.linear(style, 1, len(style), 2*ch, name+".fc")
	y := make([]float32, len(x))
	mean := make([]float32, ch)
	variance := make([]float32, ch)
	for ti := 0; ti < t; ti++ {
		row := x[ti*ch : (ti+1)*ch]
		for channel, value := range row {
			mean[channel] += value
		}
	}
	for channel := range mean {
		mean[channel] /= float32(t)
	}
	for ti := 0; ti < t; ti++ {
		row := x[ti*ch : (ti+1)*ch]
		for channel, value := range row {
			difference := value - mean[channel]
			variance[channel] += difference * difference
		}
	}
	for channel := range variance {
		variance[channel] = float32(1/math.Sqrt(float64(variance[channel]/float32(t)+1e-5))) * (1 + aff[channel])
	}
	parallelRows(t, max(8, 8192/ch), func(start, end int) {
		for ti := start; ti < end; ti++ {
			for channel, value := range x[ti*ch : (ti+1)*ch] {
				y[ti*ch+channel] = (value-mean[channel])*variance[channel] + aff[ch+channel]
			}
		}
	})
	return y
}
func (m *Model) adainBlock(x []float32, t, in, out int, upsample bool, style []float32, name string) ([]float32, int) {
	shortcut := x
	length := t
	if upsample {
		length = t * 2
		shortcut = make([]float32, length*in)
		for ti := 0; ti < t; ti++ {
			copy(shortcut[2*ti*in:(2*ti+1)*in], x[ti*in:(ti+1)*in])
			copy(shortcut[(2*ti+1)*in:(2*ti+2)*in], x[ti*in:(ti+1)*in])
		}
	}
	if in != out {
		shortcut, _ = m.conv(shortcut, length, in, out, 1, 1, 0, 1, 1, name+".conv1x1")
	}
	y := m.adaIN(x, t, in, style, name+".norm1")
	for i, z := range y {
		y[i] = leaky(z, .2)
	}
	if upsample {
		y, t = m.convT(y, t, in, in, 3, 2, 1, 1, in, name+".pool")
	}
	y, t = m.conv(y, t, in, out, 3, 1, 1, 1, 1, name+".conv1")
	y = m.adaIN(y, t, out, style, name+".norm2")
	for i, z := range y {
		y[i] = leaky(z, .2)
	}
	y, t = m.conv(y, t, out, out, 3, 1, 1, 1, 1, name+".conv2")
	const scale = .7071067811865476
	for i := range y {
		y[i] = (y[i] + shortcut[i]) * scale
	}
	return y, t
}
func (m *Model) f0n(x []float32, t int, style []float32) ([]float32, []float32) {
	shared := m.biLSTM(x, t, 640, 256, "predictor.shared")
	run := func(kind string) []float32 {
		y := shared
		n := t
		ch := 512
		for i := 0; i < 3; i++ {
			next := ch
			if i == 1 {
				next = 256
			}
			y, n = m.adainBlock(y, n, ch, next, i == 1, style, fmt.Sprintf("predictor.%s.%d", kind, i))
			ch = next
		}
		y, _ = m.conv(y, n, 256, 1, 1, 1, 0, 1, 1, "predictor."+kind+"_proj")
		return y
	}
	var f0, noise []float32
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		f0 = run("F0")
	}()
	noise = run("N")
	wait.Wait()
	return f0, noise
}
