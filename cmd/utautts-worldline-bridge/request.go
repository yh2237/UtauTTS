package main

import (
	"fmt"
	"math"
	"os"
)

func loadUnitRequest(input unit, pitches []int32) (nativeRequest, error) {
	sampleRate, samples, err := readPCM16(input.Source)
	if err != nil {
		return nativeRequest{}, fmt.Errorf("read source %s: %w", input.Source, err)
	}
	var frq []byte
	if input.FrqPath != "" {
		if data, readErr := os.ReadFile(input.FrqPath); readErr == nil {
			frq = data
		}
	}
	tempo := input.Tempo
	if tempo <= 0 {
		tempo = 120
	}
	return nativeRequest{
		CacheKey: input.CacheKey, SampleRate: sampleRate, Samples: samples, FRQ: frq, Tone: input.Tone,
		ConsonantVelocity: input.ConsonantVelocity, OffsetMS: input.OffsetMS,
		RequiredLengthMS: input.RequiredLengthMS, ConsonantMS: input.ConsonantMS,
		CutoffMS: input.CutoffMS, Volume: input.Volume, Modulation: input.Modulation,
		Tempo: tempo, PitchBend: pitches, FlagP: 86, FlagMv: 100,
	}, nil
}

func curveAt(curve []float64, timeMS, frameMS float64) float64 {
	if len(curve) == 0 {
		return 220
	}
	position := math.Max(0, timeMS) / frameMS
	left := min(len(curve)-1, int(math.Floor(position)))
	right := min(len(curve)-1, left+1)
	alpha := min(1.0, max(0.0, position-float64(left)))
	return curve[left]*(1-alpha) + curve[right]*alpha
}
