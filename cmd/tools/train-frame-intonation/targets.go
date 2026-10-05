package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

func median(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	a := append([]float64(nil), x...)
	sort.Float64s(a)
	n := len(a)
	if n%2 == 1 {
		return a[n/2]
	}
	return (a[n/2-1] + a[n/2]) * .5
}
func interpolateF0(track []float64, times []float64, step float64) []float64 {
	out := make([]float64, len(times))
	for i, t := range times {
		p := t / step
		l := int(math.Floor(p))
		if l < 0 || l >= len(track) {
			continue
		}
		if math.Abs(p-float64(l)) < 1e-9 {
			out[i] = track[l]
			continue
		}
		h := l + 1
		if h >= len(track) || track[l] <= 0 || track[h] <= 0 {
			continue
		}
		w := p - float64(l)
		out[i] = math.Exp(math.Log(track[l])*(1-w) + math.Log(track[h])*w)
	}
	return out
}
func internalF0(r record, step float64) ([]float64, error) {
	path := strings.ReplaceAll(r.AudioPath, "\\", string(filepath.Separator))
	pcm, e := audio.ReadWav(path)
	if e != nil {
		return nil, e
	}
	rate := pcm.SampleRate
	ns := len(pcm.Data) / pcm.Channels
	samples := make([]float64, ns)
	for i := range samples {
		for c := 0; c < pcm.Channels; c++ {
			samples[i] += float64(pcm.Data[i*pcm.Channels+c]) / 32768
		}
		samples[i] /= float64(pcm.Channels)
	}
	hop := max(1, int(math.Round(float64(rate)*step/1000)))
	window := max(hop*4, int(math.Round(float64(rate)*.040)))
	if window%2 != 0 {
		window++
	}
	half := window / 2
	count := max(1, int(math.Ceil(float64(ns)/float64(hop))))
	out := make([]float64, count)
	energy := make([]float64, count)
	lo, hi := max(1, int(float64(rate)/600)), min(window-2, int(float64(rate)/50))
	hamming := make([]float64, window)
	for j := range hamming {
		hamming[j] = .54 - .46*math.Cos(2*math.Pi*float64(j)/float64(window-1))
	}
	x := make([]float64, window)
	prefix := make([]float64, window+1)
	for i := range out {
		center := i * hop
		mean := 0.0
		for j := range x {
			p := center + j - half
			if p >= 0 && p < ns {
				x[j] = samples[p]
			} else {
				x[j] = 0
			}
			mean += x[j]
		}
		mean /= float64(window)
		for j := range x {
			x[j] = (x[j] - mean) * hamming[j]
			prefix[j+1] = prefix[j] + x[j]*x[j]
		}
		energy[i] = math.Sqrt(prefix[window] / float64(window))
		if prefix[window] <= 1e-10 {
			continue
		}
		best, bestLag := .30, -1
		corr := make([]float64, hi-lo+1)
		for lag := lo; lag <= hi; lag++ {
			sum := 0.0
			for j := 0; j < window-lag; j++ {
				sum += x[j] * x[j+lag]
			}
			den := math.Sqrt(math.Max(1e-20, (prefix[window]-prefix[lag])*prefix[window-lag]))
			v := sum / den
			corr[lag-lo] = v
			if v > best {
				best, bestLag = v, lag
			}
		}
		if bestLag < 0 {
			continue
		}
		lag := float64(bestLag)
		j := bestLag - lo
		if j > 0 && j+1 < len(corr) {
			a, b, c := corr[j-1], corr[j], corr[j+1]
			den := a - 2*b + c
			if math.Abs(den) > 1e-9 {
				lag += .5 * (a - c) / den
			}
		}
		out[i] = float64(rate) / math.Max(1, lag)
	}
	var active []float64
	for _, e := range energy {
		if e > 1e-8 {
			active = append(active, e)
		}
	}
	if len(active) > 0 {
		sort.Float64s(active)
		p := .15 * float64(len(active)-1)
		i := int(p)
		percentile := active[i]
		if i+1 < len(active) {
			percentile += (active[i+1] - active[i]) * (p - float64(i))
		}
		threshold := math.Max(1e-5, percentile*.35)
		for i, e := range energy {
			if e < threshold {
				out[i] = 0
			}
		}
	}
	return out, nil
}
func loadF0(r record, step float64, cache string, times []float64) ([]float64, error) {
	return loadF0Source(r, step, cache, times, "internal", "")
}
func loadF0Source(r record, step float64, cache string, times []float64, source, worldPath string) ([]float64, error) {
	tag := "internal_autocorrelation"
	if source == "world" {
		tag = "utautts_world_harvest"
	}
	path := cachePathTag(cache, r, step, tag)
	if x, e := npyF64(path); e == nil && len(x) == len(times) {
		return x, nil
	}
	var track []float64
	var e error
	if source == "world" {
		track, e = worldF0(r, step, worldPath)
	} else {
		track, e = internalF0(r, step)
	}
	if e != nil {
		return nil, e
	}
	out := interpolateF0(track, times, step)
	for i, t := range times {
		if r.Tokens[tokenAt(r.Tokens, t)].Pause {
			out[i] = 0
		}
	}
	if e = os.MkdirAll(cache, 0755); e != nil {
		return nil, e
	}
	if e = writeNPY(path, out); e != nil {
		return nil, e
	}
	return out, nil
}
func writeNPY(path string, x []float64) error {
	if _, e := os.Stat(path); e == nil {
		return fmt.Errorf("refusing to overwrite %s", path)
	}
	f, e := toolutil.CreateExclusive(path)
	if e != nil {
		return e
	}
	defer f.Close()
	header := "{'descr': '<f8', 'fortran_order': False, 'shape': (" + fmt.Sprint(len(x)) + ",), }"
	pad := 16 - (10+len(header)+1)%16
	header += strings.Repeat(" ", pad) + "\n"
	var pre []byte
	pre = append(pre, []byte("\x93NUMPY\x01\x00")...)
	pre = append(pre, byte(len(header)), byte(len(header)>>8))
	pre = append(pre, header...)
	if _, e = f.Write(pre); e != nil {
		return e
	}
	for _, v := range x {
		bits := math.Float64bits(v)
		b := []byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24), byte(bits >> 32), byte(bits >> 40), byte(bits >> 48), byte(bits >> 56)}
		if _, e = f.Write(b); e != nil {
			return e
		}
	}
	return nil
}
func targetF0(f0 []float64, mask []bool, step, smooth, low, high float64) []float32 {
	n := len(f0)
	macro := make([]float64, n)
	for a := 0; a < n; {
		if !mask[a] {
			a++
			continue
		}
		b := a + 1
		for b < n && mask[b] {
			b++
		}
		var measured []int
		for i := a; i < b; i++ {
			if f0[i] > 0 {
				measured = append(measured, i)
			}
		}
		if len(measured) > 0 {
			logs := make([]float64, b-a)
			for i := a; i < b; i++ {
				if i <= measured[0] {
					logs[i-a] = math.Log(f0[measured[0]])
				} else if i >= measured[len(measured)-1] {
					logs[i-a] = math.Log(f0[measured[len(measured)-1]])
				} else {
					j := sort.SearchInts(measured, i)
					l, h := measured[j-1], measured[j]
					w := float64(i-l) / float64(h-l)
					logs[i-a] = math.Log(f0[l])*(1-w) + math.Log(f0[h])*w
				}
			}
			width := min(len(logs), max(1, int(math.Round(smooth/math.Max(step, 1e-6)))))
			if width%2 == 0 {
				width = max(1, width-1)
			}
			for i := a; i < b; i++ {
				sum := 0.0
				for k := -width / 2; k <= width/2; k++ {
					ix := max(0, min(len(logs)-1, i-a+k))
					sum += logs[ix]
				}
				macro[i] = math.Exp(sum / float64(width))
			}
		}
		a = b
	}
	var voiced []float64
	for i, v := range macro {
		if mask[i] && v > 0 {
			voiced = append(voiced, v)
		}
	}
	baseline := median(voiced)
	if baseline == 0 {
		baseline = 200
	}
	target := make([]float64, n)
	var active []float64
	for i, v := range macro {
		if mask[i] && v > 0 {
			target[i] = 1200 * math.Log2(v/baseline)
			active = append(active, target[i])
		}
	}
	center := median(active)
	out := make([]float32, n)
	for i, v := range macro {
		if mask[i] && v > 0 {
			out[i] = float32(math.Max(low, math.Min(high, target[i]-center)) / math.Max(1, math.Max(math.Abs(low), math.Abs(high))))
		}
	}
	return out
}
func prepareRecord(r record, names map[string]int, step, smooth, low, high float64, cache string) (example, error) {
	return prepareRecordSource(r, names, step, smooth, low, high, cache, "internal", "")
}
func prepareRecordSource(r record, names map[string]int, step, smooth, low, high float64, cache, source, worldPath string) (example, error) {
	times := timeGrid(r, step)
	f0, e := loadF0Source(r, step, cache, times, source, worldPath)
	if e != nil {
		return example{}, fmt.Errorf("%s: %w", r.ID, e)
	}
	mask := make([]bool, len(times))
	for i, t := range times {
		mask[i] = !r.Tokens[tokenAt(r.Tokens, t)].Pause
	}
	target := targetF0(f0, mask, step, smooth, low, high)
	for i, v := range f0 {
		if v <= 0 && target[i] == 0 { // targetF0 fills an entire voiced phrase; an all-unvoiced phrase stays masked out.
			a := i
			for a > 0 && mask[a-1] {
				a--
			}
			b := i
			for b+1 < len(mask) && mask[b+1] {
				b++
			}
			voiced := false
			for j := a; j <= b; j++ {
				if f0[j] > 0 {
					voiced = true
					break
				}
			}
			if !voiced {
				mask[i] = false
			}
		}
	}
	var sparse []featureValue
	start, end := times[0]-step*.5, times[len(times)-1]+step*.5
	for i, t := range times {
		for name, value := range frameFeatures(r, tokenAt(r.Tokens, t), t, start, end) {
			if col, ok := names[name]; ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				sparse = append(sparse, featureValue{i, col, float32(value)})
			}
		}
	}
	return example{r.ID, sparse, target, mask, len(times)}, nil
}
