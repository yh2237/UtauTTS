package refmatch

import (
	"errors"
	"math"
	"runtime"
	"sync"

	"github.com/yh2237/gograd/tensor"
)

const (
	contextFrames   = 2
	contextWeight   = 0.5
	neighbors       = 4
	voicingPenalty  = 0.5
	smoothingFrames = 3
	postfilterBeta  = 0.4
	matchBlockRows  = 32
)

// Requestは参照音声の特徴量とF0。
type Request struct {
	// Featuresは参照音声の20msごとの特徴量（Frames×Dims）。
	Features []float32
	Frames   int
	// ReferenceF0は5msごとのF0（Hz、0は無声）。長さはFrames×4。
	ReferenceF0 []float64
	// CenterHzは出力のF0の中心。0なら音源のF0の中央値。
	CenterHz float64
	// PitchScaleは参照のF0の動きを何倍にするか。
	PitchScale float64
}

// Synthesizeは、参照の各フレームに前後2フレームまで含めて近い音源のフレームを4つ選び、その平均をWORLDで合成する。
func (index *Index) Synthesize(world World, request Request) ([]float64, error) {
	if request.Frames <= 0 || len(request.Features) != request.Frames*index.Dims || len(request.ReferenceF0) < request.Frames*subFrames {
		return nil, errors.New("reference features and F0 do not match")
	}
	f0 := request.ReferenceF0[:request.Frames*subFrames]
	nearest := index.nearest(request.Features, request.Frames, frameVoicing(f0))
	rows := request.Frames * subFrames
	spectrum := index.average(index.Spectrum, spectrumDims, nearest, rows)
	aperiodicity := index.average(index.Aperiodicity, index.APBands, nearest, rows)
	smooth(spectrum, spectrumDims)
	smooth(aperiodicity, index.APBands)

	c := newCodec(index.SampleRate, index.FFTSize, spectrumDims)
	bins := index.FFTSize/2 + 1
	outF0 := index.OutputF0(f0, request.CenterHz, request.PitchScale)
	decodedSpectrum := make([]float64, rows*bins)
	decodedAP := make([]float64, rows*bins)
	sharpened := make([]float64, spectrumDims)
	before := make([]float64, bins)
	for row := range rows {
		coded := spectrum[row*spectrumDims : (row+1)*spectrumDims]
		c.decodeSpectrum(coded, before)
		copy(sharpened, coded)
		for k := 2; k < spectrumDims; k++ {
			sharpened[k] *= 1 + postfilterBeta
		}
		after := decodedSpectrum[row*bins : (row+1)*bins]
		c.decodeSpectrum(sharpened, after)
		ratio := sum(before) / sum(after)
		for i := range after {
			after[i] *= ratio
		}
		ap := decodedAP[row*bins : (row+1)*bins]
		if outF0[row] == 0 {
			for i := range ap {
				ap[i] = 1
			}
			continue
		}
		c.decodeAperiodicity(aperiodicity[row*index.APBands:(row+1)*index.APBands], ap)
	}
	return world.Synthesize(outF0, decodedSpectrum, decodedAP, index.FFTSize, index.SampleRate, worldFrameMS)
}

func frameVoicing(f0 []float64) []bool {
	voiced := make([]bool, len(f0)/subFrames)
	for frame := range voiced {
		count := 0
		for _, v := range f0[frame*subFrames : (frame+1)*subFrames] {
			if v > 0 {
				count++
			}
		}
		voiced[frame] = count*2 >= subFrames
	}
	return voiced
}

// nearestは参照のフレームごとに、文脈を含めた類似度が高い音源のフレームをneighbors個返す。
func (index *Index) nearest(features []float32, frames int, queryVoiced []bool) [][]int {
	dims, n := index.Dims, index.Frames
	query := make([]float32, len(features))
	for t := range frames {
		copy(query[t*dims:], normalized(features[t*dims:(t+1)*dims]))
	}
	bankF0 := make([]float64, len(index.F0))
	for i, v := range index.F0 {
		bankF0[i] = float64(v)
	}
	bankVoiced := frameVoicing(bankF0)
	packed := index.packedFeatures()
	result := make([][]int, frames)
	blocks := make(chan int)
	var wg sync.WaitGroup
	workers := max(1, min(runtime.GOMAXPROCS(0), (frames+matchBlockRows-1)/matchBlockRows))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for start := range blocks {
				end := min(start+matchBlockRows, frames)
				first, last := max(0, start-contextFrames), min(frames, end+contextFrames)
				sim := make([]float32, (last-first)*n)
				tensor.SGEMMPacked(sim, query[first*dims:last*dims], packed, last-first, 1)
				for t := first; t < last; t++ {
					row := sim[(t-first)*n : (t-first+1)*n]
					for j := range row {
						if queryVoiced[t] != bankVoiced[j] {
							row[j] -= voicingPenalty
						}
					}
				}
				for t := start; t < end; t++ {
					result[t] = index.topContext(sim, first, last, t)
				}
			}
		}()
	}
	for start := 0; start < frames; start += matchBlockRows {
		blocks <- start
	}
	close(blocks)
	wg.Wait()
	return result
}

func (index *Index) topContext(sim []float32, first, last, t int) []int {
	n := index.Frames
	bestScore := make([]float32, 0, neighbors)
	best := make([]int, 0, neighbors)
	for j := range n {
		var total, count float32
		for delta := -contextFrames; delta <= contextFrames; delta++ {
			tt, jj := t+delta, j+delta
			if tt < first || tt >= last || jj < 0 || jj >= n || index.File[jj] != index.File[j] {
				continue
			}
			weight := float32(1)
			if delta != 0 {
				weight = contextWeight
			}
			total += weight * sim[(tt-first)*n+jj]
			count += weight
		}
		score := total / max(count, 1e-6)
		if len(best) == neighbors && score <= bestScore[neighbors-1] {
			continue
		}
		position := len(best)
		if position == neighbors {
			position--
		} else {
			best = append(best, 0)
			bestScore = append(bestScore, 0)
		}
		for position > 0 && bestScore[position-1] < score {
			best[position], bestScore[position] = best[position-1], bestScore[position-1]
			position--
		}
		best[position], bestScore[position] = j, score
	}
	return best
}

func (index *Index) average(values []float32, dims int, nearest [][]int, rows int) []float64 {
	out := make([]float64, rows*dims)
	for t, chosen := range nearest {
		for sub := range subFrames {
			target := out[(t*subFrames+sub)*dims : (t*subFrames+sub+1)*dims]
			for _, j := range chosen {
				source := values[(j*subFrames+sub)*dims : (j*subFrames+sub+1)*dims]
				for d, v := range source {
					target[d] += float64(v) / float64(len(chosen))
				}
			}
		}
	}
	return out
}

// smoothはscipy.ndimage.uniform_filter1d（size 3、reflect）と同じく時間方向に3行の平均をとる。
func smooth(values []float64, dims int) {
	rows := len(values) / dims
	if rows < 2 {
		return
	}
	original := append([]float64(nil), values...)
	for row := range rows {
		previous, next := max(row-1, 0), min(row+1, rows-1)
		for d := range dims {
			values[row*dims+d] = (original[previous*dims+d] + original[row*dims+d] + original[next*dims+d]) / smoothingFrames
		}
	}
}

// OutputF0は参照のF0を、中央値がcenterHz（0なら音源のF0の中央値）になるよう移し、動きをscale倍にする。
func (index *Index) OutputF0(reference []float64, centerHz, scale float64) []float64 {
	if centerHz <= 0 {
		centerHz = index.F0Median
	}
	var voiced []float64
	for _, v := range reference {
		if v > 0 {
			voiced = append(voiced, v)
		}
	}
	out := make([]float64, len(reference))
	if len(voiced) == 0 {
		return out
	}
	median := percentile(voiced, 50)
	for i, v := range reference {
		if v > 0 {
			out[i] = centerHz * math.Exp(scale*math.Log(math.Max(v, 1)/median))
		}
	}
	return out
}

func sum(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

// ReferenceF0は参照TTSのF0（f0FrameMSごと、60Hz以下は無声）を、有声区間の対数F0を約20msでなめらかにして5msごとに直す。
func ReferenceF0(f0 []float32, f0FrameMS float64, frames int) []float64 {
	var times, logF0 []float64
	voicedness := make([]float64, len(f0))
	axis := make([]float64, len(f0))
	for i, v := range f0 {
		axis[i] = float64(i) * f0FrameMS
		if v > 60 {
			voicedness[i] = 1
			times = append(times, axis[i])
			logF0 = append(logF0, math.Log(float64(v)))
		}
	}
	out := make([]float64, frames*subFrames)
	if len(times) == 0 {
		return out
	}
	filled := make([]float64, len(f0))
	for i, t := range axis {
		filled[i] = interpClamped(times, logF0, t)
	}
	smoothed := gaussianFilter(filled, 20/f0FrameMS)
	for i := range out {
		t := float64(i) * worldFrameMS
		if interpClamped(axis, voicedness, t) > 0.5 {
			out[i] = math.Exp(interpClamped(axis, smoothed, t))
		}
	}
	return out
}

// interpClampedはnumpy.interpと同じく範囲外を端の値にする。
func interpClamped(x, y []float64, at float64) float64 {
	if at <= x[0] {
		return y[0]
	}
	if at >= x[len(x)-1] {
		return y[len(y)-1]
	}
	k := 1
	for x[k] < at {
		k++
	}
	return y[k-1] + (at-x[k-1])/(x[k]-x[k-1])*(y[k]-y[k-1])
}

// gaussianFilterはscipy.ndimage.gaussian_filter1d（truncate 4、reflect）と同じ。
func gaussianFilter(values []float64, sigma float64) []float64 {
	radius := int(4*sigma + 0.5)
	weights := make([]float64, 2*radius+1)
	var total float64
	for i := range weights {
		x := float64(i - radius)
		weights[i] = math.Exp(-0.5 * x * x / (sigma * sigma))
		total += weights[i]
	}
	n := len(values)
	reflect := func(i int) int {
		for i < 0 || i >= n {
			if i < 0 {
				i = -i - 1
			}
			if i >= n {
				i = 2*n - i - 1
			}
		}
		return i
	}
	out := make([]float64, n)
	for i := range values {
		var acc float64
		for k, w := range weights {
			acc += w * values[reflect(i+k-radius)]
		}
		out[i] = acc / total
	}
	return out
}

// packedFeaturesは行列積のために並べ替えた特徴量を、索引ごとに1度だけ作る。
func (index *Index) packedFeatures() *tensor.PackedB {
	index.packOnce.Do(func() {
		index.packed = tensor.PackB(index.Features, index.Frames, index.Dims, true)
	})
	return index.packed
}
