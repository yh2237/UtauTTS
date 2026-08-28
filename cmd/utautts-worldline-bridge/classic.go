package main

import (
	"fmt"
	"math"
)

type classicSegment struct {
	unit       unit
	samples    []float32
	position   int
	skip       int
	correction int
}

func (segment classicSegment) visibleLength(sampleRate int) int {
	if len(segment.unit.Envelope) >= 5 {
		return max(0, len(segment.samples)-segment.skip)
	}
	requested := roundInt(segment.unit.LengthMS * float64(sampleRate) / 1000)
	return max(0, min(len(segment.samples)-segment.skip, requested))
}

func (segment classicSegment) end(sampleRate int) int {
	return segment.position + segment.correction + segment.visibleLength(sampleRate)
}

func (segment classicSegment) sampleAt(global, sampleRate int, correction ...int) float64 {
	adjustment := segment.correction
	if len(correction) > 0 {
		adjustment = correction[0]
	}
	local := global - segment.position - adjustment + segment.skip
	if local < segment.skip || local < 0 || local >= len(segment.samples) {
		return 0
	}
	return float64(segment.samples[local]) * segment.envelope(local-segment.skip, sampleRate)
}

func (segment classicSegment) envelope(visible, sampleRate int) float64 {
	length := segment.visibleLength(sampleRate)
	if visible < 0 || visible >= length {
		return 0
	}
	points := segment.unit.Envelope
	if len(points) >= 5 {
		sample := visible + segment.skip
		shift := -points[0].XMS
		next := 0
		for next < len(points) && float64(sample) > (points[next].XMS+shift)*float64(sampleRate)/1000+float64(segment.skip) {
			next++
		}
		if next == 0 {
			return points[0].Y
		}
		if next >= len(points) {
			return points[len(points)-1].Y
		}
		left, right := points[next-1], points[next]
		leftSample := (left.XMS+shift)*float64(sampleRate)/1000 + float64(segment.skip)
		rightSample := (right.XMS+shift)*float64(sampleRate)/1000 + float64(segment.skip)
		if leftSample >= rightSample {
			return left.Y
		}
		return left.Y + (right.Y-left.Y)*(float64(sample)-leftSample)/(rightSample-leftSample)
	}
	fadeIn := max(1, roundInt(segment.unit.FadeInMS*float64(sampleRate)/1000))
	fadeOut := max(1, roundInt(segment.unit.FadeOutMS*float64(sampleRate)/1000))
	gain := 1.0
	if visible < fadeIn {
		gain *= smoothStep(float64(visible) / float64(fadeIn))
	}
	if visible >= length-fadeOut {
		gain *= smoothStep(float64(length-visible-1) / float64(fadeOut))
	}
	return gain
}

func renderClassic(library nativeLibrary, input manifest) ([]float32, error) {
	if input.SampleRate != 44100 {
		return nil, fmt.Errorf("OpenUtau classic worldline currently requires 44100 Hz; got %d Hz", input.SampleRate)
	}
	segments := make([]classicSegment, len(input.Units))
	for index, item := range input.Units {
		segment, err := renderClassicSegment(library, item, input)
		if err != nil {
			return nil, err
		}
		segments[index] = segment
	}
	for index := 1; index < len(segments); index++ {
		segments[index].correction = findConvergenceCorrection(segments[index-1], segments[index], input)
	}
	length := 1
	for _, segment := range segments {
		length = max(length, segment.end(input.SampleRate))
	}
	if input.Engine == "classic-worldline-faithful-gpu" {
		return mixClassicGPU(input.GPUPath, segments, input.SampleRate, length)
	}
	mixed := make([]float32, length)
	for _, segment := range segments {
		start := segment.position + segment.correction
		for visible := 0; visible < segment.visibleLength(input.SampleRate); visible++ {
			output := start + visible
			if output >= 0 && output < len(mixed) {
				mixed[output] += float32(segment.sampleAt(output, input.SampleRate))
			}
		}
	}
	return mixed, nil
}

func renderClassicSegment(library nativeLibrary, item unit, input manifest) (classicSegment, error) {
	tempo := item.Tempo
	if tempo <= 0 {
		tempo = 120
	}
	pitchFrameMS := 60000 / tempo * 5 / 480
	pitchLengthMS := item.PitchLengthMS
	if pitchLengthMS <= 0 {
		pitchLengthMS = item.RequiredLengthMS
	}
	bends := make([]int32, max(2, int(math.Ceil(pitchLengthMS/pitchFrameMS))))
	for frame := range bends {
		timeMS := item.PitchStartMS + float64(frame)*pitchFrameMS
		target := curveAt(input.F0Curve, timeMS, 10)
		baseF0 := 440 * math.Pow(2, float64(item.Tone-69)/12)
		bend := roundInt(1200 * math.Log2(target/baseF0))
		bends[frame] = int32(max(-2048, min(2047, bend)))
	}
	request, err := loadUnitRequest(item, bends)
	if err != nil {
		return classicSegment{}, err
	}
	samples, err := library.Resample(request)
	if err != nil {
		return classicSegment{}, fmt.Errorf("worldline Resample %s: %w", item.Source, err)
	}
	return classicSegment{unit: item, samples: samples, position: roundInt(item.PositionMS * float64(input.SampleRate) / 1000), skip: roundInt(item.SkipMS * float64(input.SampleRate) / 1000)}, nil
}

func findConvergenceCorrection(previous, current classicSegment, input manifest) int {
	fs := input.SampleRate
	overlapStart := max(previous.position+previous.correction, current.position)
	overlapEnd := min(previous.end(fs), current.position+max(256, roundInt(current.unit.FadeInMS*float64(fs)/1000)))
	if overlapEnd-overlapStart < 128 {
		return 0
	}
	f0 := curveAt(input.F0Curve, current.unit.PositionMS+current.unit.FadeInMS*0.5, 10)
	if math.IsNaN(f0) || math.IsInf(f0, 0) || f0 < 40 {
		return 0
	}
	radius := max(1, min(550, roundInt(float64(fs)/f0*0.5)))
	bestShift, bestScore := 0, math.Inf(-1)
	for shift := -radius; shift <= radius; shift++ {
		var cross, leftEnergy, rightEnergy float64
		for sample := overlapStart; sample < overlapEnd; sample += 2 {
			left := previous.sampleAt(sample, fs)
			right := current.sampleAt(sample, fs, shift)
			cross += left * right
			leftEnergy += left * left
			rightEnergy += right * right
		}
		if leftEnergy < 1e-8 || rightEnergy < 1e-8 {
			continue
		}
		score := cross / math.Sqrt(leftEnergy*rightEnergy)
		if score > bestScore {
			bestScore, bestShift = score, shift
		}
	}
	if bestScore > 0 {
		return bestShift
	}
	return 0
}

func roundInt(value float64) int { return int(math.RoundToEven(value)) }

func smoothStep(value float64) float64 {
	value = max(0.0, min(1.0, value))
	return value * value * (3 - 2*value)
}
