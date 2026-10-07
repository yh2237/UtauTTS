package main

// 内部自己相関F0（v10の学習と同じ抽出法）。cmd/tools/train-frame-intonation の実装を移植。

import (
	"math"
	"sort"
)

func internalF0Track(samples []float64, rate int, stepMS float64) []float64 {
	ns := len(samples)
	hop := max(1, int(math.Round(float64(rate)*stepMS/1000)))
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
	corr := make([]float64, hi-lo+1)
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
	return out
}
