package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func buildTestWav(format uint16, bits, channels, rate int, samples []byte, extensible bool) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("RIFF")
	binary.Write(&buffer, binary.LittleEndian, uint32(0))
	buffer.WriteString("WAVE")
	var formatChunk bytes.Buffer
	tag := format
	if extensible {
		tag = 0xfffe
	}
	binary.Write(&formatChunk, binary.LittleEndian, tag)
	binary.Write(&formatChunk, binary.LittleEndian, uint16(channels))
	binary.Write(&formatChunk, binary.LittleEndian, uint32(rate))
	binary.Write(&formatChunk, binary.LittleEndian, uint32(rate*channels*bits/8))
	binary.Write(&formatChunk, binary.LittleEndian, uint16(channels*bits/8))
	binary.Write(&formatChunk, binary.LittleEndian, uint16(bits))
	if extensible {
		binary.Write(&formatChunk, binary.LittleEndian, uint16(22))
		binary.Write(&formatChunk, binary.LittleEndian, uint16(bits))
		binary.Write(&formatChunk, binary.LittleEndian, uint32(0))
		// KSDATAFORMAT_SUBTYPE_PCM / IEEE_FLOATの先頭2バイトに実際の形式が入る。
		binary.Write(&formatChunk, binary.LittleEndian, format)
		formatChunk.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71})
	}
	writeChunk := func(id string, body []byte) {
		buffer.WriteString(id)
		binary.Write(&buffer, binary.LittleEndian, uint32(len(body)))
		buffer.Write(body)
		if len(body)%2 != 0 {
			buffer.WriteByte(0)
		}
	}
	writeChunk("fmt ", formatChunk.Bytes())
	writeChunk("data", samples)
	out := buffer.Bytes()
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out
}

func TestDecodeExtendedWavFormats(t *testing.T) {
	rate := 16000
	tests := []struct {
		name   string
		format uint16
		bits   int
		data   []byte
		want   []int16
	}{
		{"uint8", 1, 8, []byte{128, 255, 0}, []int16{0, 127 << 8, -128 << 8}},
		{"int24", 1, 24, []byte{0x00, 0x00, 0x00, 0xFF, 0xFF, 0x7F, 0x00, 0x00, 0x80}, []int16{0, 32767, -32768}},
		{"int32", 1, 32, []byte{0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0x7F, 0x00, 0x00, 0x00, 0x80}, []int16{0, 32767, -32768}},
		{"float32", 3, 32, float32Bytes(0.5, -0.5), []int16{16384, -16384}},
		{"float64", 3, 64, float64Bytes(0.5, -0.5), []int16{16384, -16384}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wav := buildTestWav(test.format, test.bits, 1, rate, test.data, false)
			pcm, err := DecodeWav(bytes.NewReader(wav))
			if err != nil {
				t.Fatal(err)
			}
			if pcm.SampleRate != rate || pcm.Channels != 1 {
				t.Fatalf("metadata = %d Hz, %d ch", pcm.SampleRate, pcm.Channels)
			}
			if len(pcm.Data) != len(test.want) {
				t.Fatalf("samples = %d, want %d", len(pcm.Data), len(test.want))
			}
			for i, want := range test.want {
				if pcm.Data[i] != want {
					t.Fatalf("sample %d = %d, want %d", i, pcm.Data[i], want)
				}
			}
		})
	}
}

func TestDecodeExtensibleWav(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x40}
	wav := buildTestWav(1, 24, 1, 16000, data, true)
	pcm, err := DecodeWav(bytes.NewReader(wav))
	if err != nil {
		t.Fatal(err)
	}
	if pcm.SampleRate != 16000 || len(pcm.Data) != 2 || pcm.Data[1] != 16384 {
		t.Fatalf("decoded = %+v", pcm)
	}
}

func TestDecodeExtendedWavStereo(t *testing.T) {
	// 24bitステレオは、フレームごとに2サンプル並ぶ。
	data := []byte{0x00, 0x00, 0x40, 0x00, 0x00, 0xC0}
	wav := buildTestWav(1, 24, 2, 44100, data, false)
	pcm, err := DecodeWav(bytes.NewReader(wav))
	if err != nil {
		t.Fatal(err)
	}
	if pcm.Channels != 2 || len(pcm.Data) != 2 {
		t.Fatalf("decoded = %+v", pcm)
	}
	if pcm.Data[0] != 16384 || pcm.Data[1] != -16384 {
		t.Fatalf("samples = %v", pcm.Data)
	}
}

func float32Bytes(values ...float32) []byte {
	var buffer bytes.Buffer
	for _, value := range values {
		binary.Write(&buffer, binary.LittleEndian, math.Float32bits(value))
	}
	return buffer.Bytes()
}

func float64Bytes(values ...float64) []byte {
	var buffer bytes.Buffer
	for _, value := range values {
		binary.Write(&buffer, binary.LittleEndian, math.Float64bits(value))
	}
	return buffer.Bytes()
}
