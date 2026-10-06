package speechtiming

import (
	"fmt"
	"math"
	"sort"
)

const (
	// WORLDのフレーム周期(ms)。
	FrameMS    = 10.0
	maxStretch = 2.0
	// 句末の子音も保護するため、ノート開始より前から伸縮を止める。
	finalConsonantMS = 120.0
	rampMS           = 60.0
	// この長さを超える隙間は休止とする(秒)。
	longPauseSec = 0.15
	melLowHz     = 40.0
	melHighHz    = 12000.0
)

// PhoneSpanはモデルへ渡す音素区間（フレーズ先頭基準のms）。codaを含む実際の区間。
type PhoneSpan struct {
	Label      string
	StartMS    float64
	DurationMS float64
}

// 時刻は合成計画基準のms。
type Mora struct {
	Text string
	// Spansがあれば音素区間としてそのまま使う。無ければTextをかなとして解析する。
	Spans                   []PhoneSpan
	NoteStartMS             float64
	DurationMS              float64
	EffectivePreutteranceMS float64
}

// Featuresは合成直前のWORLD特徴量（10msフレーム、Spectrum/Aperiodicityはframes×(FFTSize/2+1)）。
type Features struct {
	Frames       int
	FFTSize      int
	SampleRate   int
	F0           []float64
	Spectrum     []float64
	Aperiodicity []float64
}

// strengthは1が標準、0は無効。文頭余白はms。
func Warp(model Predictor, morae []Mora, leadingMarginMS float64, input Features, strength float64) (Features, error) {
	frames := input.Frames
	bins := input.FFTSize/2 + 1
	if frames < 2 || len(input.F0) != frames || len(input.Spectrum) != frames*bins || len(input.Aperiodicity) != frames*bins {
		return Features{}, fmt.Errorf("speech timing: inconsistent features")
	}
	if strength <= 0 || len(morae) == 0 {
		return input, nil
	}
	phones, starts, ends := phoneTimeline(morae, leadingMarginMS, frames)
	ids, cont, speech := frameInputs(model.Phones(), phones, input.F0)
	target, err := model.Predict(ids, cont)
	if err != nil {
		return Features{}, err
	}
	if len(target) != frames {
		return Features{}, fmt.Errorf("speech timing: %d target frames for %d", len(target), frames)
	}
	source := logMel(input.Spectrum, frames, input.FFTSize, input.SampleRate, model.Mels())
	targetRows := make([][]float64, frames)
	for t, row := range target {
		targetRows[t] = make([]float64, len(row))
		for c, value := range row {
			targetRows[t][c] = float64(value)
		}
	}
	normalize(source, speech)
	normalize(targetRows, speech)
	anchors := make([]float64, 0, len(starts)+len(ends))
	for _, value := range append(append([]float64(nil), starts...), ends...) {
		anchors = append(anchors, value*1000/FrameMS)
	}
	raw := warpMap(source, targetRows, anchors, frames)
	weight := protectPhraseEnds(frames, starts, ends)
	positions := make([]float64, frames)
	for t := range positions {
		value := float64(t) + strength*weight[t]*(raw[t]-float64(t))
		value = math.Min(math.Max(value, 0), float64(frames-1))
		if t > 0 {
			value = math.Max(value, positions[t-1])
		}
		positions[t] = value
	}
	return resample(input, positions), nil
}

func frameInputs(phoneNames []string, phones []phoneSpan, f0 []float64) ([][3]int, [][4]float32, []bool) {
	index := make(map[string]int, len(phoneNames))
	for i, name := range phoneNames {
		index[name] = i
	}
	id := func(label string) int {
		if value, ok := index[label]; ok {
			return value
		}
		return index["<unk>"]
	}
	frames := len(f0)
	ids := make([][3]int, frames)
	cont := make([][4]float32, frames)
	speech := make([]bool, frames)
	silence := id("sil")
	for t := range ids {
		ids[t] = [3]int{silence, silence, silence}
	}
	for i, phone := range phones {
		a := int(math.Round(phone.start * 1000 / FrameMS))
		b := int(math.Round(phone.end * 1000 / FrameMS))
		a = max(0, a)
		b = min(frames, max(b, a+1))
		if a >= frames {
			break
		}
		previous, next := "sil", "sil"
		if i > 0 {
			previous = phones[i-1].label
		}
		if i+1 < len(phones) {
			next = phones[i+1].label
		}
		duration := float32(math.Log((phone.end-phone.start)*1000+1) / 6)
		for t := a; t < b; t++ {
			ids[t] = [3]int{id(phone.label), id(previous), id(next)}
			cont[t][0] = (float32(t-a) + 0.5) / float32(max(1, b-a))
			cont[t][1] = duration
		}
		if phone.label != "sil" {
			from := int(phone.start * 1000 / FrameMS)
			to := int(phone.end*1000/FrameMS) + 1
			for t := max(0, from); t < min(frames, to); t++ {
				speech[t] = true
			}
		}
	}
	var sum float64
	var voiced []int
	for t, value := range f0 {
		if value > 0 {
			voiced = append(voiced, t)
			sum += math.Log(value)
		}
	}
	if len(voiced) > 0 {
		mean := sum / float64(len(voiced))
		for t := range frames {
			cont[t][2] = float32(interpLogF0(f0, voiced, t, mean) / 0.3)
			if f0[t] > 0 {
				cont[t][3] = 1
			}
		}
	}
	return ids, cont, speech
}

func interpLogF0(f0 []float64, voiced []int, t int, mean float64) float64 {
	k := sort.SearchInts(voiced, t)
	switch {
	case k < len(voiced) && voiced[k] == t:
		return math.Log(f0[t]) - mean
	case k == 0:
		return math.Log(f0[voiced[0]]) - mean
	case k == len(voiced):
		return math.Log(f0[voiced[len(voiced)-1]]) - mean
	}
	left, right := voiced[k-1], voiced[k]
	w := float64(t-left) / float64(right-left)
	return math.Log(f0[left])*(1-w) + math.Log(f0[right])*w - mean
}

// 学習と同じ40Hz〜12kHzの三角フィルタで対数メル(dB)へ変換する。
func logMel(spectrum []float64, frames, fftSize, sampleRate, mels int) [][]float64 {
	bins := fftSize/2 + 1
	mel := func(hz float64) float64 { return 2595 * math.Log10(1+hz/700) }
	points := make([]float64, mels+2)
	low, high := mel(melLowHz), mel(melHighHz)
	for i := range points {
		points[i] = 700 * (math.Pow(10, (low+(high-low)*float64(i)/float64(mels+1))/2595) - 1)
	}
	type weight struct {
		bin   int
		value float64
	}
	filters := make([][]weight, mels)
	for m := range mels {
		lo, mid, hi := points[m], points[m+1], points[m+2]
		var total float64
		for bin := range bins {
			hz := float64(bin) * float64(sampleRate) / float64(fftSize)
			value := math.Max(0, math.Min((hz-lo)/(mid-lo), (hi-hz)/(hi-mid)))
			if value > 0 {
				filters[m] = append(filters[m], weight{bin, value})
				total += value
			}
		}
		for i := range filters[m] {
			filters[m][i].value /= math.Max(total, 1e-12)
		}
	}
	result := make([][]float64, frames)
	for t := range frames {
		row := spectrum[t*bins : (t+1)*bins]
		result[t] = make([]float64, mels)
		for m, filter := range filters {
			var sum float64
			for _, w := range filter {
				sum += row[w.bin] * w.value
			}
			result[t][m] = 10 * math.Log10(sum+1e-12)
		}
	}
	return result
}

// 声質差を減らすため、発声フレームの平均と標準偏差で正規化する。
func normalize(rows [][]float64, speech []bool) {
	if len(rows) == 0 {
		return
	}
	count := 0
	for _, value := range speech {
		if value {
			count++
		}
	}
	use := func(t int) bool { return count < 2 || speech[t] }
	n := float64(max(count, 2))
	if count < 2 {
		n = float64(len(rows))
	}
	for c := range rows[0] {
		var mean float64
		for t, row := range rows {
			if use(t) {
				mean += row[c]
			}
		}
		mean /= n
		var variance float64
		for t, row := range rows {
			if use(t) {
				d := row[c] - mean
				variance += d * d
			}
		}
		scale := 1 / (math.Sqrt(variance/n) + 1e-3)
		for _, row := range rows {
			row[c] = (row[c] - mean) * scale
		}
	}
}

// 出力フレームから原音フレームへの対応。anchorsは固定点。
func warpMap(source, target [][]float64, anchors []float64, frames int) []float64 {
	edgeSet := map[int]bool{0: true, frames: true}
	for _, anchor := range anchors {
		value := int(math.Round(anchor))
		if value > 0 && value < frames {
			edgeSet[value] = true
		}
	}
	edges := make([]int, 0, len(edgeSet))
	for value := range edgeSet {
		edges = append(edges, value)
	}
	sort.Ints(edges)
	mapping := make([]float64, frames)
	for t := range mapping {
		mapping[t] = float64(t)
	}
	for i := 0; i+1 < len(edges); i++ {
		a, b := edges[i], edges[i+1]
		if b-a < 3 {
			continue
		}
		local := dtwSegment(source[a:b], target[a:b])
		for t, value := range local {
			mapping[a+t] = float64(a) + value
		}
	}
	smooth := make([]float64, frames)
	for t := range frames {
		var sum float64
		// 恒等写像を端でも保つため、線形に外挿する。
		for k := -2; k <= 2; k++ {
			switch index := t + k; {
			case index < 0:
				sum += mapping[0] + float64(index)
			case index >= frames:
				sum += mapping[frames-1] + float64(index-frames+1)
			default:
				sum += mapping[index]
			}
		}
		smooth[t] = sum / 5
	}
	for t := 1; t < frames; t++ {
		step := math.Min(math.Max(smooth[t]-smooth[t-1], 1/maxStretch), maxStretch)
		smooth[t] = smooth[t-1] + step
	}
	for i := 0; i+1 < len(edges); i++ {
		a, b := edges[i], edges[i+1]
		if b-a < 2 {
			continue
		}
		lo, hi := smooth[a], smooth[b-1]
		if hi-lo > 1e-6 {
			for t := a; t < b; t++ {
				smooth[t] = float64(a) + (smooth[t]-lo)*float64(b-1-a)/(hi-lo)
			}
		}
	}
	for t := range smooth {
		smooth[t] = math.Min(math.Max(smooth[t], 0), float64(frames-1))
	}
	return smooth
}

// DTWの移動は(1,1)、(1,2)、(2,1)に制限する。
func dtwSegment(source, target [][]float64) []float64 {
	n, m := len(target), len(source)
	result := make([]float64, n)
	if n == 0 {
		return result
	}
	if m <= 1 || n <= 1 {
		for i := range result {
			if n > 1 {
				result[i] = float64(max(m-1, 0)) * float64(i) / float64(n-1)
			}
		}
		return result
	}
	cost := make([]float64, n*m)
	for i := range n {
		for j := range m {
			var sum float64
			for c := range target[i] {
				d := target[i][c] - source[j][c]
				sum += d * d
			}
			cost[i*m+j] = sum / float64(len(target[i]))
		}
	}
	const inf = 1e18
	acc := make([]float64, n*m)
	for i := range acc {
		acc[i] = inf
	}
	acc[0] = cost[0]
	at := func(i, j int) float64 {
		if i < 0 || j < 0 {
			return inf
		}
		return acc[i*m+j]
	}
	for i := 1; i < n; i++ {
		for j := range m {
			best := at(i-1, j-1)
			if j >= 2 {
				best = math.Min(best, at(i-1, j-2))
			}
			if i >= 2 && j >= 1 {
				best = math.Min(best, at(i-2, j-1)+cost[(i-1)*m+j])
			}
			acc[i*m+j] = best + cost[i*m+j]
		}
	}
	path := map[int]int{n - 1: m - 1}
	i, j := n-1, m-1
	for i > 0 {
		type option struct {
			value float64
			i, j  int
		}
		var options []option
		if j >= 1 {
			options = append(options, option{at(i-1, j-1), i - 1, j - 1})
		}
		if j >= 2 {
			options = append(options, option{at(i-1, j-2), i - 1, j - 2})
		}
		if i >= 2 && j >= 1 {
			options = append(options, option{at(i-2, j-1), i - 2, j - 1})
		}
		if len(options) == 0 {
			break
		}
		best := options[0]
		for _, candidate := range options[1:] {
			if candidate.value < best.value || (candidate.value == best.value && (candidate.i < best.i || (candidate.i == best.i && candidate.j < best.j))) {
				best = candidate
			}
		}
		i, j = best.i, best.j
		path[i] = j
	}
	known := make([]int, 0, len(path))
	for key := range path {
		known = append(known, key)
	}
	sort.Ints(known)
	for t := range result {
		k := sort.SearchInts(known, t)
		switch {
		case k < len(known) && known[k] == t:
			result[t] = float64(path[t])
		case k == 0:
			result[t] = float64(path[known[0]])
		case k == len(known):
			result[t] = float64(path[known[len(known)-1]])
		default:
			left, right := known[k-1], known[k]
			w := float64(t-left) / float64(right-left)
			result[t] = float64(path[left])*(1-w) + float64(path[right])*w
		}
	}
	return result
}

// 句末が切れないよう、最後のモーラは子音も含めて伸縮しない。
func protectPhraseEnds(frames int, starts, ends []float64) []float64 {
	weight := make([]float64, frames)
	for t := range weight {
		weight[t] = 1
	}
	ramp := rampMS / FrameMS
	for _, end := range ends {
		last := end
		for _, start := range starts {
			if start < end {
				last = start
			}
		}
		hold := last*1000/FrameMS - finalConsonantMS/FrameMS
		stop := end*1000/FrameMS + 3
		for t := range frames {
			value := float64(t)
			if value >= hold && value < stop {
				weight[t] = 0
			} else if value < hold && value >= hold-ramp {
				weight[t] = math.Min(weight[t], (hold-value)/ramp)
			}
		}
	}
	return weight
}

// 有声判定は原音位置に従い、F0の値は元の曲線を保つ。
func resample(input Features, positions []float64) Features {
	frames := input.Frames
	bins := input.FFTSize/2 + 1
	output := Features{Frames: frames, FFTSize: input.FFTSize, SampleRate: input.SampleRate,
		F0: make([]float64, frames), Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins)}
	var voiced []int
	for t, value := range input.F0 {
		if value > 0 {
			voiced = append(voiced, t)
		}
	}
	for t, position := range positions {
		left := int(math.Floor(position))
		right := min(left+1, frames-1)
		w := position - float64(left)
		for bin := range bins {
			for _, pair := range [][2][]float64{{input.Spectrum, output.Spectrum}, {input.Aperiodicity, output.Aperiodicity}} {
				a := math.Log(math.Max(pair[0][left*bins+bin], 1e-16))
				b := math.Log(math.Max(pair[0][right*bins+bin], 1e-16))
				pair[1][t*bins+bin] = math.Exp(a*(1-w) + b*w)
			}
			output.Aperiodicity[t*bins+bin] = math.Min(output.Aperiodicity[t*bins+bin], 1)
		}
		nearest := min(max(int(math.Round(position)), 0), frames-1)
		if input.F0[nearest] > 0 && len(voiced) > 0 {
			if input.F0[t] > 0 {
				output.F0[t] = input.F0[t]
			} else {
				output.F0[t] = math.Exp(interpLogF0(input.F0, voiced, t, 0))
			}
		}
	}
	return output
}
