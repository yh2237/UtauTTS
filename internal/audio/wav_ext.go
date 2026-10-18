package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// decodeExtendedWavは、audiodspのwavが扱わない形式のWAVを読む。
// 8/24/32bitの整数、32/64bitの浮動小数点、WAVE_FORMAT_EXTENSIBLEに対応する。
// チャンクの読み方はaudiodspのwavにそろえ、壊れたファイルは受け付けない。
func decodeExtendedWav(data []byte) (*PCM, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("wav: invalid header")
	}
	var format uint16
	var channels, rate, bits int
	var samples []byte
	fmtFound := false
	for at := 12; at < len(data); {
		if at+8 > len(data) {
			return nil, io.ErrUnexpectedEOF
		}
		id := string(data[at : at+4])
		size := int(binary.LittleEndian.Uint32(data[at+4:]))
		body := at + 8
		if size < 0 || body+size > len(data) {
			return nil, io.ErrUnexpectedEOF
		}
		switch {
		case id == "fmt ":
			if size < 16 {
				return nil, errors.New("wav: invalid fmt chunk")
			}
			format = binary.LittleEndian.Uint16(data[body:])
			channels = int(binary.LittleEndian.Uint16(data[body+2:]))
			rate = int(binary.LittleEndian.Uint32(data[body+4:]))
			bits = int(binary.LittleEndian.Uint16(data[body+14:]))
			if format == 0xfffe {
				// WAVE_FORMAT_EXTENSIBLEでは、GUIDの先頭2バイトに実際の形式が入る。
				if size < 40 {
					return nil, errors.New("wav: extensible fmt chunk is too short")
				}
				format = binary.LittleEndian.Uint16(data[body+24:])
			}
			fmtFound = true
		case id == "data" && fmtFound:
			samples = data[body : body+size]
		}
		at = body + size
		if size%2 == 1 {
			if at >= len(data) {
				// 最後のdataチャンクは、パディングがなくてもEOFで終われる。
				if id == "data" && fmtFound {
					break
				}
				return nil, io.ErrUnexpectedEOF
			}
			at++
		}
	}
	if !fmtFound || samples == nil {
		return nil, errors.New("wav: missing fmt or data chunk")
	}
	if channels <= 0 || rate <= 0 {
		return nil, errors.New("wav: sample rate and channels must be positive")
	}
	sampleBytes := bits / 8
	if sampleBytes == 0 {
		return nil, fmt.Errorf("wav: unsupported format %d with %d bits", format, bits)
	}
	// 端数のバイトは、サンプルにならないので捨てる。
	samples = samples[:len(samples)-len(samples)%sampleBytes]
	count := len(samples) / sampleBytes
	if count%channels != 0 {
		return nil, errors.New("wav: sample data is not aligned to channels")
	}
	out := &PCM{SampleRate: rate, Channels: channels, Data: make([]int16, count)}
	switch {
	case format == 1 && bits == 8:
		for i := range out.Data {
			out.Data[i] = int16(int(samples[i])-128) << 8
		}
	case format == 1 && bits == 16:
		for i := range out.Data {
			out.Data[i] = int16(binary.LittleEndian.Uint16(samples[i*2:]))
		}
	case format == 1 && bits == 24:
		for i := range out.Data {
			value := int32(samples[i*3]) | int32(samples[i*3+1])<<8 | int32(samples[i*3+2])<<16
			if value&0x800000 != 0 {
				value -= 0x1000000
			}
			out.Data[i] = int16(value >> 8)
		}
	case format == 1 && bits == 32:
		for i := range out.Data {
			out.Data[i] = int16(int32(binary.LittleEndian.Uint32(samples[i*4:])) >> 16)
		}
	case format == 3 && bits == 32:
		for i := range out.Data {
			out.Data[i] = floatToInt16(float64(math.Float32frombits(binary.LittleEndian.Uint32(samples[i*4:]))))
		}
	case format == 3 && bits == 64:
		for i := range out.Data {
			out.Data[i] = floatToInt16(math.Float64frombits(binary.LittleEndian.Uint64(samples[i*8:])))
		}
	default:
		return nil, fmt.Errorf("wav: unsupported format %d with %d bits", format, bits)
	}
	return out, nil
}

func floatToInt16(value float64) int16 {
	switch {
	case math.IsNaN(value):
		value = 0
	case value > 1:
		value = 1
	case value < -1:
		value = -1
	}
	return int16(math.Round(value * 32767))
}
