package base

import (
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/pitch"
	"utautts/internal/plan"
)

// EstimateUnitPitchは録音区間の代表F0を推定し、結果をキャッシュする。
func EstimateUnitPitch(unit plan.Unit, mono *audio.PCM) (float64, error) {
	key := unitPitchCacheKey{
		path: unit.Source, offset: math.Float64bits(unit.OffsetMS), cutoff: math.Float64bits(unit.CutoffMS),
		consonant: math.Float64bits(unit.ConsonantMS),
	}
	if info, err := os.Stat(unit.Source); err == nil {
		key.size = info.Size()
		key.modTime = info.ModTime().UnixNano()
	}
	globalUnitPitchCache.Lock()
	if element, found := globalUnitPitchCache.entries[key]; found {
		globalUnitPitchCache.order.MoveToFront(element)
		value := element.Value.(unitPitchCacheEntry).value
		globalUnitPitchCache.Unlock()
		return value, nil
	}
	globalUnitPitchCache.Unlock()

	trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
	if err != nil {
		return 0, err
	}
	wave := pcmFloats(trimmed.Data)
	start := min(len(wave), msToFrames(unit.ConsonantMS, mono.SampleRate))
	end := min(len(wave), start+msToFrames(180, mono.SampleRate))
	value := 0.0
	if end-start >= msToFrames(30, mono.SampleRate) {
		value = pitch.EstimateMedian(wave[start:end], mono.SampleRate)
	}

	globalUnitPitchCache.Lock()
	if element, found := globalUnitPitchCache.entries[key]; found {
		globalUnitPitchCache.order.MoveToFront(element)
		value = element.Value.(unitPitchCacheEntry).value
	} else {
		element := globalUnitPitchCache.order.PushFront(unitPitchCacheEntry{key: key, value: value})
		globalUnitPitchCache.entries[key] = element
		if globalUnitPitchCache.order.Len() > maxUnitPitchCacheEntries {
			oldest := globalUnitPitchCache.order.Back()
			delete(globalUnitPitchCache.entries, oldest.Value.(unitPitchCacheEntry).key)
			globalUnitPitchCache.order.Remove(oldest)
		}
	}
	globalUnitPitchCache.Unlock()
	return value, nil
}

// EffectiveUnitPitchFactorはピッチ処理が有効なときだけ手動係数を返す。
func EffectiveUnitPitchFactor(unit plan.Unit, applyPitch bool) float64 {
	if !applyPitch || unit.PitchFactor <= 0 {
		return 1
	}
	return unit.PitchFactor
}

// PitchCurveFactorAtはセント単位カーブの指定時刻の倍率を返す。
func PitchCurveFactorAt(curve *PitchCurve, timeMS float64) float64 {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return 1
	}
	position := math.Max(0, timeMS) / curve.FrameMS
	left := int(math.Floor(position))
	if left >= len(curve.Cents)-1 {
		return math.Pow(2, curve.Cents[len(curve.Cents)-1]/1200)
	}
	progress := position - float64(left)
	cents := curve.Cents[left]*(1-progress) + curve.Cents[left+1]*progress
	return math.Pow(2, cents/1200)
}

func clampPitchFactor(factor float64) float64 {
	return math.Max(0.75, math.Min(1.35, factor))
}

// ClampPitchFactorはピッチ倍率を有界にする。
func ClampPitchFactor(factor float64) float64 { return clampPitchFactor(factor) }

// ResampleForPitchは一定倍率でsourceを伸縮する。
func ResampleForPitch(source []float64, factor float64) []float64 {
	if len(source) < 16 || factor <= 0 || math.Abs(factor-1) < 0.001 {
		return append([]float64(nil), source...)
	}
	factor = clampPitchFactor(factor)
	return linearResample(source, max(16, int(math.Round(float64(len(source))/factor))))
}

// ResampleForPitchCurveは1/f(t)を積分し、時間変化するピッチでsourceを伸縮する。
// 平坦なカーブはResampleForPitchと同じ結果になる。
func ResampleForPitchCurve(source []float64, baseFactor float64, curve *PitchCurve, startMS, spanMS float64) []float64 {
	if len(source) < 16 || baseFactor <= 0 {
		return append([]float64(nil), source...)
	}
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return ResampleForPitch(source, baseFactor)
	}
	span := math.Max(1e-3, spanMS)
	cumulative := make([]float64, len(source)+1)
	for i := range source {
		frame := 0.0
		if len(source) > 1 {
			frame = float64(i) / float64(len(source)-1)
		}
		factor := clampPitchFactor(baseFactor * PitchCurveFactorAt(curve, startMS+span*frame))
		cumulative[i+1] = cumulative[i] + 1/factor
	}
	targetFrames := max(16, int(math.Round(cumulative[len(source)])))
	result := make([]float64, targetFrames)
	segment := 0
	for output := range result {
		for segment < len(source)-1 && float64(output) >= cumulative[segment+1] {
			segment++
		}
		fraction := 0.0
		if segment < len(source)-1 {
			width := cumulative[segment+1] - cumulative[segment]
			if width > 0 {
				fraction = (float64(output) - cumulative[segment]) / width
			}
		}
		right := source[segment]
		if segment+1 < len(source) {
			right = source[segment+1]
		}
		result[output] = source[segment] + (right-source[segment])*fraction
	}
	return result
}

// MedianFloatは中央値を返す。空入力は0。
func MedianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

// IdentityFactorsは全要素1の係数配列を作る。
func IdentityFactors(size int) []float64 {
	result := make([]float64, size)
	for index := range result {
		result[index] = 1
	}
	return result
}

// NonzeroFloatsは正の要素だけを返す。
func NonzeroFloats(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, value := range values {
		if value > 0 {
			result = append(result, value)
		}
	}
	return result
}

// AnalyzeIntonationは音源を測定してイントネーション係数を返す。
func AnalyzeIntonation(synthesisPlan *plan.Plan, timings []EffectiveTiming, cache *SourceCache, strength float64) []float64 {
	pitches := make([]float64, len(synthesisPlan.Units))
	for i, unit := range synthesisPlan.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			continue
		}
		pitches[i], err = EstimateUnitPitch(unit, mono)
		if err != nil {
			continue
		}
	}
	return AnalyzeIntonationFromPitches(synthesisPlan, timings, pitches, strength)
}

// AnalyzeIntonationFromPitchesは測定済みF0からイントネーション係数を返す。
func AnalyzeIntonationFromPitches(synthesisPlan *plan.Plan, timings []EffectiveTiming, pitches []float64, strength float64) []float64 {
	factors := IdentityFactors(len(synthesisPlan.Units))
	strength = math.Max(0, math.Min(MaxIntonationStrength, strength))
	if strength == 0 {
		return factors
	}
	if synthesisPlan.SingleCV {
		pitches = StabilizeSingleCVPitches(synthesisPlan, pitches)
	} else {
		pitches = StabilizeWorldlinePitches(pitches)
	}
	voiced := NonzeroFloats(pitches)
	reference := MedianFloat(voiced)
	if reference <= 0 {
		return factors
	}
	for i := range pitches {
		if pitches[i] <= 0 {
			continue
		}
		for pitches[i] > reference*1.6 {
			pitches[i] /= 2
		}
		for pitches[i] < reference/1.6 {
			pitches[i] *= 2
		}
	}

	for start := 0; start < len(synthesisPlan.Units); {
		if synthesisPlan.Units[start].Role == "transition" {
			start++
			continue
		}
		end := start + 1
		lastPosition := synthesisPlan.Units[start].Position
		for end < len(synthesisPlan.Units) {
			unit := synthesisPlan.Units[end]
			if unit.Role == "transition" {
				end++
				continue
			}
			if unit.Position != lastPosition+1 {
				break
			}
			lastPosition = unit.Position
			end++
		}
		moraCount := 0
		for index := start; index < end; index++ {
			if synthesisPlan.Units[index].Role != "transition" {
				moraCount++
			}
		}
		moraIndex := 0
		for i := start; i < end; i++ {
			if synthesisPlan.Units[i].Role == "transition" {
				continue
			}
			position := 0.0
			if moraCount > 1 {
				position = float64(moraIndex) / float64(moraCount-1)
			}
			semitones := 0.3 - 0.8*position
			if moraIndex == 0 {
				semitones -= 0.35
			}
			if moraIndex == 1 {
				semitones += 0.25
			}
			target := reference * math.Pow(2, semitones/12)
			unit := &synthesisPlan.Units[i]
			unit.SourceF0Hz = pitches[i]
			if pitches[i] > 0 {
				effectiveStrength := strength
				if timings[i].Scale < 1 {
					effectiveStrength *= math.Max(0.25, timings[i].Scale)
				}
				factor := math.Pow(target/pitches[i], effectiveStrength)
				maxShift := 0.08 * math.Max(1, strength)
				factors[i] = math.Max(1-maxShift, math.Min(1+maxShift, factor))
				pitchFactor := unit.PitchFactor
				if pitchFactor <= 0 {
					pitchFactor = 1
				}
				unit.TargetF0Hz = pitches[i] * factors[i] * pitchFactor
			}
			unit.IntonationFactor = factors[i]
			moraIndex++
		}
		start = end
	}
	return factors
}

// StabilizeSingleCVPitchesは単独音の倍音/分周誤検出を近傍へ補正する。
func StabilizeSingleCVPitches(synthesisPlan *plan.Plan, values []float64) []float64 {
	base := StabilizeWorldlinePitches(values)
	result := append([]float64(nil), base...)
	if synthesisPlan == nil || !synthesisPlan.SingleCV {
		return result
	}
	for index, value := range result {
		if value <= 0 || index >= len(synthesisPlan.Units) || synthesisPlan.Units[index].Silent || synthesisPlan.Units[index].Role == "transition" {
			continue
		}
		context := singleCVPitchContext(synthesisPlan, base, index)
		if len(context) < 2 {
			continue
		}
		if !singleCVPitchContextReliable(context) {
			continue
		}
		reference := MedianFloat(context)
		if reference <= 0 {
			continue
		}
		ratio := value / reference
		if ratio >= .84 && ratio <= 1.19 {
			continue
		}
		best, bestDistance := value, math.Abs(math.Log2(ratio))
		for _, factor := range []float64{4.0 / 3, 3.0 / 2, 2, 3, 4, 3.0 / 4, 2.0 / 3, 1.0 / 2, 1.0 / 3, 1.0 / 4} {
			candidate := value * factor
			candidateRatio := candidate / reference
			if candidateRatio < .90 || candidateRatio > 1.10 {
				continue
			}
			distance := math.Abs(math.Log2(candidateRatio))
			if distance < bestDistance {
				best, bestDistance = candidate, distance
			}
		}
		if best == value {
			best = reference
		}
		result[index] = best
	}
	return result
}

func singleCVPitchContextReliable(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	return minimum > 0 && maximum/minimum <= 1.20
}

func singleCVPitchContext(synthesisPlan *plan.Plan, values []float64, index int) []float64 {
	result := make([]float64, 0, 5)
	for distance := 1; distance <= 2; distance++ {
		for _, neighbor := range []int{index - distance, index + distance} {
			if neighbor < 0 || neighbor >= len(values) || neighbor >= len(synthesisPlan.Units) {
				continue
			}
			unit := synthesisPlan.Units[neighbor]
			if unit.Silent || unit.Role == "transition" || unit.Position < 0 || unit.Position >= len(synthesisPlan.Morae) || values[neighbor] <= 0 {
				continue
			}
			if neighbor != index && math.Abs(float64(unit.Position-synthesisPlan.Units[index].Position)) > float64(distance) {
				continue
			}
			result = append(result, values[neighbor])
		}
	}
	return result
}

// StabilizeWorldlinePitchesは短い有声録音の倍音と分周誤検出を近い原音の高さへ補正する。
func StabilizeWorldlinePitches(values []float64) []float64 {
	result := append([]float64(nil), values...)
	reference := MedianFloat(NonzeroFloats(values))
	for index, value := range values {
		if value <= 0 {
			continue
		}
		if reference > 0 && value < reference*.67 {
			best, bestDistance := value, math.Inf(1)
			for _, factor := range []float64{2, 3, 4} {
				candidate := value * factor
				ratio := candidate / reference
				if ratio < .87 || ratio > 1.15 {
					continue
				}
				if distance := math.Abs(math.Log2(ratio)); distance < bestDistance {
					best, bestDistance = candidate, distance
				}
			}
			result[index] = best
		}
	}
	for index, value := range values {
		if value <= 0 || result[index] != value {
			continue
		}
		neighbor := nearestWorldlinePitch(result, index)
		if neighbor <= 0 {
			continue
		}
		ratio := value / neighbor
		if ratio < 1.35 {
			continue
		}
		factor := 2.0 / 3.0
		if ratio >= 1.8 {
			factor = 0.5
		}
		correctedRatio := ratio * factor
		if correctedRatio >= 0.87 && correctedRatio <= 1.15 {
			result[index] = value * factor
		}
	}
	return result
}

func nearestWorldlinePitch(values []float64, index int) float64 {
	for distance := 1; distance < len(values); distance++ {
		left := index - distance
		if left >= 0 && values[left] > 0 {
			return values[left]
		}
		right := index + distance
		if right < len(values) && values[right] > 0 {
			return values[right]
		}
	}
	return 0
}

// MeasureWorldlinePitchesはユニットごとの原音F0を並列に測定する。
func MeasureWorldlinePitches(synthesisPlan *plan.Plan, cache *SourceCache) ([]float64, int, error) {
	values := make([]float64, len(synthesisPlan.Units))
	sampleRate := 0
	type pitchJob struct {
		index int
		unit  plan.Unit
		mono  *audio.PCM
	}
	jobs := make([]pitchJob, 0, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, 0, err
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		jobs = append(jobs, pitchJob{index: index, unit: unit, mono: mono})
	}
	errs := make([]error, len(jobs))
	measure := func(jobIndex int) {
		job := jobs[jobIndex]
		values[job.index], errs[jobIndex] = EstimateUnitPitch(job.unit, job.mono)
	}
	workers := min(len(jobs), max(1, runtime.GOMAXPROCS(0)))
	if workers <= 1 {
		for jobIndex := range jobs {
			measure(jobIndex)
		}
	} else {
		var group sync.WaitGroup
		group.Add(workers)
		for worker := 0; worker < workers; worker++ {
			go func(start int) {
				defer group.Done()
				for jobIndex := start; jobIndex < len(jobs); jobIndex += workers {
					measure(jobIndex)
				}
			}(worker)
		}
		group.Wait()
	}
	for _, err := range errs {
		if err != nil {
			return nil, 0, err
		}
	}
	if synthesisPlan.SingleCV {
		return StabilizeSingleCVPitches(synthesisPlan, values), sampleRate, nil
	}
	return StabilizeWorldlinePitches(values), sampleRate, nil
}
