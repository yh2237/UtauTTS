package native

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/yh2237/UtauTTS/internal/tts"
)

// provideInferenceは、Web版でONNX Runtime Webが求めた推論の結果を受け取る。
// 大きい配列（音声と特徴量）は仮想FSのfloat32のファイルで受け取り、読んだら消す。
func (e *Engine) provideInference(requestJSON []byte) error {
	var request struct {
		Reads []struct {
			Key       string    `json:"key"`
			Durations []int     `json:"durations"`
			F0        []float32 `json:"f0"`
			AudioPath string    `json:"audio_path"`
		} `json:"reads"`
		Features []struct {
			Key    string `json:"key"`
			Path   string `json:"path"`
			Frames int    `json:"frames"`
			Dims   int    `json:"dims"`
		} `json:"features"`
	}
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		return fmt.Errorf("decode inference results: %w", err)
	}
	reads := make([]tts.ReferenceRead, len(request.Reads))
	for i, read := range request.Reads {
		reads[i] = tts.ReferenceRead{Key: read.Key, Durations: read.Durations, F0: read.F0}
		if read.AudioPath != "" {
			audio, err := takeFloat32File(read.AudioPath)
			if err != nil {
				return err
			}
			reads[i].Audio = audio
		}
	}
	features := make([]tts.ReferenceFeatures, len(request.Features))
	for i, item := range request.Features {
		values, err := takeFloat32File(item.Path)
		if err != nil {
			return err
		}
		features[i] = tts.ReferenceFeatures{Key: item.Key, Values: values, Frames: item.Frames, Dims: item.Dims}
	}
	return e.synth.ProvideReferenceInference(reads, features)
}

func takeFloat32File(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("%s: %d bytes is not float32", path, len(data))
	}
	values := make([]float32, len(data)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return values, nil
}
