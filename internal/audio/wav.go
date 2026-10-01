package audio

import (
	"errors"
	"io"
	"os"

	"github.com/yh2237/audiodsp/wav"

	"utautts/internal/atomicfile"
)

type PCM struct {
	SampleRate int
	Channels   int
	Data       []int16
}

func ReadWav(path string) (*PCM, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return DecodeWav(file)
}

func DecodeWav(input io.Reader) (*PCM, error) {
	pcm, err := wav.Decode(input)
	if err != nil {
		return nil, err
	}
	return (*PCM)(pcm), nil
}

func WriteWav(path string, pcm *PCM) error {
	if pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return errors.New("invalid pcm metadata")
	}
	if len(pcm.Data) == 0 {
		return errors.New("empty pcm data")
	}

	return atomicfile.Write(path, func(file io.Writer) error {
		return wav.Encode(file, (*wav.PCM)(pcm))
	})
}

// 合成結果用。サンプルレート・チャンネル数が不正な場合はnilを返す。
func PCMToWavBytes(pcm *PCM) []byte {
	data, err := wav.Bytes((*wav.PCM)(pcm))
	if err != nil {
		return nil
	}
	return data
}
