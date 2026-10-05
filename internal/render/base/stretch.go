package base

import (
	"math"

	"github.com/yh2237/audiodsp/stretch"
)

func WSOLAStretch(source []float64, targetFrames, sampleRate int) ([]float64, error) {
	return wsola(source, targetFrames, sampleRate), nil
}

func StretchWSOLA(source []float64, targetFrames, sampleRate int) []float64 {
	return wsola(source, targetFrames, sampleRate)
}

func StretchWSOLAAnchored(source []float64, targetFrames, sampleRate int, sourceAnchors, targetAnchors []int) []float64 {
	return stretch.Anchored(source, targetFrames, sampleRate, sourceAnchors, targetAnchors)
}

func RetimeWithCompressedPrefixUsing(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
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

func StretchPreservingPrefixUsing(source []float64, targetFrames, prefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	return stretchPreservingPrefixUsing(source, targetFrames, prefixFrames, sampleRate, stretch)
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
	return stretch.WSOLA(source, targetFrames, sampleRate)
}

func linearResample(source []float64, targetFrames int) []float64 {
	return stretch.Linear(source, targetFrames)
}
