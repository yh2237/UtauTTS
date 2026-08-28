package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

func readPCM16(path string) (int, []float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()
	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return 0, nil, err
	}
	if string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return 0, nil, fmt.Errorf("not PCM WAV: %s", path)
	}
	var format, channels, bits uint16
	var sampleRate uint32
	var data []byte
	for {
		var chunk [8]byte
		if _, err := io.ReadFull(file, chunk[:]); err != nil {
			if err == io.EOF {
				break
			}
			return 0, nil, err
		}
		size := binary.LittleEndian.Uint32(chunk[4:])
		payload := make([]byte, size)
		if _, err := io.ReadFull(file, payload); err != nil {
			return 0, nil, err
		}
		if size&1 != 0 {
			_, _ = file.Seek(1, io.SeekCurrent)
		}
		switch string(chunk[:4]) {
		case "fmt ":
			if len(payload) >= 16 {
				format = binary.LittleEndian.Uint16(payload)
				channels = binary.LittleEndian.Uint16(payload[2:])
				sampleRate = binary.LittleEndian.Uint32(payload[4:])
				bits = binary.LittleEndian.Uint16(payload[14:])
			}
		case "data":
			data = payload
		}
	}
	if format != 1 || channels == 0 || bits != 16 || sampleRate == 0 || data == nil {
		return 0, nil, fmt.Errorf("worldline bridge supports PCM16 WAV only: %s", path)
	}
	frames := len(data) / (int(channels) * 2)
	samples := make([]float64, frames)
	for frame := range samples {
		var sum float64
		for channel := 0; channel < int(channels); channel++ {
			offset := (frame*int(channels) + channel) * 2
			sum += float64(int16(binary.LittleEndian.Uint16(data[offset:]))) / 32768
		}
		samples[frame] = sum / float64(channels)
	}
	return int(sampleRate), samples, nil
}

func writePCM16(path string, sampleRate int, samples []float32) error {
	var peak float32
	for _, sample := range samples {
		if isFinite32(sample) && float32(math.Abs(float64(sample))) > peak {
			peak = float32(math.Abs(float64(sample)))
		}
	}
	scale := float32(1)
	if peak > 0.98 {
		scale = 0.98 / peak
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	dataSize := len(samples) * 2
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	for _, value := range []any{uint32(36 + dataSize), [4]byte{'W', 'A', 'V', 'E'}, [4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(1), uint32(sampleRate), uint32(sampleRate * 2), uint16(2), uint16(16), [4]byte{'d', 'a', 't', 'a'}, uint32(dataSize)} {
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			return err
		}
	}
	for _, sample := range samples {
		if !isFinite32(sample) {
			sample = 0
		}
		sample = max(-1, min(1, sample*scale))
		value := int16(math.RoundToEven(float64(sample * 32767)))
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			return err
		}
	}
	return nil
}

func isFinite32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
