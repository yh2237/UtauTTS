package main

import (
	"fmt"
	"math"
	"sync"
)

func renderWorldlineR(library nativeLibrary, input manifest) ([]float32, error) {
	phrase, err := library.PhraseNew()
	if err != nil {
		return nil, err
	}
	defer library.PhraseDelete(phrase)

	errors := make(chan error, len(input.Units))
	var group sync.WaitGroup
	for index := range input.Units {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			item := input.Units[index]
			request, loadErr := loadUnitRequest(item, make([]int32, 2))
			if loadErr != nil {
				errors <- loadErr
				return
			}
			if validateErr := validateWorldlineRUnit(item, request.SampleRate, len(request.Samples)); validateErr != nil {
				errors <- validateErr
				return
			}
			if addErr := library.PhraseAdd(phrase, index, request, phraseTiming{item.PositionMS, item.SkipMS, item.LengthMS, item.FadeInMS, item.FadeOutMS}); addErr != nil {
				errors <- addErr
			}
		}()
	}
	group.Wait()
	close(errors)
	for addErr := range errors {
		if addErr != nil {
			return nil, addErr
		}
	}

	gender := repeated(0.5, len(input.F0Curve))
	tension := repeated(0.5, len(input.F0Curve))
	breathiness := repeated(0.5, len(input.F0Curve))
	voicing := repeated(1, len(input.F0Curve))
	if err := library.PhraseSetCurves(phrase, input.F0Curve, gender, tension, breathiness, voicing); err != nil {
		return nil, err
	}
	return library.PhraseSynth(phrase)
}

func repeated(value float64, length int) []float64 {
	result := make([]float64, length)
	for index := range result {
		result[index] = value
	}
	return result
}

func validateWorldlineRUnit(item unit, sampleRate, sampleCount int) error {
	const frameMS = 10.0
	totalMS := float64(sampleCount) * 1000 / float64(sampleRate)
	inputLengthMS := totalMS - item.OffsetMS - item.CutoffMS
	if item.CutoffMS < 0 {
		inputLengthMS = -item.CutoffMS
	}
	if item.OffsetMS < 0 || item.OffsetMS+inputLengthMS > totalMS+0.1 {
		return fmt.Errorf("oto range exceeds source audio: %s", item.Source)
	}
	startFrame := int(item.OffsetMS / frameMS)
	frameCount := int(math.Ceil((item.OffsetMS+inputLengthMS)/frameMS)) - startFrame
	maximumFrames := int(math.Ceil(totalMS / frameMS))
	frameCount = min(frameCount, maximumFrames-startFrame)
	if frameCount <= 0 {
		return fmt.Errorf("oto cutoff is before offset: %s", item.Source)
	}
	if item.RequiredLengthMS <= 0 || item.LengthMS <= 0 || item.SkipMS+item.LengthMS > item.RequiredLengthMS+frameMS+0.1 {
		return fmt.Errorf("WORLDLINE-R timing exceeds remapped source: %s", item.Source)
	}
	return nil
}
