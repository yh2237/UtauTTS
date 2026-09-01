package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type worldFRQ struct {
	hopSize int
	f0      []float64
}

func readWorldFRQ(path string) (worldFRQ, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return worldFRQ{}, err
	}
	if len(data) < 40 || string(data[:8]) != "FREQ0003" {
		return worldFRQ{}, fmt.Errorf("invalid FREQ0003 header")
	}
	hopSize := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	length := int(int32(binary.LittleEndian.Uint32(data[36:40])))
	if hopSize <= 0 || length < 1 || len(data) < 40+length*16 {
		return worldFRQ{}, fmt.Errorf("invalid FREQ0003 shape")
	}
	f0 := make([]float64, length)
	for index := range f0 {
		bits := binary.LittleEndian.Uint64(data[40+index*16 : 48+index*16])
		f0[index] = math.Float64frombits(bits)
	}
	return worldFRQ{hopSize: hopSize, f0: f0}, nil
}

func sampleWorldFRQ(frq worldFRQ, frames, targetHop int, f0Floor float64) []float64 {
	result := make([]float64, frames)
	ratio := float64(targetHop) / float64(frq.hopSize)
	for frame := range result {
		left := min(len(frq.f0)-1, int(math.Floor(float64(frame)*ratio)))
		right := min(len(frq.f0)-1, int(math.Ceil(float64(frame+1)*ratio)))
		var sum float64
		var count int
		for index := left; index <= right; index++ {
			if frq.f0[index] > f0Floor {
				sum += frq.f0[index]
				count++
			}
		}
		if count > 0 {
			result[frame] = sum / float64(count)
		}
	}
	return result
}
