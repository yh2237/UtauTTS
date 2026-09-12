package render

import "math"

// wsolaStretchはWSOLAを共通のストレッチ関数型で呼び出す。
func wsolaStretch(source []float64, targetFrames, sampleRate int) ([]float64, error) {
	return wsola(source, targetFrames, sampleRate), nil
}

// StretchWSOLAは実験・診断処理から標準WSOLAを再利用する。
func StretchWSOLA(source []float64, targetFrames, sampleRate int) []float64 {
	return wsola(source, targetFrames, sampleRate)
}

// StretchWSOLAAnchoredは連続波形を分割せず、指定した時間写像に沿って伸縮する。
func StretchWSOLAAnchored(source []float64, targetFrames, sampleRate int, sourceAnchors, targetAnchors []int) []float64 {
	if len(sourceAnchors) < 2 || len(sourceAnchors) != len(targetAnchors) {
		return wsola(source, targetFrames, sampleRate)
	}
	for index := 1; index < len(sourceAnchors); index++ {
		if sourceAnchors[index] <= sourceAnchors[index-1] || targetAnchors[index] <= targetAnchors[index-1] {
			return wsola(source, targetFrames, sampleRate)
		}
	}
	segment := 0
	return wsolaMapped(source, targetFrames, sampleRate, func(output int) float64 {
		for segment+1 < len(targetAnchors)-1 && output > targetAnchors[segment+1] {
			segment++
		}
		leftTarget, rightTarget := targetAnchors[segment], targetAnchors[segment+1]
		leftSource, rightSource := sourceAnchors[segment], sourceAnchors[segment+1]
		progress := float64(output-leftTarget) / float64(max(1, rightTarget-leftTarget))
		return float64(leftSource) + progress*float64(rightSource-leftSource)
	})
}

func retimeWithCompressedPrefixUsing(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	if targetFrames <= 0 || len(source) == 0 {
		return nil, nil
	}
	sourcePrefixFrames = max(0, min(sourcePrefixFrames, len(source)))
	targetPrefixFrames = max(0, min(targetPrefixFrames, targetFrames))
	if sourcePrefixFrames == targetPrefixFrames {
		return stretchPreservingPrefixUsing(source, targetFrames, sourcePrefixFrames, sampleRate, stretch)
	}

	tailFrames := targetFrames - targetPrefixFrames
	crossfade := min(msToFrames(4, sampleRate), targetPrefixFrames, tailFrames, sourcePrefixFrames, len(source)-sourcePrefixFrames)
	if crossfade < 2 {
		result := make([]float64, targetFrames)
		prefix, err := stretch(source[:sourcePrefixFrames], targetPrefixFrames, sampleRate)
		if err != nil {
			return nil, err
		}
		tail, err := stretch(source[sourcePrefixFrames:], tailFrames, sampleRate)
		if err != nil {
			return nil, err
		}
		copy(result[:targetPrefixFrames], prefix)
		copy(result[targetPrefixFrames:], tail)
		return result, nil
	}

	// 境界の両側を含む共通区間を混合し、接続点の位相不連続を防ぐ。
	prefix, err := stretch(source[:sourcePrefixFrames+crossfade], targetPrefixFrames+crossfade, sampleRate)
	if err != nil {
		return nil, err
	}
	tail, err := stretch(source[sourcePrefixFrames-crossfade:], tailFrames+crossfade, sampleRate)
	if err != nil {
		return nil, err
	}
	overlap := crossfade * 2
	prefixOnly := targetPrefixFrames - crossfade
	result := make([]float64, targetFrames)
	copy(result[:prefixOnly], prefix[:prefixOnly])
	for i := 0; i < overlap; i++ {
		alpha := 0.5 - 0.5*math.Cos(math.Pi*float64(i+1)/float64(overlap+1))
		result[prefixOnly+i] = prefix[prefixOnly+i]*(1-alpha) + tail[i]*alpha
	}
	copy(result[targetPrefixFrames+crossfade:], tail[overlap:])
	declickJoin(result, targetPrefixFrames, msToFrames(2, sampleRate))
	return result, nil
}

func stretchPreservingPrefixUsing(source []float64, targetFrames, prefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	if targetFrames <= 0 || len(source) == 0 {
		return nil, nil
	}
	if targetFrames == len(source) {
		return append([]float64(nil), source...), nil
	}
	prefixFrames = min(prefixFrames, len(source), targetFrames)
	if prefixFrames < 0 {
		prefixFrames = 0
	}
	result := make([]float64, targetFrames)
	copy(result, source[:prefixFrames])
	remainingTarget := targetFrames - prefixFrames
	if remainingTarget == 0 {
		return result, nil
	}
	remainingSource := source[prefixFrames:]
	if len(remainingSource) < 2 {
		remainingSource = source
	}
	stretched, err := stretch(remainingSource, remainingTarget, sampleRate)
	if err != nil {
		return nil, err
	}
	copy(result[prefixFrames:], stretched)
	crossfade := min(msToFrames(3, sampleRate), prefixFrames, remainingTarget)
	for i := 0; i < crossfade; i++ {
		position := prefixFrames + i
		before := source[min(prefixFrames+i, len(source)-1)]
		alpha := float64(i+1) / float64(crossfade+1)
		result[position] = before*(1-alpha) + result[position]*alpha
	}
	return result, nil
}

func declickJoin(wave []float64, position, radius int) {
	if position <= 0 || position >= len(wave) || radius < 1 {
		return
	}
	left := max(0, position-radius)
	right := min(len(wave)-1, position+radius)
	if right-left < 3 {
		return
	}
	localDelta := 0.0
	count := 0
	for i := left + 1; i <= right; i++ {
		if i == position {
			continue
		}
		localDelta += math.Abs(wave[i] - wave[i-1])
		count++
	}
	localDelta /= float64(max(1, count))
	if math.Abs(wave[position]-wave[position-1]) <= math.Max(0.08, localDelta*4) {
		return
	}
	start, end := wave[left], wave[right]
	for i := left + 1; i < right; i++ {
		alpha := float64(i-left) / float64(right-left)
		wave[i] = start*(1-alpha) + end*alpha
	}
}

func wsola(source []float64, targetFrames, sampleRate int) []float64 {
	ratio := float64(len(source)) / float64(targetFrames)
	return wsolaMapped(source, targetFrames, sampleRate, func(output int) float64 {
		return float64(output) * ratio
	})
}

func wsolaMapped(source []float64, targetFrames, sampleRate int, sourcePosition func(int) float64) []float64 {
	if targetFrames <= 0 || len(source) == 0 {
		return nil
	}
	if len(source) < 16 || targetFrames < 16 {
		return linearResample(source, targetFrames)
	}
	window := min(msToFrames(40, sampleRate), len(source), targetFrames)
	if window < 16 {
		return linearResample(source, targetFrames)
	}
	if window%2 == 1 {
		window--
	}
	synthesisHop := max(1, window/2)
	search := max(1, min(msToFrames(5, sampleRate), window/4))
	accumulator := make([]float64, targetFrames+window)
	weights := make([]float64, len(accumulator))
	previousSource := 0
	for outputPosition := 0; outputPosition < targetFrames; outputPosition += synthesisHop {
		expected := int(math.Round(sourcePosition(outputPosition)))
		maxStart := max(0, len(source)-window)
		expected = min(expected, maxStart)
		start := expected
		if outputPosition > 0 {
			start = bestMatch(source, previousSource+synthesisHop, expected, search, window/2, maxStart)
		}
		for i := 0; i < window && outputPosition+i < len(accumulator); i++ {
			weight := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i+1)/float64(window+1))
			accumulator[outputPosition+i] += source[start+i] * weight
			weights[outputPosition+i] += weight
		}
		previousSource = start
	}

	result := make([]float64, targetFrames)
	for i := range result {
		if weights[i] > 1e-12 {
			result[i] = accumulator[i] / weights[i]
		}
	}
	return result
}

func bestMatch(source []float64, reference, expected, search, compare, maxStart int) int {
	reference = max(0, min(reference, maxStart))
	low := max(0, expected-search)
	high := min(maxStart, expected+search)
	best := expected
	bestScore := math.Inf(-1)
	for candidate := low; candidate <= high; candidate++ {
		length := min(compare, len(source)-reference, len(source)-candidate)
		if length < 4 {
			continue
		}
		numerator, leftEnergy, rightEnergy := 0.0, 0.0, 0.0
		for i := 0; i < length; i++ {
			left := source[reference+i]
			right := source[candidate+i]
			numerator += left * right
			leftEnergy += left * left
			rightEnergy += right * right
		}
		denominator := math.Sqrt(leftEnergy*rightEnergy) + 1e-12
		score := numerator / denominator
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

func linearResample(source []float64, targetFrames int) []float64 {
	if targetFrames <= 0 || len(source) == 0 {
		return nil
	}
	if len(source) == 1 {
		result := make([]float64, targetFrames)
		for i := range result {
			result[i] = source[0]
		}
		return result
	}
	result := make([]float64, targetFrames)
	for i := range result {
		position := float64(i) * float64(len(source)-1) / float64(max(1, targetFrames-1))
		left := int(math.Floor(position))
		right := min(left+1, len(source)-1)
		fraction := position - float64(left)
		result[i] = source[left]*(1-fraction) + source[right]*fraction
	}
	return result
}
