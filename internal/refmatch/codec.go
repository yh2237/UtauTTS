// Package refmatchは参照音声に近い音源のフレームを選び、WORLDで合成する。
package refmatch

import (
	"math"
	"sort"
)

const (
	melM0             = 1127.01048
	melF0             = 700.0
	codecFloorHz      = 40.0
	codecCeilHz       = 20000.0
	apIntervalHz      = 3000.0
	apUpperLimitHz    = 15000.0
	worldSafeGuardMin = 0.000000000001
)

// codecはWORLDのCodeSpectralEnvelope・DecodeSpectralEnvelope・CodeAperiodicity・DecodeAperiodicityと同じ値を返す。
type codec struct {
	rate, fftSize, dims int
	codeMelAxis         []float64
	codeFreqAxis        []float64
	codeBasis           []float64
	decodeMelAxis       []float64
	decodeFreqAxis      []float64
	decodeBasis         []float64
	apBands             int
}

func newCodec(rate, fftSize, dims int) *codec {
	half := fftSize / 2
	toMel := func(hz float64) float64 { return melM0 * math.Log(hz/melF0+1) }
	floor, ceil := toMel(codecFloorHz), toMel(math.Min(float64(rate)/2, codecCeilHz))
	c := &codec{rate: rate, fftSize: fftSize, dims: dims,
		codeMelAxis: make([]float64, half), codeFreqAxis: make([]float64, half+1),
		codeBasis: make([]float64, dims*half), decodeMelAxis: make([]float64, half+2),
		decodeFreqAxis: make([]float64, half+1), decodeBasis: make([]float64, dims*half)}
	for i := 0; i < half; i++ {
		mel := (ceil-floor)*float64(i)/float64(half) + floor
		c.codeMelAxis[i] = mel
		c.decodeMelAxis[i+1] = melF0 * (math.Exp(mel/melM0) - 1)
	}
	c.decodeMelAxis[half+1] = float64(rate) / 2
	for i := 0; i <= half; i++ {
		hz := float64(i) * float64(rate) / float64(fftSize)
		c.codeFreqAxis[i] = toMel(hz)
		c.decodeFreqAxis[i] = hz
	}
	codeScale := 2 / math.Sqrt(float64(fftSize)) / math.Sqrt(float64(half))
	decodeScale := math.Sqrt(float64(fftSize)) * math.Sqrt(float64(half))
	for k := 0; k < dims; k++ {
		theta := math.Pi * float64(k) / float64(fftSize)
		cs, ds := codeScale, decodeScale
		if k == 0 {
			cs /= math.Sqrt2
			ds /= math.Sqrt2
		}
		for p := 0; p < half; p++ {
			angle := 2 * math.Pi * float64(k*p%half) / float64(half)
			c.codeBasis[k*half+p] = cs * math.Cos(theta-angle)
			c.decodeBasis[k*half+p] = ds * math.Cos(angle+theta)
		}
	}
	c.apBands = int(math.Min(apUpperLimitHz, float64(rate)/2-apIntervalHz) / apIntervalHz)
	return c
}

func (c *codec) codeSpectrum(spectrum, coded []float64) {
	half := c.fftSize / 2
	logSpectrum := make([]float64, half+1)
	for i, v := range spectrum[:half+1] {
		logSpectrum[i] = math.Log(v)
	}
	mel := interp1(c.codeFreqAxis, logSpectrum, c.codeMelAxis)
	v := make([]float64, half)
	for i := 0; i < half/2; i++ {
		v[i] = mel[i*2]
		v[i+half/2] = mel[half-i*2-1]
	}
	for k := 0; k < c.dims; k++ {
		basis := c.codeBasis[k*half : (k+1)*half]
		var sum float64
		for p, x := range v {
			sum += x * basis[p]
		}
		coded[k] = sum
	}
}

func (c *codec) decodeSpectrum(coded, spectrum []float64) {
	half := c.fftSize / 2
	out := make([]float64, half)
	for k := 0; k < c.dims; k++ {
		basis := c.decodeBasis[k*half : (k+1)*half]
		for p := range out {
			out[p] += coded[k] * basis[p]
		}
	}
	mel := make([]float64, half+2)
	for i := 0; i < half/2; i++ {
		mel[1+i*2] = out[i]
		mel[2+i*2] = out[half-i-1]
	}
	mel[0], mel[half+1] = mel[1], mel[half]
	values := interp1(c.decodeMelAxis, mel, c.decodeFreqAxis)
	for i, v := range values {
		spectrum[i] = math.Exp(v / float64(half))
	}
}

func (c *codec) codeAperiodicity(aperiodicity, coded []float64) {
	bins := c.fftSize/2 + 1
	step := float64(c.rate) / float64(c.fftSize)
	logAP := make([]float64, bins)
	for i, v := range aperiodicity[:bins] {
		logAP[i] = 20 * math.Log10(v)
	}
	for band := 0; band < c.apBands; band++ {
		position := apIntervalHz * float64(band+1) / step
		base := int(position)
		fraction := position - float64(base)
		delta := 0.0
		if base+1 < bins {
			delta = logAP[base+1] - logAP[base]
		}
		coded[band] = logAP[base] + delta*fraction
	}
}

func (c *codec) decodeAperiodicity(coded, aperiodicity []float64) {
	bins := c.fftSize/2 + 1
	for i := range aperiodicity[:bins] {
		aperiodicity[i] = 1 - worldSafeGuardMin
	}
	var mean float64
	for _, v := range coded[:c.apBands] {
		mean += v
	}
	if mean/float64(c.apBands) > -0.5 {
		return
	}
	axis := make([]float64, c.apBands+2)
	values := make([]float64, c.apBands+2)
	for i := 0; i <= c.apBands; i++ {
		axis[i] = float64(i) * apIntervalHz
	}
	axis[c.apBands+1] = float64(c.rate) / 2
	values[0] = -60
	copy(values[1:], coded[:c.apBands])
	values[c.apBands+1] = -worldSafeGuardMin
	freq := make([]float64, bins)
	for i := range freq {
		freq[i] = float64(c.rate) / float64(c.fftSize) * float64(i)
	}
	for i, v := range interp1(axis, values, freq) {
		aperiodicity[i] = math.Pow(10, v/20)
	}
}

// interp1はWORLDのinterp1と同じく、範囲外を端の区間の傾きで外挿する。
func interp1(x, y, xi []float64) []float64 {
	out := make([]float64, len(xi))
	for i, v := range xi {
		k := sort.Search(len(x), func(j int) bool { return x[j] > v })
		k = min(max(k, 1), len(x)-1)
		out[i] = y[k-1] + (v-x[k-1])/(x[k]-x[k-1])*(y[k]-y[k-1])
	}
	return out
}
