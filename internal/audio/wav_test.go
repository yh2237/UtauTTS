package audio

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"testing"
)

type fragmentReader struct{ reader io.Reader }

func (r fragmentReader) Read(data []byte) (int, error) {
	return r.reader.Read(data[:min(3, len(data))])
}

func TestDecodeWavPreservesSignedPCMAndChunkBoundaries(t *testing.T) {
	pcm := &PCM{SampleRate: 48000, Channels: 2, Data: make([]int16, 6006)}
	values := []int16{-32768, -1, 0, 32767}
	for i := range pcm.Data {
		pcm.Data[i] = values[i%len(values)]
	}
	data := PCMToWavBytes(pcm)
	withJunk := append([]byte(nil), data[:12]...)
	withJunk = append(withJunk, []byte{'J', 'U', 'N', 'K', 3, 0, 0, 0, 1, 2, 3, 0}...)
	withJunk = append(withJunk, data[12:]...)
	binary.LittleEndian.PutUint32(withJunk[4:8], uint32(len(withJunk)-8))
	for _, reader := range []io.Reader{bytes.NewReader(data), fragmentReader{bytes.NewReader(withJunk)}} {
		got, err := DecodeWav(reader)
		if err != nil || !reflect.DeepEqual(got, pcm) {
			t.Fatalf("PCM changed after streaming decode: %v, %v", got, err)
		}
	}
	if _, err := DecodeWav(bytes.NewReader(data[:len(data)-1])); err == nil {
		t.Fatal("truncated PCM accepted")
	}
	odd := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(odd[40:44], uint32(len(pcm.Data)*2-1))
	if _, err := DecodeWav(bytes.NewReader(odd)); err == nil {
		t.Fatal("unaligned 16-bit data accepted")
	}
}
