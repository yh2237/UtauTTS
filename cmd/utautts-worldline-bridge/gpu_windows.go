//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func mixClassicGPU(path string, segments []classicSegment, sampleRate, resultLength int) ([]float32, error) {
	if path == "" {
		return nil, fmt.Errorf("faithful GPU renderer requires gpu_path")
	}
	for _, segment := range segments {
		if len(segment.unit.Envelope) != 5 {
			return nil, fmt.Errorf("faithful GPU mixer requires five-point envelopes")
		}
	}

	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("load CUDA faithful mixer: %w", err)
	}
	defer dll.Release()
	mix, err := dll.FindProc("UtauTTSGPUFaithfulMix")
	if err != nil {
		return nil, fmt.Errorf("load CUDA faithful mixer export: %w", err)
	}

	sampleCount := 0
	for _, segment := range segments {
		sampleCount += len(segment.samples)
	}
	samples := make([]float32, sampleCount)
	offsets := make([]int32, len(segments))
	lengths := make([]int32, len(segments))
	starts := make([]int32, len(segments))
	skips := make([]int32, len(segments))
	visibleLengths := make([]int32, len(segments))
	envelopeX := make([]float64, len(segments)*5)
	envelopeY := make([]float64, len(segments)*5)
	offset := 0
	for index, segment := range segments {
		offsets[index] = int32(offset)
		lengths[index] = int32(len(segment.samples))
		starts[index] = int32(segment.position + segment.correction)
		skips[index] = int32(segment.skip)
		visibleLengths[index] = int32(segment.visibleLength(sampleRate))
		copy(samples[offset:], segment.samples)
		offset += len(segment.samples)
		for point := 0; point < 5; point++ {
			envelopeX[index*5+point] = segment.unit.Envelope[point].XMS
			envelopeY[index*5+point] = segment.unit.Envelope[point].Y
		}
	}

	result := make([]float32, max(1, resultLength))
	errorBuffer := make([]byte, 512)
	ok, _, _ := mix.Call(
		slicePointer(samples), uintptr(len(samples)),
		slicePointer(offsets), slicePointer(lengths),
		slicePointer(starts), slicePointer(skips), slicePointer(visibleLengths),
		slicePointer(envelopeX), slicePointer(envelopeY), uintptr(len(segments)),
		uintptr(sampleRate), slicePointer(result), uintptr(len(result)),
		slicePointer(errorBuffer), uintptr(len(errorBuffer)),
	)
	if ok == 0 {
		return nil, fmt.Errorf("CUDA faithful mix failed: %s", zeroTerminated(errorBuffer))
	}
	return result, nil
}

func slicePointer[T any](values []T) uintptr {
	if len(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&values[0]))
}

func zeroTerminated(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}
	return string(buffer)
}
