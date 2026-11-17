package f0cache

import (
	"crypto/sha1"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

const WorldHarvest = "utautts_world_harvest"

type Token interface {
	F0Timing() (start, end float64, pause bool)
}

func pyFloat(v float64) string {
	s := strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func Path[T Token](dir, id, recordID string, tokens []T, step float64, source string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, t := range tokens {
		if i > 0 {
			b.WriteString(", ")
		}
		start, end, pause := t.F0Timing()
		b.WriteString("[" + pyFloat(start) + ", " + pyFloat(end) + ", " + strconv.FormatBool(pause) + "]")
	}
	b.WriteByte(']')
	if recordID != "" {
		id = recordID
	}
	key := fmt.Sprintf("%s|%g|%s|%s", id, step, source, b.String())
	h := sha1.Sum([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("%x.npy", h))
}

func TimeGrid[T Token](start, end float64, tokens []T, step float64) []float64 {
	if end <= start {
		start, _, _ = tokens[0].F0Timing()
		_, end, _ = tokens[len(tokens)-1].F0Timing()
	}
	n := max(1, int(math.Ceil((end-start)/step)))
	out := make([]float64, n)
	for i := range out {
		out[i] = start + (float64(i)+.5)*step
	}
	return out
}

func TokenAt[T Token](tokens []T, t float64) int {
	for i, x := range tokens {
		start, end, _ := x.F0Timing()
		if start <= t && t < math.Max(start+1e-6, end) {
			return i
		}
	}
	start, _, _ := tokens[0].F0Timing()
	if t < start {
		return 0
	}
	return len(tokens) - 1
}

func Interpolate(track, times []float64, step float64) []float64 {
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
